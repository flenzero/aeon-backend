#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

if [[ "$#" -eq 0 ]]; then
  aeon_compose logs -f --tail=200
else
  aeon_compose logs -f --tail=200 "$@"
fi
