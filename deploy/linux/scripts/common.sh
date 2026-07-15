#!/usr/bin/env bash
set -euo pipefail

AEON_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

if [[ -f "$AEON_SCRIPT_DIR/../docker-compose.yml" && -d "$AEON_SCRIPT_DIR/../ops" ]]; then
  AEON_ROOT="$(cd "$AEON_SCRIPT_DIR/.." && pwd)"
elif [[ -f "$AEON_SCRIPT_DIR/../go.mod" ]]; then
  AEON_ROOT="$(cd "$AEON_SCRIPT_DIR/.." && pwd)"
elif [[ -f "$AEON_SCRIPT_DIR/../../../go.mod" ]]; then
  AEON_ROOT="$(cd "$AEON_SCRIPT_DIR/../../.." && pwd)"
else
  echo "cannot locate Aeonblight package root from $AEON_SCRIPT_DIR" >&2
  exit 2
fi

if [[ -f "$AEON_ROOT/docker-compose.yml" ]]; then
  AEON_COMPOSE_FILE="$AEON_ROOT/docker-compose.yml"
elif [[ -f "$AEON_ROOT/deploy/linux/docker-compose.release.yml" ]]; then
  AEON_COMPOSE_FILE="$AEON_ROOT/deploy/linux/docker-compose.release.yml"
else
  AEON_COMPOSE_FILE="$AEON_ROOT/deploy/linux/docker-compose.prod.yml"
fi

AEON_ENV_FILE="${ENV_FILE:-$AEON_ROOT/.env}"
AEON_DOCKERFILE="${AEON_DOCKERFILE:-$AEON_ROOT/deploy/Dockerfile}"
AEON_BUILD_CONTEXT="${AEON_BUILD_CONTEXT:-$AEON_ROOT}"

export AEON_ENV_FILE
export AEON_DOCKERFILE
export AEON_BUILD_CONTEXT

aeon_need_env() {
  if [[ ! -f "$AEON_ENV_FILE" ]]; then
    echo "missing env file: $AEON_ENV_FILE" >&2
    echo "copy .env.example to .env and fill required values first" >&2
    exit 2
  fi
}

aeon_compose() {
  aeon_load_env
  local compose_image_tag="${IMAGE_TAG:-}"
  if [[ -z "$compose_image_tag" ]]; then
    compose_image_tag="$(awk -F= '$1 == "IMAGE_TAG" {print $2; exit}' "$AEON_ENV_FILE" 2>/dev/null || true)"
  fi
  compose_image_tag="$(aeon_resolve_image_tag "${ACCOUNT_API_IMAGE:-aeonblight/account-api}" "${compose_image_tag:-latest}")"
  echo "compose IMAGE_TAG=$compose_image_tag"
  IMAGE_TAG="$compose_image_tag" docker compose --env-file "$AEON_ENV_FILE" -f "$AEON_COMPOSE_FILE" "$@"
}

aeon_load_env() {
  aeon_need_env
  set -a
  # shellcheck disable=SC1090
  source "$AEON_ENV_FILE"
  set +a
}

aeon_image_exists() {
  docker image inspect "$1" >/dev/null 2>&1
}

aeon_resolve_image_tag() {
  local image_name="$1"
  local requested_tag="${2:-}"

  if [[ -n "$requested_tag" ]] && aeon_image_exists "$image_name:$requested_tag"; then
    printf '%s\n' "$requested_tag"
    return 0
  fi

  if [[ -n "$requested_tag" ]]; then
    printf '%s\n' "$requested_tag"
  else
    printf 'latest\n'
  fi
}

aeon_image() {
  local image_name="$1"
  local requested_tag="${2:-${IMAGE_TAG:-latest}}"
  local resolved_tag
  resolved_tag="$(aeon_resolve_image_tag "$image_name" "$requested_tag")"
  printf '%s:%s\n' "$image_name" "$resolved_tag"
}

aeon_sudo() {
  if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
    "$@"
  else
    sudo "$@"
  fi
}
