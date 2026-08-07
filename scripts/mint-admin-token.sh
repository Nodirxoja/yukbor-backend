#!/usr/bin/env bash
# Mint a long-lived admin JWT directly from JWT_SECRET, without a login flow.
#
# Why this exists: access tokens last 24h (auth.AccessTokenTTL), so an admin
# token captured during seeding would leave the dashboard broken by the next
# morning. On the server we hold the signing secret, so we can issue a token
# with a lifetime that suits a back office instead of a phone.
#
# The token is HS256 — the same algorithm pkg/jwtx verifies — built here with
# openssl so this needs no Go toolchain on the server.
#
#   ./scripts/mint-admin-token.sh <user-uuid> [days]     # prints the token
#   ./scripts/mint-admin-token.sh <user-uuid> 365 --write # also updates .env
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

USER_ID="${1:-}"
DAYS="${2:-365}"
WRITE="${3:-}"

if [ -z "$USER_ID" ]; then
  echo "usage: $0 <user-uuid> [days] [--write]" >&2
  exit 2
fi
[ -f .env ] || { echo "no .env — run scripts/server-setup.sh first" >&2; exit 1; }
# shellcheck disable=SC1091
. ./.env
[ -n "${JWT_SECRET:-}" ] || { echo "JWT_SECRET missing from .env" >&2; exit 1; }

b64url() { openssl base64 -A | tr '+/' '-_' | tr -d '='; }

NOW=$(date +%s)
EXP=$((NOW + DAYS * 86400))

HEADER=$(printf '%s' '{"alg":"HS256","typ":"JWT"}' | b64url)
PAYLOAD=$(printf '{"sub":"%s","role":"admin","typ":"access","exp":%s,"iat":%s}' \
          "$USER_ID" "$EXP" "$NOW" | b64url)
SIGNING_INPUT="$HEADER.$PAYLOAD"
SIG=$(printf '%s' "$SIGNING_INPUT" \
      | openssl dgst -sha256 -hmac "$JWT_SECRET" -binary | b64url)
TOKEN="$SIGNING_INPUT.$SIG"

if [ "$WRITE" = "--write" ]; then
  if grep -q '^ADMIN_TOKEN=' .env; then
    # '|' as the sed delimiter: a JWT contains '/' but never '|'.
    sed -i "s|^ADMIN_TOKEN=.*|ADMIN_TOKEN=$TOKEN|" .env
  else
    printf 'ADMIN_TOKEN=%s\n' "$TOKEN" >> .env
  fi
  echo "  ADMIN_TOKEN written to .env (valid $DAYS days, until $(date -d "@$EXP" 2>/dev/null || date -r "$EXP"))"
  echo "  reloading caddy so it picks up the new token"
  docker compose -f docker-compose.prod.yml up -d caddy >/dev/null
  echo "  done"
else
  printf '%s\n' "$TOKEN"
fi
