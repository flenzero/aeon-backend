#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -lt 1 ]]; then
  echo "usage: $0 SERVICE [SERVICE...]" >&2
  echo "services: postgres redis account-api economy-api admin-api economy-worker" >&2
  exit 2
fi

"$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/restart.sh" "$@"
