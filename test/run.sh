#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODE="${1:-auto}"
GOCACHE="${GOCACHE:-$ROOT/work/go-cache}"
GOMODCACHE="${GOMODCACHE:-/tmp/aeon-backend-go-mod}"
BIN_DIR="$ROOT/work/bin"
LOG_DIR="$ROOT/work/test-logs"
DATABASE_URL_DEFAULT="postgres://aeonblight:aeonblight_dev_password@127.0.0.1:55432/aeonblight_game?sslmode=disable"

mkdir -p "$GOCACHE" "$GOMODCACHE" "$BIN_DIR" "$LOG_DIR"
export GOCACHE GOMODCACHE
cd "$ROOT"

echo "[1/4] Go unit + HTTP/Solana/NFT contract tests"
AEONBLIGHT_SKIP_LOCAL_ENV=true DATABASE_URL= go test ./... -count=1

echo "[2/4] Build four independently runnable modules"
for service in account-api economy-api admin-api economy-worker; do
  go build -o "$BIN_DIR/$service" "./cmd/$service"
done

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
  env -u DATABASE_URL \
    AEONBLIGHT_ENV=production \
    REDIS_ENABLED=false \
    INTERNAL_KEY=dev-internal-key \
    JWT_SECRET=dev-jwt-secret \
    ADMIN_TOKEN=dev-admin-token \
    ADDR="$addr" \
    "$@" "$BIN_DIR/$service" >"$LOG_DIR/$service.log" 2>&1 &
  PIDS+=("$!")
}

wait_health() {
  local url="$1"
  local name="$2"
  for _ in $(seq 1 50); do
    if curl --fail --silent "$url/health" >/dev/null; then
      return 0
    fi
    sleep 0.1
  done
  echo "$name failed to become healthy; log follows" >&2
  sed -n '1,160p' "$LOG_DIR/$name.log" >&2
  return 1
}

echo "[3/4] Start local binaries and verify health + worker tick"
start_service account-api 127.0.0.1:18081
start_service economy-api 127.0.0.1:18082 ECONOMY_CONFIG_DIR="$ROOT/configs/economy"
start_service admin-api 127.0.0.1:18083
wait_health http://127.0.0.1:18081 account-api
wait_health http://127.0.0.1:18082 economy-api
wait_health http://127.0.0.1:18083 admin-api

env -u DATABASE_URL \
  AEONBLIGHT_ENV=production \
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

echo "[4/4] PostgreSQL/Redis integration"
if docker info >/dev/null 2>&1; then
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
  DATABASE_URL="${DATABASE_URL:-$DATABASE_URL_DEFAULT}" go test ./internal/platform/store -run '^TestPostgres' -count=1 -v
elif [[ "$MODE" == "--full" || "$MODE" == "full" ]]; then
  echo "Docker daemon is required for --full mode" >&2
  exit 1
else
  echo "SKIP: Docker daemon is not running; rerun with Docker Desktop active for PostgreSQL/Redis integration"
fi

echo "PASS: local module startup, 89 HTTP routes, Solana RPC contract, NFT backend lifecycle, unit tests and builds"
