#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

services=("$@")
build_flags=()
if grep -q '^[[:space:]]*build:' "$AEON_COMPOSE_FILE"; then
  build_flags=(--build)
fi

if [[ "${#services[@]}" -eq 0 ]]; then
  aeon_compose up -d --force-recreate "${build_flags[@]}"
else
  aeon_compose up -d --force-recreate "${build_flags[@]}" "${services[@]}"
fi

aeon_compose ps
echo "deployment command completed"
echo "run ops/migrate-db.sh bootstrap for a new database, or ops/migrate-db.sh up for an existing database"
