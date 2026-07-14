#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ARG="${1:-}"
SCOPE="${TEST_SCOPE:-contract}"
STUB_MODE="${STUB_MODE:-enabled}"
GOCACHE="${GOCACHE:-$ROOT/work/go-cache}"
GOMODCACHE="${GOMODCACHE:-/tmp/aeon-backend-go-mod}"
BIN_DIR="$ROOT/work/bin"
LOG_DIR="$ROOT/work/test-logs"
DATABASE_URL_DEFAULT="postgres://aeonblight:aeonblight_dev_password@127.0.0.1:55432/aeonblight_game?sslmode=disable"

case "$ARG" in
  --unit|unit) SCOPE=unit ;;
  --contract|contract|"") ;;
  --integration|integration) SCOPE=integration ;;
  --full|full) SCOPE=full ;;
  *) echo "unknown test mode: $ARG" >&2; exit 2 ;;
esac

case "$SCOPE" in
  unit|contract|integration|full) ;;
  *) echo "TEST_SCOPE must be unit, contract, integration, or full" >&2; exit 2 ;;
esac
case "$STUB_MODE" in
  enabled|disabled) ;;
  *) echo "STUB_MODE must be enabled or disabled" >&2; exit 2 ;;
esac

mkdir -p "$GOCACHE" "$GOMODCACHE" "$BIN_DIR" "$LOG_DIR"
export GOCACHE GOMODCACHE
cd "$ROOT"

echo "Aeonblight test profile: scope=$SCOPE stub=$STUB_MODE"
echo "[1/4] Unit tests"
AEONBLIGHT_SKIP_LOCAL_ENV=true APP_PROFILE=test TEST_SCOPE=unit STUB_MODE=enabled DATABASE_URL= go test ./internal/... -count=1

if [[ "$SCOPE" == "unit" ]]; then
  echo "PASS: unit tests"
  exit 0
fi

echo "[2/4] Contract tests and four module builds"
AEONBLIGHT_SKIP_LOCAL_ENV=true APP_PROFILE=test TEST_SCOPE=contract STUB_MODE=enabled DATABASE_URL= go test ./... -count=1
for service in account-api economy-api admin-api economy-worker; do
  go build -o "$BIN_DIR/$service" "./cmd/$service"
done

if [[ "$SCOPE" == "contract" ]]; then
  echo "PASS: unit/contract tests, 116 HTTP routes, Solana RPC contract, NFT lifecycle, and four builds"
  exit 0
fi

echo "[3/4] PostgreSQL/Redis integration with explicit migration"
if ! docker info >/dev/null 2>&1; then
  echo "Docker daemon is required for TEST_SCOPE=$SCOPE" >&2
  exit 1
fi
docker compose -f deploy/docker-compose.yml --env-file local.env up -d postgres redis
ready=false
for _ in $(seq 1 60); do
  if docker compose -f deploy/docker-compose.yml --env-file local.env exec -T postgres pg_isready -U aeonblight -d aeonblight_game >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 0.5
done
if [[ "$ready" != true ]]; then
  echo "PostgreSQL did not become ready" >&2
  exit 1
fi

ACTIVE_DATABASE_URL="${DATABASE_URL:-$DATABASE_URL_DEFAULT}"
has_schema="$(psql "$ACTIVE_DATABASE_URL" -X -A -t -v ON_ERROR_STOP=1 -c "SELECT to_regclass('public.schema_migrations') IS NOT NULL")"
if [[ "$has_schema" == "t" ]]; then
  DATABASE_URL="$ACTIVE_DATABASE_URL" "$ROOT/scripts/db-migrate.sh" up >"$LOG_DIR/db-migrate.log"
else
  DATABASE_URL="$ACTIVE_DATABASE_URL" "$ROOT/scripts/db-migrate.sh" bootstrap >"$LOG_DIR/db-migrate.log"
fi
DATABASE_URL="${DATABASE_URL:-$DATABASE_URL_DEFAULT}" go test ./internal/platform/store -run '^TestPostgres' -count=1 -v

if [[ "$SCOPE" == "integration" ]]; then
  echo "PASS: unit/contract/PostgreSQL integration tests"
  exit 0
fi

PIDS=()
cleanup() {
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
  done
}
trap cleanup EXIT INT TERM

start_service() {
  local service="$1"
  local addr="$2"
  shift 2
  env \
    AEONBLIGHT_SKIP_LOCAL_ENV=true \
    APP_PROFILE=test \
    TEST_SCOPE=full \
    STUB_MODE="$STUB_MODE" \
    DATABASE_URL="${DATABASE_URL:-$DATABASE_URL_DEFAULT}" \
    REDIS_ENABLED=true \
    REDIS_ADDR=127.0.0.1:56379 \
    INTERNAL_KEY=dev-internal-key \
    JWT_SECRET=dev-jwt-secret \
    ADMIN_TOKEN=dev-admin-token \
    REQUIRED_SCHEMA_VERSION=20260714_admin_signed_login_v1 \
    ADDR="$addr" \
    "$@" "$BIN_DIR/$service" >"$LOG_DIR/$service.log" 2>&1 &
  PIDS+=("$!")
}

wait_endpoint() {
  local url="$1"
  local name="$2"
  for _ in $(seq 1 50); do
    if curl --fail --silent "$url" >/dev/null; then
      return 0
    fi
    sleep 0.1
  done
  echo "$name failed at $url; log follows" >&2
  sed -n '1,200p' "$LOG_DIR/$name.log" >&2
  return 1
}

echo "[4/4] Full local startup, liveness, readiness, and worker tick"
start_service account-api 127.0.0.1:18081
start_service economy-api 127.0.0.1:18082 ECONOMY_CONFIG_DIR="$ROOT/configs/economy"
start_service admin-api 127.0.0.1:18083
for port_name in "18081 account-api" "18082 economy-api" "18083 admin-api"; do
  read -r port name <<<"$port_name"
  wait_endpoint "http://127.0.0.1:$port/health" "$name"
  wait_endpoint "http://127.0.0.1:$port/ready" "$name"
done

env \
  AEONBLIGHT_SKIP_LOCAL_ENV=true \
  APP_PROFILE=test \
  TEST_SCOPE=full \
  STUB_MODE="$STUB_MODE" \
  ECONOMY_API_URL=http://127.0.0.1:18082 \
  INTERNAL_KEY=dev-internal-key \
  WORKER_INTERVAL_SECONDS=1 \
  "$BIN_DIR/economy-worker" >"$LOG_DIR/economy-worker.log" 2>&1 &
PIDS+=("$!")
sleep 2
if ! grep -q "worker tick completed" "$LOG_DIR/economy-worker.log"; then
  echo "economy-worker did not complete a local tick" >&2
  sed -n '1,160p' "$LOG_DIR/economy-worker.log" >&2
  exit 1
fi

echo "PASS: full local stack, 116 HTTP routes, readiness, Solana contracts, NFT lifecycle, integration tests, and four modules"
