#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

default_services=(account-api economy-api admin-api)
services=("$@")
if [[ "${#services[@]}" -eq 0 ]]; then
  services=("${default_services[@]}")
fi
deploy_all=false
if [[ "${#services[@]}" -eq 1 && "${services[0]}" == "all" ]]; then
  deploy_all=true
  services=()
elif [[ " ${services[*]} " == *" all "* ]]; then
  echo "usage: $0 [all|SERVICE...]" >&2
  echo "use 'all' by itself, or pass one or more compose service names" >&2
  exit 2
fi

build_flags=()
if grep -q '^[[:space:]]*build:' "$AEON_COMPOSE_FILE"; then
  build_flags=(--build)
fi

if [[ "$deploy_all" == "true" ]]; then
  aeon_compose up -d --force-recreate "${build_flags[@]}"
else
  aeon_compose up -d --force-recreate "${build_flags[@]}" "${services[@]}"
fi

aeon_compose ps
echo "deployment command completed"
if [[ "$deploy_all" == "true" ]]; then
  echo "deployed services: all"
else
  echo "deployed services: ${services[*]}"
fi
echo "run ops/migrate-db.sh bootstrap for a new database, or ops/migrate-db.sh up for an existing database"
