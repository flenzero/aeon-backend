#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
  echo "rerun with sudo or as root" >&2
  exit 2
fi

if [[ -r /etc/os-release ]]; then
  # shellcheck disable=SC1091
  source /etc/os-release
else
  echo "cannot detect Linux distribution: /etc/os-release is missing" >&2
  exit 2
fi

install_debian() {
  apt-get update
  apt-get install -y ca-certificates curl gnupg lsb-release nginx certbot python3-certbot-nginx postgresql-client openssl tar gzip

  if ! command -v docker >/dev/null 2>&1; then
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL "https://download.docker.com/linux/${ID}/gpg" -o /etc/apt/keyrings/docker.asc
    chmod a+r /etc/apt/keyrings/docker.asc
    arch="$(dpkg --print-architecture)"
    codename="${VERSION_CODENAME:-$(lsb_release -cs)}"
    printf 'deb [arch=%s signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/%s %s stable\n' "$arch" "$ID" "$codename" >/etc/apt/sources.list.d/docker.list
    apt-get update
    apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  fi
}

install_rhel() {
  pkg_mgr="dnf"
  if ! command -v dnf >/dev/null 2>&1; then
    pkg_mgr="yum"
  fi
  "$pkg_mgr" install -y ca-certificates curl gnupg nginx certbot postgresql openssl tar gzip
  if ! command -v docker >/dev/null 2>&1; then
    "$pkg_mgr" install -y yum-utils
    yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
    "$pkg_mgr" install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  fi
}

case "${ID_LIKE:-$ID}" in
  *debian*|*ubuntu*)
    install_debian
    ;;
  *rhel*|*fedora*|*centos*)
    install_rhel
    ;;
  *)
    echo "unsupported distribution: ${PRETTY_NAME:-$ID}" >&2
    echo "install Docker Engine, Compose plugin, nginx, certbot, and psql manually, then continue" >&2
    exit 2
    ;;
esac

systemctl enable --now docker
systemctl enable --now nginx

if [[ -n "${SUDO_USER:-}" && "$SUDO_USER" != "root" ]]; then
  usermod -aG docker "$SUDO_USER" || true
fi

docker --version
docker compose version
nginx -v
certbot --version
psql --version

echo "dependencies installed"
echo "log out and back in if your user was just added to the docker group"
