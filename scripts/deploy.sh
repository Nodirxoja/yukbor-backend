#!/usr/bin/env bash
# Build and (re)start the production stack, then prove it is actually serving.
# Idempotent — this is the normal way to ship a change.
#
#   ./scripts/deploy.sh            pull latest, rebuild, restart, verify
#   ./scripts/deploy.sh --no-pull  rebuild what is already checked out
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
COMPOSE="docker compose -f docker-compose.prod.yml"

[ -f .env ] || { echo "no .env — run ./scripts/server-setup.sh <site-address> first" >&2; exit 1; }
# shellcheck disable=SC1091
. ./.env

if [ "${1:-}" != "--no-pull" ]; then
  echo "==> pulling"
  git pull --ff-only
fi

echo "==> building"
$COMPOSE build

echo "==> starting (migrations run first and must exit 0)"
$COMPOSE up -d

echo "==> waiting for the stack to report healthy"
for i in $(seq 1 60); do
  if curl -fsS --max-time 3 http://127.0.0.1:8080/health >/dev/null 2>&1; then
    echo "    gateway up after ${i}s"
    break
  fi
  sleep 1
  [ "$i" = 60 ] && { echo "    gateway never came up"; $COMPOSE ps; $COMPOSE logs --tail=40; exit 1; }
done

echo "==> service health, through the gateway"
./scripts/smoke.sh http://127.0.0.1:8080

echo "==> over TLS, from the outside"
if curl -fsS --max-time 10 "https://$SITE_ADDRESS/health" >/dev/null 2>&1; then
  curl -sS "https://$SITE_ADDRESS/health" | sed 's/^/    /'
  echo
else
  echo "    https://$SITE_ADDRESS/health not answering yet."
  echo "    On a first deploy Caddy needs a minute to get a certificate; check:"
  echo "      $COMPOSE logs caddy --tail=30"
fi

echo "==> running containers"
$COMPOSE ps --format 'table {{.Service}}\t{{.Status}}'
