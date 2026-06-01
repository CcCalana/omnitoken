#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${ROOT_DIR}/.env"
MASTER_KEY_FILE="${ROOT_DIR}/.omnitoken-master-key"
SSL_CERT="${ROOT_DIR}/deploy/ssl/server.crt"
SSL_KEY="${ROOT_DIR}/deploy/ssl/server.key"

FAILURES=0

pass() {
  printf '✓ %s\n' "$1"
}

fail() {
  printf '✗ %s\n' "$1"
  printf '  fix: %s\n' "$2"
  FAILURES=$((FAILURES + 1))
}

warn() {
  printf '! %s\n' "$1"
}

version_ge() {
  local current="$1"
  local required="$2"
  [ "$(printf '%s\n%s\n' "$required" "$current" | sort -V | head -n1)" = "$required" ]
}

env_value() {
  local key="$1"
  if [ ! -f "$ENV_FILE" ]; then
    return 0
  fi
  awk -F= -v key="$key" '
    $0 !~ /^[[:space:]]*#/ && $1 == key {
      value = substr($0, index($0, "=") + 1)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
      gsub(/^"|"$/, "", value)
      gsub(/^'\''|'\''$/, "", value)
      print value
      exit
    }
  ' "$ENV_FILE"
}

port_in_use() {
  local port="$1"
  if command -v ss >/dev/null 2>&1; then
    ss -ltn "sport = :${port}" | awk 'NR > 1 { found = 1 } END { exit found ? 0 : 1 }'
    return
  fi
  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -iTCP:"${port}" -sTCP:LISTEN >/dev/null 2>&1
    return
  fi
  warn "cannot check port ${port}: install ss or lsof"
  return 1
}

check_docker() {
  if ! command -v docker >/dev/null 2>&1; then
    fail "docker is not installed" "install Docker 24+"
    return
  fi
  local version
  version="$(docker version --format '{{.Server.Version}}' 2>/dev/null || true)"
  if [ -z "$version" ]; then
    fail "docker daemon is not reachable" "start Docker and ensure the current user can run docker"
  elif version_ge "$version" "24.0.0"; then
    pass "Docker version ${version}"
  else
    fail "Docker version ${version} is below 24.0.0" "upgrade Docker to 24+"
  fi

  if docker compose version >/dev/null 2>&1; then
    pass "docker compose is available"
  elif command -v docker-compose >/dev/null 2>&1; then
    pass "docker-compose is available"
  else
    fail "docker compose is not available" "install Docker Compose v2"
  fi
}

check_ports() {
  for port in 80 443; do
    if port_in_use "$port"; then
      fail "port ${port} is already in use" "stop the process using port ${port} before deployment"
    else
      pass "port ${port} is available"
    fi
  done
}

check_env() {
  if [ ! -f "$ENV_FILE" ]; then
    fail ".env file is missing" "copy .env.example to .env and set DeepSeek keys"
    return
  fi
  pass ".env exists"

  local found_key=0
  for key in OMNITOKEN_DEEPSEEK_KEYS_1 OMNITOKEN_DEEPSEEK_KEYS_2 OMNITOKEN_DEEPSEEK_KEYS_3; do
    if [ -n "$(env_value "$key")" ]; then
      found_key=1
    fi
  done
  if [ "$found_key" -eq 1 ]; then
    pass "at least one OMNITOKEN_DEEPSEEK_KEYS_1/2/3 value is set"
  else
    fail "no DeepSeek key is configured" "set at least one OMNITOKEN_DEEPSEEK_KEYS_1/2/3 value in .env"
  fi
}

check_master_key() {
  if [ ! -f "$MASTER_KEY_FILE" ]; then
    fail ".omnitoken-master-key is missing" "run: openssl rand -hex 32 > .omnitoken-master-key && chmod 600 .omnitoken-master-key"
    return
  fi
  local key
  key="$(tr -d '[:space:]' < "$MASTER_KEY_FILE")"
  if [ "${#key}" -ge 64 ]; then
    pass ".omnitoken-master-key length is at least 64 characters"
  else
    fail ".omnitoken-master-key is shorter than 64 characters" "regenerate it with: openssl rand -hex 32 > .omnitoken-master-key"
  fi
}

check_ssl() {
  if [ -f "$SSL_CERT" ]; then
    pass "deploy/ssl/server.crt exists"
  else
    fail "deploy/ssl/server.crt is missing" "generate or copy the TLS certificate to deploy/ssl/server.crt"
  fi
  if [ -f "$SSL_KEY" ]; then
    pass "deploy/ssl/server.key exists"
  else
    fail "deploy/ssl/server.key is missing" "generate or copy the TLS private key to deploy/ssl/server.key"
  fi
}

check_memory() {
  if ! command -v free >/dev/null 2>&1; then
    warn "cannot check memory: free command not found"
    return
  fi
  local mem_mb
  mem_mb="$(free -m | awk '/^Mem:/ { print $2 }')"
  if [ "${mem_mb:-0}" -ge 4096 ]; then
    pass "memory ${mem_mb} MB"
  else
    fail "memory ${mem_mb:-0} MB is below 4096 MB" "use a server with at least 4 GB RAM"
  fi
}

check_disk() {
  local available_kb
  available_kb="$(df -Pk "$ROOT_DIR" | awk 'NR == 2 { print $4 }')"
  if [ "${available_kb:-0}" -ge 10485760 ]; then
    pass "disk free space is at least 10 GB"
  else
    fail "disk free space is below 10 GB" "free disk space or use a larger volume"
  fi
}

check_docker
check_ports
check_env
check_master_key
check_ssl
check_memory
check_disk

if [ "$FAILURES" -ne 0 ]; then
  printf '\n%d deployment preflight check(s) failed.\n' "$FAILURES"
  exit 1
fi

printf '\nAll deployment preflight checks passed.\n'
