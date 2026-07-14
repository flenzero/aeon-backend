#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ACTION="${1:-}"

if [[ -z "${DATABASE_URL:-}" ]]; then
  echo "db migration refused: DATABASE_URL is required" >&2
  exit 2
fi

if ! command -v psql >/dev/null 2>&1; then
  echo "db migration refused: psql is required" >&2
  exit 2
fi

case "$ACTION" in
  bootstrap)
    has_table="$(psql "$DATABASE_URL" -X -A -t -v ON_ERROR_STOP=1 -c "SELECT to_regclass('public.schema_migrations') IS NOT NULL")"
    if [[ "$has_table" == "t" ]]; then
      echo "db migration refused: bootstrap is only for a new database; run '$0 up' for an existing database" >&2
      exit 2
    fi
    echo "Applying canonical Aeonblight schema"
    psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$ROOT/migrations/aeonblight_full_schema.sql"
    ;;
  up)
    has_table="$(psql "$DATABASE_URL" -X -A -t -v ON_ERROR_STOP=1 -c "SELECT to_regclass('public.schema_migrations') IS NOT NULL")"
    if [[ "$has_table" != "t" ]]; then
      echo "db migration refused: schema_migrations is missing; run '$0 bootstrap' explicitly for a new database" >&2
      exit 2
    fi
    for file in "$ROOT"/migrations/updates/*.sql; do
      version="$(basename "$file" .sql)"
      applied="$(psql "$DATABASE_URL" -X -A -t -v ON_ERROR_STOP=1 -c "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = '$version')")"
      if [[ "$applied" == "t" ]]; then
        echo "SKIP $version"
        continue
      fi
      echo "APPLY $version"
      psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$file"
    done
    ;;
  status)
    psql "$DATABASE_URL" -X -v ON_ERROR_STOP=1 -c "SELECT version, applied_at FROM schema_migrations ORDER BY applied_at, version"
    ;;
  *)
    echo "usage: DATABASE_URL=... $0 {bootstrap|up|status}" >&2
    exit 2
    ;;
esac
