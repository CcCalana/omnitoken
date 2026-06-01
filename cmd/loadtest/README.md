# cmd/loadtest

Small local smoke-load tool for Demo-Ready gateway checks. It sends bounded non-stream chat requests, then verifies admin usage aggregation.

For concurrency baselines, run duration-based profiles with explicit request budgets:

```powershell
$env:MAX_REQUESTS = "200000"
go run .\cmd\loadtest `
  -concurrency 50 `
  -duration 60s `
  -allow-failures `
  -gateway http://localhost:8080 `
  -admin http://localhost:8081 `
  -model chat-fast `
  -key $VirtualKey `
  -admin-token $AdminToken
```

`cmd/loadtest/mockark` is a standard-library OpenAI-compatible mock upstream for
measurement only. Docker Compose builds it through `deploy/Dockerfile.mockark`
as service `mock-ark`.

## Quota probe

`quota_probe.sh` runs the vNext quota baseline profiles with a 10s warmup and a
60s measurement window for each concurrency level:

```bash
OUT_DIR=/private/tmp/omnitoken-quota-probe \
MAX_REQUESTS=200000 \
ADMIN_TOKEN=probe-admin \
MODEL=ark-code-latest \
cmd/loadtest/quota_probe.sh
```

Inputs:

- `VIRTUAL_KEY_FILE` points to a file containing the gateway virtual key.
- `CONCURRENCY_LIST` defaults to `10 30 50`.
- `WARMUP_DURATION` defaults to `10s`; `MEASURE_DURATION` defaults to `60s`.
- `COMPOSE_FILE` defaults to `deploy/docker-compose.yml`.

Outputs are written under `OUT_DIR`: loadtest summaries, three DB samples per
profile, and final `docker stats` snapshots.

## Postgres sampling

T-CONC-DSN sets `application_name` on gateway/admin Postgres clients so concurrent runs can sample the right sessions:

```sql
SELECT application_name, state, count(*) AS connections
FROM pg_stat_activity
WHERE application_name IN ('omnitoken-gateway', 'omnitoken-admin')
GROUP BY application_name, state
ORDER BY application_name, state;
```
