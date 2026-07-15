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
loaded_images=()
for image_tar in "$image_dir"/*.tar; do
  if [[ ! -f "$image_tar" ]]; then
    continue
  fi
  found=true
  echo "Loading $(basename "$image_tar")"
  while IFS= read -r line; do
    echo "$line"
    if [[ "$line" == Loaded\ image:\ * ]]; then
      loaded_images+=("${line#Loaded image: }")
    fi
  done < <(docker load -i "$image_tar")
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

channel_tag="${CHANNEL_TAG:-latest}"
for image_name in \
  "${ACCOUNT_API_IMAGE:-aeonblight/account-api}" \
  "${ECONOMY_API_IMAGE:-aeonblight/economy-api}" \
  "${ADMIN_API_IMAGE:-aeonblight/admin-api}" \
  "${ECONOMY_WORKER_IMAGE:-aeonblight/economy-worker}" \
  "${MIGRATOR_IMAGE:-aeonblight/db-migrate}"; do
  selected_image=""
  for loaded_image in "${loaded_images[@]}"; do
    if [[ "$loaded_image" == "$image_name:"* ]]; then
      selected_image="$loaded_image"
    fi
  done
  if [[ -n "$selected_image" && "$selected_image" != "$image_name:$channel_tag" ]]; then
    docker tag "$selected_image" "$image_name:$channel_tag"
    echo "Tagged $selected_image as $image_name:$channel_tag"
  fi
done

echo "Docker images loaded"
