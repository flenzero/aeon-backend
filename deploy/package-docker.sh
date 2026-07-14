#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${OUT_DIR:-$ROOT/outputs/packages}"
STAMP="$(date +%Y%m%d%H%M%S)"
BACKUP_PACKAGE_NAME="${BACKUP_PACKAGE_NAME:-aeonblight-server-$STAMP}"
LATEST_PACKAGE_NAME="${LATEST_PACKAGE_NAME:-aeonblight-server}"
STAGE="$OUT_DIR/$BACKUP_PACKAGE_NAME"
LATEST_STAGE="$OUT_DIR/$LATEST_PACKAGE_NAME"
PLATFORM="${PLATFORM:-linux/amd64}"
IMAGE_TAG="${IMAGE_TAG:-$STAMP}"
CHANNEL_TAG="${CHANNEL_TAG:-latest}"
IMAGE_PREFIX="${IMAGE_PREFIX:-aeonblight}"
BUILD_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/aeonblight-release.XXXXXX")"
COMPLETED=false

if [[ -e "$STAGE" ]]; then
  echo "package directory already exists: $STAGE" >&2
  exit 2
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required to build the release package" >&2
  exit 2
fi
if ! command -v go >/dev/null 2>&1; then
  echo "go is required to build the release package" >&2
  exit 2
fi

case "$PLATFORM" in
  linux/amd64) GOARCH_VALUE=amd64 ;;
  linux/arm64) GOARCH_VALUE=arm64 ;;
  *)
    echo "unsupported PLATFORM=$PLATFORM; use linux/amd64 or linux/arm64" >&2
    exit 2
    ;;
esac

cleanup() {
  rm -rf "$BUILD_ROOT"
  if [[ "$COMPLETED" != "true" ]]; then
    rm -rf "$STAGE"
    rm -rf "$LATEST_STAGE.tmp"
  fi
}
trap cleanup EXIT

mkdir -p "$OUT_DIR" "$STAGE/images"

ensure_writable_go_dir() {
  local env_name="$1"
  local fallback="$2"
  local current="${!env_name:-}"
  if [[ -z "$current" ]]; then
    current="$(go env "$env_name" 2>/dev/null || true)"
  fi
  if [[ -n "$current" ]] && mkdir -p "$current" 2>/dev/null && touch "$current/.aeonblight-write-test" 2>/dev/null; then
    rm -f "$current/.aeonblight-write-test"
    export "$env_name=$current"
    return 0
  fi
  mkdir -p "$fallback"
  export "$env_name=$fallback"
}

ensure_writable_go_dir GOCACHE "$ROOT/outputs/.cache/go-build"
ensure_writable_go_dir GOMODCACHE "$ROOT/outputs/.cache/go-mod"

find_ca_bundle() {
  for path in \
    /etc/ssl/cert.pem \
    /etc/ssl/certs/ca-certificates.crt \
    /opt/homebrew/etc/ca-certificates/cert.pem \
    /usr/local/etc/openssl@3/cert.pem \
    /usr/local/etc/openssl/cert.pem; do
    if [[ -f "$path" ]]; then
      printf '%s\n' "$path"
      return 0
    fi
  done
  return 1
}

CA_BUNDLE="$(find_ca_bundle || true)"
if [[ -z "$CA_BUNDLE" ]]; then
  echo "cannot find a local CA certificate bundle for scratch images" >&2
  echo "install ca-certificates locally or set SSL_CERT_FILE to a PEM bundle before packaging" >&2
  exit 2
fi

if [[ -n "${SSL_CERT_FILE:-}" && -f "$SSL_CERT_FILE" ]]; then
  CA_BUNDLE="$SSL_CERT_FILE"
fi

build_binary() {
  local package="$1"
  local output="$2"
  echo "Compiling $package -> $output"
  CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH_VALUE" go build -trimpath -ldflags="-s -w" -o "$output" "$package"
}

build_image() {
  local image="$1"
  local dockerfile="$2"
  echo "Building $image for $PLATFORM"
  if docker buildx version >/dev/null 2>&1; then
    docker buildx build --load --platform "$PLATFORM" -f "$dockerfile" -t "$image" "$3"
  else
    docker build --platform "$PLATFORM" -f "$dockerfile" -t "$image" "$3"
  fi
}

save_image() {
  local primary_image="$1"
  local channel_image="$2"
  local output="$2"
  output="$3"
  echo "Saving $primary_image and $channel_image -> $output"
  docker save -o "$output" "$primary_image" "$channel_image"
}

for service in account-api economy-api admin-api economy-worker; do
  image="$IMAGE_PREFIX/$service:$IMAGE_TAG"
  channel_image="$IMAGE_PREFIX/$service:$CHANNEL_TAG"
  context="$BUILD_ROOT/$service"
  mkdir -p "$context"
  build_binary "./cmd/$service" "$context/app"
  cp "$CA_BUNDLE" "$context/ca-certificates.crt"
  cp -R "$ROOT/configs" "$context/configs"
  find "$context" -name '.DS_Store' -delete
  build_image "$image" "$ROOT/deploy/Dockerfile.release-app" "$context"
  docker tag "$image" "$channel_image"
  save_image "$image" "$channel_image" "$STAGE/images/$service.tar"
done

migrator_image="$IMAGE_PREFIX/db-migrate:$IMAGE_TAG"
migrator_channel_image="$IMAGE_PREFIX/db-migrate:$CHANNEL_TAG"
migrator_context="$BUILD_ROOT/db-migrate"
mkdir -p "$migrator_context"
build_binary "./cmd/db-migrate" "$migrator_context/db-migrate"
cp "$CA_BUNDLE" "$migrator_context/ca-certificates.crt"
cp -R "$ROOT/migrations" "$migrator_context/migrations"
find "$migrator_context" -name '.DS_Store' -delete
build_image "$migrator_image" "$ROOT/deploy/Dockerfile.release-migrator" "$migrator_context"
docker tag "$migrator_image" "$migrator_channel_image"
save_image "$migrator_image" "$migrator_channel_image" "$STAGE/images/db-migrate.tar"

cp "$ROOT/deploy/linux/docker-compose.release.yml" "$STAGE/docker-compose.yml"
sed \
  -e "s|^IMAGE_TAG=.*|IMAGE_TAG=$CHANNEL_TAG|" \
  -e "s|^ACCOUNT_API_IMAGE=.*|ACCOUNT_API_IMAGE=$IMAGE_PREFIX/account-api|" \
  -e "s|^ECONOMY_API_IMAGE=.*|ECONOMY_API_IMAGE=$IMAGE_PREFIX/economy-api|" \
  -e "s|^ADMIN_API_IMAGE=.*|ADMIN_API_IMAGE=$IMAGE_PREFIX/admin-api|" \
  -e "s|^ECONOMY_WORKER_IMAGE=.*|ECONOMY_WORKER_IMAGE=$IMAGE_PREFIX/economy-worker|" \
  -e "s|^MIGRATOR_IMAGE=.*|MIGRATOR_IMAGE=$IMAGE_PREFIX/db-migrate|" \
  "$ROOT/deploy/linux/env/production.env.example" > "$STAGE/.env.example"
cp -R "$ROOT/deploy/linux/nginx" "$STAGE/nginx"
cp -R "$ROOT/deploy/linux/scripts" "$STAGE/ops"
cp "$ROOT/deploy/linux/README.md" "$STAGE/DEPLOYMENT.md"

find "$STAGE" -name '.DS_Store' -delete
find "$STAGE/ops" -type f -name '*.sh' -exec chmod 0755 {} +

{
  echo "Aeonblight release package"
  echo "created_at=$STAMP"
  echo "platform=$PLATFORM"
  echo "image_tag=$IMAGE_TAG"
  echo "channel_tag=$CHANNEL_TAG"
  echo "image_prefix=$IMAGE_PREFIX"
  echo "images:"
  echo "  $IMAGE_PREFIX/account-api:$IMAGE_TAG"
  echo "  $IMAGE_PREFIX/account-api:$CHANNEL_TAG"
  echo "  $IMAGE_PREFIX/economy-api:$IMAGE_TAG"
  echo "  $IMAGE_PREFIX/economy-api:$CHANNEL_TAG"
  echo "  $IMAGE_PREFIX/admin-api:$IMAGE_TAG"
  echo "  $IMAGE_PREFIX/admin-api:$CHANNEL_TAG"
  echo "  $IMAGE_PREFIX/economy-worker:$IMAGE_TAG"
  echo "  $IMAGE_PREFIX/economy-worker:$CHANNEL_TAG"
  echo "  $IMAGE_PREFIX/db-migrate:$IMAGE_TAG"
  echo "  $IMAGE_PREFIX/db-migrate:$CHANNEL_TAG"
} > "$STAGE/RELEASE.txt"

rm -rf "$LATEST_STAGE.tmp"
cp -R "$STAGE" "$LATEST_STAGE.tmp"
rm -rf "$LATEST_STAGE"
mv "$LATEST_STAGE.tmp" "$LATEST_STAGE"

COMPLETED=true
printf 'latest: %s\n' "$LATEST_STAGE"
printf 'backup: %s\n' "$STAGE"
