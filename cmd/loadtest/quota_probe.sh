#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT_DIR="${OUT_DIR:-/private/tmp/omnitoken-quota-probe}"
COMPOSE_FILE="${COMPOSE_FILE:-deploy/docker-compose.yml}"
GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"
ADMIN_URL="${ADMIN_URL:-http://localhost:8081}"
ADMIN_TOKEN="${ADMIN_TOKEN:-probe-admin}"
MODEL="${MODEL:-ark-code-latest}"
VIRTUAL_KEY_FILE="${VIRTUAL_KEY_FILE:-/private/tmp/omnitoken_probe_key}"
CONCURRENCY_LIST="${CONCURRENCY_LIST:-10 30 50}"
WARMUP_DURATION="${WARMUP_DURATION:-10s}"
MEASURE_DURATION="${MEASURE_DURATION:-60s}"
MAX_REQUESTS="${MAX_REQUESTS:-200000}"

if [[ ! -f "$VIRTUAL_KEY_FILE" ]]; then
  echo "missing virtual key file: $VIRTUAL_KEY_FILE" >&2
  exit 2
fi

mkdir -p "$OUT_DIR"
KEY="$(<"$VIRTUAL_KEY_FILE")"

psql_exec() {
  docker compose -f "$COMPOSE_FILE" exec -T postgres psql -U omnitoken -d omnitoken "$@"
}

sample_db() {
  local profile="$1"
  local sample="$2"
  {
    echo "-- pg_stat_statements sample ${sample}"
    psql_exec -c "
SELECT calls,
       round(mean_exec_time::numeric, 3) AS mean_ms,
       round(max_exec_time::numeric, 3) AS max_ms
FROM pg_stat_statements
WHERE query LIKE '%u.monthly_budget_cents%'
  AND query LIKE '%LEFT JOIN usage_events%'
ORDER BY calls DESC
LIMIT 5;"
    echo "-- pg_stat_activity sample ${sample}"
    psql_exec -c "
SELECT application_name, state, count(*) AS connections
FROM pg_stat_activity
WHERE application_name IN ('omnitoken-gateway', 'omnitoken-admin')
GROUP BY application_name, state
ORDER BY application_name, state;"
  } >"${OUT_DIR}/c${profile}-db-sample-${sample}.txt"
}

run_loadtest() {
  local concurrency="$1"
  local duration="$2"
  local output="$3"
  MAX_REQUESTS="$MAX_REQUESTS" go run ./cmd/loadtest \
    -concurrency "$concurrency" \
    -duration "$duration" \
    -allow-failures \
    -gateway "$GATEWAY_URL" \
    -admin "$ADMIN_URL" \
    -model "$MODEL" \
    -key "$KEY" \
    -admin-token "$ADMIN_TOKEN" >"$output"
}

cd "$ROOT_DIR"
psql_exec -c "CREATE EXTENSION IF NOT EXISTS pg_stat_statements;" >/dev/null

for concurrency in $CONCURRENCY_LIST; do
  echo "profile c${concurrency}: warmup ${WARMUP_DURATION}"
  run_loadtest "$concurrency" "$WARMUP_DURATION" "${OUT_DIR}/c${concurrency}-warmup.txt"
  psql_exec -c "SELECT pg_stat_statements_reset();" >/dev/null

  echo "profile c${concurrency}: measure ${MEASURE_DURATION}"
  run_loadtest "$concurrency" "$MEASURE_DURATION" "${OUT_DIR}/c${concurrency}-loadtest.txt" &
  pid=$!
  sleep 15
  sample_db "$concurrency" 1
  sleep 20
  sample_db "$concurrency" 2
  sleep 20
  sample_db "$concurrency" 3
  wait "$pid"

  docker stats --no-stream --format 'name={{.Name}} mem={{.MemUsage}} pids={{.PIDs}} cpu={{.CPUPerc}}' \
    omnitoken-gateway-1 omnitoken-postgres-1 omnitoken-admin-1 >"${OUT_DIR}/c${concurrency}-docker-stats.txt"
done

echo "quota probe outputs: ${OUT_DIR}"
