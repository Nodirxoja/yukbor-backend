#!/usr/bin/env bash
# Pre-demo sanity check: every service alive behind the gateway.
# Usage: ./scripts/smoke.sh [base-url]   (default http://localhost:8080)
set -uo pipefail

BASE="${1:-http://localhost:8080}"
fail=0

check() {
  local name="$1" path="$2"
  local body status
  body=$(curl -fsS --max-time 5 "$BASE$path" 2>/dev/null)
  status=$?
  if [ $status -eq 0 ]; then
    printf '  ok    %-16s %s\n' "$name" "$body"
  else
    printf '  FAIL  %-16s (curl exit %d)\n' "$name" "$status"
    fail=1
  fi
}

echo "smoke: $BASE"
check gateway       /health
check auth          /health/auth
check orders        /health/orders
check wallet        /health/wallet
check notifications /health/notifications
check reviews       /health/reviews

if [ $fail -ne 0 ]; then
  echo
  echo "stack is NOT healthy — try: docker compose ps && docker compose logs --tail=50"
  exit 1
fi
echo "all services healthy"
