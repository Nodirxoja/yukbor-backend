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

# Docker recreates a missing bind-mount source as root, so dist/ can end up
# owned by root after a container restart — and then the build writes nothing
# and says almost nothing about why. Check it up front.
for d in dist node_modules; do
  if [ -e "$d" ] && [ ! -w "$d" ]; then
    echo "  $d/ is not writable by $(whoami) — a root-run container created it."
    echo "  Fix with: sudo chown -R \$(id -u):\$(id -g) $(pwd)"
    exit 1
  fi
done

echo "==> removing any local dev env (it would be inlined into a public bundle)"
rm -f .env.local .env.production.local

echo "==> building (real backend, mocks off)"
# Stated explicitly, not left to a default: this build must never ship mocks.
export VITE_USE_MOCKS=false
if command -v npm >/dev/null 2>&1; then
  npm ci --silent && npm run build
else
  # No node on the server: build in a throwaway container. Run it as the host
  # user, or dist/ comes out root-owned and every later step — copying the API
  # reference in, the next deploy's rebuild — fails with permission denied.
  docker run --rm -e VITE_USE_MOCKS=false -e HOME=/tmp \
    -u "$(id -u):$(id -g)" -v "$PWD":/app -w /app node:20-alpine \
    sh -c 'npm ci && npm run build'
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

echo "==> checking no mock data was bundled"
# Mock records carry names that exist nowhere else; finding one means the
# build fell back to mock mode and would show fabricated orders as real.
if grep -rq 'Dilnoza Yusupova' dist/ 2>/dev/null; then
  echo "  BUILD REJECTED — mock data is in the bundle (VITE_USE_MOCKS was not false)."
  exit 1
fi
echo "    clean — no mock records in dist/"

echo "==> bundling the API reference"
# Swagger UI is VENDORED, not loaded from a CDN: the reference has to work on
# venue wifi, on a plane, and after the CDN this was pinned to changes its
# paths. It is ~1MB of static files, which is a cheap price for that.
mkdir -p dist/docs
cp ../docs/openapi.yaml dist/openapi.yaml
for f in swagger-ui.css swagger-ui-bundle.js swagger-ui-standalone-preset.js; do
  cp "node_modules/swagger-ui-dist/$f" "dist/docs/$f"
done
cat > dist/docs/index.html <<'HTML'
<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>YUK BOR API</title>
    <link rel="stylesheet" href="/docs/swagger-ui.css" />
    <style>
      body { margin: 0; background: #fafafa; }
      .topbar { display: none; }
    </style>
  </head>
  <body>
    <div id="swagger"></div>
    <script src="/docs/swagger-ui-bundle.js"></script>
    <script src="/docs/swagger-ui-standalone-preset.js"></script>
    <script>
      window.ui = SwaggerUIBundle({
        url: '/openapi.yaml',
        dom_id: '#swagger',
        deepLinking: true,
        // "Try it out" hits this same origin, so the examples are live rather
        // than illustrative.
        presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
        layout: 'BaseLayout',
        persistAuthorization: true,
        defaultModelsExpandDepth: 1,
        docExpansion: 'list',
        tryItOutEnabled: true,
      })
    </script>
  </body>
</html>
HTML
echo "    /docs and /openapi.yaml bundled"

du -sh dist | sed 's/^/    /'
