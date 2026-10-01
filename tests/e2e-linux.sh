#!/usr/bin/env sh
set -eu
repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo"
command -v docker >/dev/null 2>&1 || { echo "docker is required" >&2; exit 1; }
docker compose version >/dev/null
if [ ! -s configs/server.crt ] || [ ! -s configs/server.key ]; then
  sh tests/gen-certs.sh
fi
export GHOST_PSK=${GHOST_PSK:-$(openssl rand -hex 32)}
cleanup() { docker compose -f deployments/compose.yml down --remove-orphans; }
trap cleanup EXIT INT TERM
docker compose -f deployments/compose.yml up -d --build
i=0
until curl --silent --show-error --output /dev/null --max-time 15 \
  --proxy socks5h://127.0.0.1:1080 https://example.com/ >/dev/null; do
  i=$((i+1)); [ "$i" -lt 30 ] || exit 1
  sleep 1
done
echo "Compose SOCKS-to-gateway E2E passed"
