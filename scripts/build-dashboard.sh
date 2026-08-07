#!/usr/bin/env bash
# Build the dashboard for production and PROVE no admin token got baked in.
#
# Vite inlines every VITE_* value into the JavaScript bundle, and that bundle is
# served publicly. scripts/seed.sh writes dashboard/.env.local with a real
# VITE_ADMIN_TOKEN for local development — if that file is still lying around at
# build time, Vite happily embeds an admin JWT into a public asset. That is not
# hypothetical: it happened on the first production build, because seeding runs
# before building.
#
# So this removes the file first and then greps the output for a JWT, failing
# the build if one is found. The check is the point; the removal is just tidying.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../dashboard"

echo "==> removing any local dev env (it would be inlined into a public bundle)"
rm -f .env.local .env.production.local

echo "==> building"
if command -v npm >/dev/null 2>&1; then
  npm ci --silent && npm run build
else
  # No node on the server: build in a throwaway container.
  docker run --rm -v "$PWD":/app -w /app node:20-alpine \
    sh -c 'npm ci --silent && npm run build'
fi

echo "==> checking the bundle for leaked credentials"
LEAKS=$(grep -rlE 'eyJhbGciOiJIUzI1NiI|VITE_ADMIN_TOKEN' dist/ 2>/dev/null || true)
if [ -n "$LEAKS" ]; then
  echo
  echo "  BUILD REJECTED — an admin JWT is embedded in these public assets:"
  printf '    %s\n' $LEAKS
  echo
  echo "  The dashboard must ship with NO token; the reverse proxy injects one."
  echo "  Delete dashboard/.env.local and rebuild."
  exit 1
fi
echo "    clean — no JWT in dist/"

du -sh dist | sed 's/^/    /'
