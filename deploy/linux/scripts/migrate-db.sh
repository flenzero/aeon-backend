#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

action="${1:-status}"
case "$action" in
  bootstrap|up|status) ;;
  *)
    echo "usage: $0 {bootstrap|up|status}" >&2
    exit 2
    ;;
esac

aeon_load_env

if ! command -v psql >/dev/null 2>&1; then
  psql_available=false
else
  psql_available=true
fi

if [[ -x "$AEON_ROOT/scripts/db-migrate.sh" && "$psql_available" == "true" ]]; then
  if [[ -z "${MIGRATION_DATABASE_URL:-}" ]]; then
    echo "MIGRATION_DATABASE_URL is required in .env" >&2
    exit 2
  fi
  DATABASE_URL="$MIGRATION_DATABASE_URL" "$AEON_ROOT/scripts/db-migrate.sh" "$action"
  exit 0
fi

project="${COMPOSE_PROJECT_NAME:-aeonblight}"
migrator_name="${MIGRATOR_IMAGE:-aeonblight/db-migrate}"
migrator_image="$(aeon_image "$migrator_name" "${IMAGE_TAG:-latest}")"

if [[ -z "${DATABASE_URL:-}" ]]; then
  echo "DATABASE_URL is required in .env for the migrator image" >&2
  exit 2
fi

if ! docker image inspect "$migrator_image" >/dev/null 2>&1; then
  if [[ -f "$AEON_ROOT/images/db-migrate.tar" ]]; then
    echo "Loading missing migrator image from images/db-migrate.tar"
    docker load -i "$AEON_ROOT/images/db-migrate.tar"
  fi
fi

if ! docker image inspect "$migrator_image" >/dev/null 2>&1; then
  migrator_image="$(aeon_image "$migrator_name" "${IMAGE_TAG:-latest}")"
fi

if ! docker image inspect "$migrator_image" >/dev/null 2>&1; then
  echo "missing Docker image: $migrator_image" >&2
  echo "run ops/load-images.sh first, or set IMAGE_TAG in .env to a tag listed in RELEASE.txt" >&2
  exit 2
fi

docker run --rm \
  --network "${project}_default" \
  -e DATABASE_URL="$DATABASE_URL" \
  "$migrator_image" \
  "$action"
