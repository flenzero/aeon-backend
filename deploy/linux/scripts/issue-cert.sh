#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

aeon_load_env

API_DOMAIN="${API_DOMAIN:-api.aeonblight.com}"
GAME_DOMAIN="${GAME_DOMAIN:-game.aeonblight.com}"
CERT_NAME="${CERT_NAME:-aeonblight-api-game}"
ACME_WEBROOT="${ACME_WEBROOT:-/var/www/letsencrypt}"
CERTBOT_EMAIL="${CERTBOT_EMAIL:-${1:-}}"

if [[ -z "$CERTBOT_EMAIL" ]]; then
  echo "usage: CERTBOT_EMAIL=ops@example.com $0" >&2
  exit 2
fi

"$AEON_SCRIPT_DIR/configure-nginx.sh" http

domains=()
for domain in "$API_DOMAIN" "$GAME_DOMAIN"; do
  if [[ -n "$domain" ]]; then
    domains+=("-d" "$domain")
  fi
done

aeon_sudo certbot certonly \
  --webroot \
  --webroot-path "$ACME_WEBROOT" \
  --cert-name "$CERT_NAME" \
  "${domains[@]}" \
  --email "$CERTBOT_EMAIL" \
  --agree-tos \
  --no-eff-email \
  --non-interactive

"$AEON_SCRIPT_DIR/configure-nginx.sh" ssl

echo "certificate issued and nginx switched to ssl mode"
