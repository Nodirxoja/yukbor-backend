#!/usr/bin/env bash
# Seed demo data on a production box, without ever leaving it in dev mode.
#
# scripts/seed.sh drives the real registration flow, which needs the OTP back —
# and APP_ENV=prod deliberately withholds it. So this flips to dev just long
# enough to seed, then flips back and verifies the shortcuts are shut again.
# The seeded data persists across the flip; only behaviour changes.
#
#   ./scripts/seed-prod.sh
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
COMPOSE="docker compose -f docker-compose.prod.yml"
BASE="http://127.0.0.1:8080"

[ -f .env ] || { echo "no .env — run scripts/server-setup.sh first" >&2; exit 1; }

restore_prod() {
  sed -i 's|^APP_ENV=.*|APP_ENV=prod|' .env
  $COMPOSE up -d auth >/dev/null
  for _ in $(seq 1 30); do
    curl -fsS --max-time 2 "$BASE/health/auth" >/dev/null 2>&1 && break
    sleep 1
  done
}
# Any failure — including Ctrl-C — must still leave the box in prod.
trap restore_prod EXIT

echo "==> switching auth to dev so the OTP is returned"
sed -i 's|^APP_ENV=.*|APP_ENV=dev|' .env
$COMPOSE up -d auth >/dev/null
for _ in $(seq 1 30); do
  curl -fsS --max-time 2 "$BASE/health/auth" >/dev/null 2>&1 && break
  sleep 1
done

echo "==> seeding"
./scripts/seed.sh "$BASE"

echo
echo "==> minting a long-lived admin token for the dashboard"
if [ -f .seed-tokens ]; then
  ADMIN_ID=$(grep '^ADMIN_ID=' .seed-tokens | cut -d= -f2)
  if [ -n "$ADMIN_ID" ]; then
    ./scripts/mint-admin-token.sh "$ADMIN_ID" 365 --write
  else
    echo "  could not read ADMIN_ID from .seed-tokens — mint manually:"
    echo "    ./scripts/mint-admin-token.sh <admin-uuid> 365 --write"
  fi
else
  echo "  .seed-tokens not written — mint manually once you have the admin id"
fi

echo
echo "==> restoring APP_ENV=prod"
restore_prod
trap - EXIT

echo "==> verifying the demo shortcuts are shut"
CODE=$(curl -sS -X POST "$BASE/auth/otp/request" -H 'Content-Type: application/json' \
       -d '{"phoneNumber":"+998900000999"}' | grep -o '"devCode"' || true)
if [ -n "$CODE" ]; then
  echo "  !! devCode is STILL being returned — the box is not in prod mode"
  exit 1
fi
echo "  devCode absent from OTP responses — prod confirmed"
grep '^APP_ENV=' .env | sed 's/^/  /'
