#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

services=("$@")
if [[ "${#services[@]}" -eq 0 ]]; then
  aeon_compose restart
else
  aeon_compose restart "${services[@]}"
fi

aeon_compose ps
