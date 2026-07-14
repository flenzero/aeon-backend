#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

image_dir="$AEON_ROOT/images"
if [[ ! -d "$image_dir" ]]; then
  echo "missing image directory: $image_dir" >&2
  exit 2
fi

found=false
for image_tar in "$image_dir"/*.tar; do
  if [[ ! -f "$image_tar" ]]; then
    continue
  fi
  found=true
  echo "Loading $(basename "$image_tar")"
  docker load -i "$image_tar"
done

if [[ "$found" != "true" ]]; then
  echo "no Docker image tar files found in $image_dir" >&2
  exit 2
fi

if [[ -f "$AEON_ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$AEON_ENV_FILE"
  set +a
fi

release_tag="$(aeon_release_value image_tag || true)"
channel_tag="${CHANNEL_TAG:-latest}"
if [[ -n "$release_tag" ]]; then
  for image_name in \
    "${ACCOUNT_API_IMAGE:-aeonblight/account-api}" \
    "${ECONOMY_API_IMAGE:-aeonblight/economy-api}" \
    "${ADMIN_API_IMAGE:-aeonblight/admin-api}" \
    "${ECONOMY_WORKER_IMAGE:-aeonblight/economy-worker}" \
    "${MIGRATOR_IMAGE:-aeonblight/db-migrate}"; do
    if docker image inspect "$image_name:$release_tag" >/dev/null 2>&1; then
      docker tag "$image_name:$release_tag" "$image_name:$channel_tag"
      echo "Tagged $image_name:$release_tag as $image_name:$channel_tag"
    fi
  done
fi

echo "Docker images loaded"
