#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

mode="${1:-http}"
case "$mode" in
  http|ssl) ;;
  *)
    echo "usage: $0 {http|ssl}" >&2
    exit 2
    ;;
esac

aeon_load_env

API_DOMAIN="${API_DOMAIN:-api.aeonblight.com}"
GAME_DOMAIN="${GAME_DOMAIN:-game.aeonblight.com}"
CERT_NAME="${CERT_NAME:-aeonblight-api-game}"
ACME_WEBROOT="${ACME_WEBROOT:-/var/www/letsencrypt}"
LE_LIVE_DIR="${LE_LIVE_DIR:-/etc/letsencrypt/live/$CERT_NAME}"
ACCOUNT_API_PORT="${ACCOUNT_API_PORT:-8081}"
ECONOMY_API_PORT="${ECONOMY_API_PORT:-8082}"
ADMIN_API_PORT="${ADMIN_API_PORT:-8083}"
GAME_UPSTREAM="${GAME_UPSTREAM:-http://127.0.0.1:7777}"

if [[ "$mode" == "ssl" && ! -f "$LE_LIVE_DIR/fullchain.pem" ]]; then
  echo "missing certificate: $LE_LIVE_DIR/fullchain.pem" >&2
  echo "run ops/issue-cert.sh first" >&2
  exit 2
fi

template_dir="$AEON_ROOT/nginx"
if [[ ! -d "$template_dir" ]]; then
  template_dir="$AEON_ROOT/deploy/linux/nginx"
fi
template="$template_dir/aeonblight.$mode.conf.template"

if [[ ! -f "$template" ]]; then
  echo "missing nginx template: $template" >&2
  exit 2
fi

escape_sed() {
  printf '%s' "$1" | sed -e 's/[&|]/\\&/g'
}

ensure_nginx_main_config() {
  if ! command -v nginx >/dev/null 2>&1; then
    echo "nginx is not installed; run sudo ops/install-deps.sh first" >&2
    exit 2
  fi
  aeon_sudo install -d -m 0755 /etc/nginx /etc/nginx/conf.d /etc/nginx/sites-available /etc/nginx/sites-enabled /var/log/nginx
  if [[ -f /etc/nginx/nginx.conf ]]; then
    return 0
  fi
  tmp_conf="$(mktemp)"
  cat >"$tmp_conf" <<'NGINX'
worker_processes auto;
pid /run/nginx.pid;

events {
    worker_connections 1024;
}

http {
    default_type application/octet-stream;
    sendfile on;
    keepalive_timeout 65;
    server_tokens off;

    access_log /var/log/nginx/access.log;
    error_log /var/log/nginx/error.log;

    include /etc/nginx/conf.d/*.conf;
    include /etc/nginx/sites-enabled/*;
}
NGINX
  aeon_sudo install -m 0644 "$tmp_conf" /etc/nginx/nginx.conf
  rm -f "$tmp_conf"
  echo "created minimal /etc/nginx/nginx.conf"
}

ensure_ssl_port_available() {
  if [[ "$mode" != "ssl" ]]; then
    return 0
  fi
  if ! command -v ss >/dev/null 2>&1; then
    return 0
  fi
  local listeners
  listeners="$(ss -lntp 2>/dev/null | awk '$4 ~ /:443$/ {print}' || true)"
  if [[ -n "$listeners" ]] && grep -vq 'nginx' <<<"$listeners"; then
    echo "port 443 is already used by a non-nginx process; nginx cannot serve HTTPS:" >&2
    echo "$listeners" >&2
    echo "stop or move that process, then run $0 ssl again" >&2
    exit 2
  fi
}

rendered="$(mktemp)"
sed \
  -e "s|__API_DOMAIN__|$(escape_sed "$API_DOMAIN")|g" \
  -e "s|__GAME_DOMAIN__|$(escape_sed "$GAME_DOMAIN")|g" \
  -e "s|__ACME_WEBROOT__|$(escape_sed "$ACME_WEBROOT")|g" \
  -e "s|__LE_LIVE_DIR__|$(escape_sed "$LE_LIVE_DIR")|g" \
  -e "s|__ACCOUNT_API_PORT__|$(escape_sed "$ACCOUNT_API_PORT")|g" \
  -e "s|__ECONOMY_API_PORT__|$(escape_sed "$ECONOMY_API_PORT")|g" \
  -e "s|__ADMIN_API_PORT__|$(escape_sed "$ADMIN_API_PORT")|g" \
  -e "s|__GAME_UPSTREAM__|$(escape_sed "$GAME_UPSTREAM")|g" \
  "$template" >"$rendered"

ensure_nginx_main_config
ensure_ssl_port_available
aeon_sudo install -d -m 0755 "$ACME_WEBROOT"
aeon_sudo install -d -m 0755 /etc/nginx/sites-available /etc/nginx/sites-enabled
aeon_sudo install -m 0644 "$rendered" /etc/nginx/sites-available/aeonblight.conf
rm -f "$rendered"

aeon_sudo ln -sfn /etc/nginx/sites-available/aeonblight.conf /etc/nginx/sites-enabled/aeonblight.conf
if [[ "${DISABLE_DEFAULT_NGINX_SITE:-true}" == "true" ]]; then
  aeon_sudo rm -f /etc/nginx/sites-enabled/default
fi

aeon_sudo nginx -t
if command -v systemctl >/dev/null 2>&1; then
  if aeon_sudo systemctl is-active --quiet nginx; then
    aeon_sudo systemctl reload nginx || aeon_sudo systemctl restart nginx
  else
    if ! aeon_sudo systemctl start nginx; then
      echo "failed to start nginx; inspect with:" >&2
      echo "  systemctl status nginx.service" >&2
      echo "  journalctl -xeu nginx.service" >&2
      exit 1
    fi
  fi
else
  aeon_sudo service nginx status >/dev/null 2>&1 && aeon_sudo service nginx reload || aeon_sudo service nginx start
fi

echo "nginx configured in $mode mode"
