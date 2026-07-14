#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

aeon_load_env

curl -fsS "http://127.0.0.1:${ACCOUNT_API_PORT:-8081}/health"
printf '\n'
curl -fsS "http://127.0.0.1:${ACCOUNT_API_PORT:-8081}/ready"
printf '\n'
curl -fsS "http://127.0.0.1:${ECONOMY_API_PORT:-8082}/health"
printf '\n'
curl -fsS "http://127.0.0.1:${ECONOMY_API_PORT:-8082}/ready"
printf '\n'
curl -fsS "http://127.0.0.1:${ADMIN_API_PORT:-8083}/health"
printf '\n'
curl -fsS "http://127.0.0.1:${ADMIN_API_PORT:-8083}/ready"
printf '\n'
