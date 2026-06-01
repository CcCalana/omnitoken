#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CERT_FILE="${ROOT_DIR}/deploy/ssl/server.crt"

usage() {
  echo "usage: $0 <SERVER-IP-or-host> <VIRTUAL-KEY> [--cacert <path>]"
}

if [ "$#" -lt 2 ]; then
  usage
  exit 2
fi

SERVER="$1"
VIRTUAL_KEY="$2"
shift 2

while [ "$#" -gt 0 ]; do
  case "$1" in
    --cacert)
      if [ "$#" -lt 2 ]; then
        usage
        exit 2
      fi
      CERT_FILE="$2"
      shift 2
      ;;
    *)
      usage
      exit 2
      ;;
  esac
done

if [ ! -f "$CERT_FILE" ]; then
  echo "✗ CA certificate not found: ${CERT_FILE}"
  exit 1
fi

BASE_URL="https://${SERVER}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

request() {
  local method="$1"
  local path="$2"
  local data_file="$3"
  local output_file="$4"
  shift 4

  local status
  if [ -n "$data_file" ]; then
    status="$(curl -sS --cacert "$CERT_FILE" -o "$output_file" -w '%{http_code}' \
      -X "$method" "${BASE_URL}${path}" \
      -H 'Content-Type: application/json' \
      "$@" \
      --data-binary "@${data_file}")"
  else
    status="$(curl -sS --cacert "$CERT_FILE" -o "$output_file" -w '%{http_code}' \
      -X "$method" "${BASE_URL}${path}" \
      "$@")"
  fi
  printf '%s' "$status"
}

require_status() {
  local name="$1"
  local got="$2"
  local want="$3"
  local body_file="$4"
  if [ "$got" != "$want" ]; then
    echo "✗ ${name}: HTTP ${got}, want ${want}"
    sed 's/^/  /' "$body_file"
    exit 1
  fi
  echo "✓ ${name}: HTTP ${got}"
}

require_json_field() {
  local name="$1"
  local body_file="$2"
  local jq_expr="$3"
  if ! jq -e "$jq_expr" "$body_file" >/dev/null; then
    echo "✗ ${name}: missing expected JSON field (${jq_expr})"
    sed 's/^/  /' "$body_file"
    exit 1
  fi
  echo "✓ ${name}: JSON field ok"
}

HEALTH_BODY="${TMP_DIR}/health.json"
status="$(request GET /healthz '' "$HEALTH_BODY")"
require_status "healthz" "$status" "200" "$HEALTH_BODY"

ANTHROPIC_REQ="${TMP_DIR}/anthropic.json"
ANTHROPIC_BODY="${TMP_DIR}/anthropic-response.json"
cat > "$ANTHROPIC_REQ" <<'JSON'
{"model":"chat-fast","max_tokens":16,"messages":[{"role":"user","content":"say hi"}]}
JSON
status="$(request POST /v1/messages "$ANTHROPIC_REQ" "$ANTHROPIC_BODY" -H "x-api-key: ${VIRTUAL_KEY}")"
require_status "Anthropic /v1/messages" "$status" "200" "$ANTHROPIC_BODY"
require_json_field "Anthropic /v1/messages" "$ANTHROPIC_BODY" '.type == "message"'

OPENAI_REQ="${TMP_DIR}/openai.json"
OPENAI_BODY="${TMP_DIR}/openai-response.json"
cat > "$OPENAI_REQ" <<'JSON'
{"model":"chat-fast","messages":[{"role":"user","content":"say hi"}],"max_tokens":16}
JSON
status="$(request POST /v1/chat/completions "$OPENAI_REQ" "$OPENAI_BODY" -H "Authorization: Bearer ${VIRTUAL_KEY}")"
require_status "OpenAI /v1/chat/completions" "$status" "200" "$OPENAI_BODY"
require_json_field "OpenAI /v1/chat/completions" "$OPENAI_BODY" '.choices'

echo "All deployment smoke checks passed."
