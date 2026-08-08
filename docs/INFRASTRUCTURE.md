# Infrastructure

What is actually running, where, and how to operate it. Verified against the
live server on 2026-08-08 — every figure below was read off the box, not
recalled.

## At a glance

| | |
|---|---|
| Public URL | `https://yukbor.duckdns.org` (also `https://51-250-17-150.sslip.io`) |
| Server | Yandex Cloud, `51.250.17.150`, Ubuntu 22.04.5, 4 vCPU / 3.8 GiB RAM, 19 GB disk (6.5 GB used) |
| SSH | `ssh -i ~/.ssh/id_ed25519_personal admin-node@51.250.17.150` (root has no password; `sudo -i`) |
| Repo on server | `/opt/yukbor`, tracking `git@github.com:Nodirxoja/yukbor-backend.git` |
| Runtime | Docker 29.7.2 + Compose, `docker-compose.prod.yml` |
| TLS | Caddy 2, automatic Let's Encrypt, valid to ~5 Nov 2026 |
| Database | Postgres 16-alpine, 8 migrations applied, ~8.5 MB of data |
| Deploy | `cd /opt/yukbor && ./scripts/deploy.sh` |

## Architecture

```
                              internet
                                 │
                          ┌──────┴──────┐
                          │  Caddy 2    │  :80, :443 — the ONLY thing
                          │  (TLS term) │  reachable from outside
                          └──────┬──────┘
                                 │ reverse_proxy
                          ┌──────┴──────┐
                          │  gateway    │  :8080, bound to 127.0.0.1 only
                          │ (reverse    │
                          │  proxy)     │
                          └──┬──┬──┬──┬─┘
              ┌──────────────┘  │  │  └──────────────┐
              ▼                 ▼  ▼                 ▼
         ┌────────┐      ┌─────────┐  ┌──────────────┐  ┌─────────┐
         │  auth  │      │ orders  │  │notifications │  │ reviews │
         │  :8081 │      │  :8082  │  │    :8084     │  │  :8085  │
         └───┬────┘      └────┬────┘  └──────┬───────┘  └────┬────┘
             │                │              │               │
             │           ┌────┴────┐         │               │
             │           │ wallet  │         │               │
             │           │  :8083  │         │               │
             │           └────┬────┘         │               │
             └────────────────┴───────┬───────┴───────────────┘
                                       ▼
                              ┌─────────────────┐
                              │   postgres 16    │  no published port —
                              │  (5 schemas)     │  reachable only on the
                              └─────────────────┘  compose network

  dashboard/dist (static SPA) served directly by Caddy from disk,
  no container of its own.
```

Every service is a ~35 MB Alpine-based Go binary (see image sizes below).
`internal/<svc>` never imports another `internal/<svc>` — the only path
between services is HTTP, with a shared `X-Internal-Token` header for the
`/internal/*` endpoints the gateway does not route at all.

## The routing rule that isn't obvious

`/orders`, `/orders/*`, `/users`, `/users/*` are **both** iOS API endpoints
and dashboard browser routes. Caddy tells them apart by the request, not the
path:

- `Sec-Fetch-Mode: navigate` (every current browser, top-level navigation) or
  `Accept: text/html` → served the dashboard SPA
- everything else (the app's `URLSession`, `fetch`, `curl`) → proxied to the
  gateway as JSON

Do not "simplify" this by routing on path alone — that was tried and broke
every dashboard deep link the day it shipped.

## Services

| Service | Port | Image size | Owns |
|---|---|---|---|
| gateway | 8080 | 28.7 MB | reverse proxy, CORS, `/health/*` aggregation |
| auth | 8081 | 36.7 MB | OTP, MyID KYC, licence registry, tokens, users, admin login |
| orders | 8082 | 37.7 MB | orders, legs, pricing, backhaul |
| wallet | 8083 | 37.5 MB | escrow, commission, payouts, admin stats/transactions |
| notifications | 8084 | 36.8 MB | notification feed, WebSocket hub |
| reviews | 8085 | 37.5 MB | reviews, rating aggregate |
| migrate | — | 34.2 MB | one-shot; applies `migrations/*.sql`, then exits |
| caddy | 80, 443 | (upstream) | TLS, static SPA, routing |
| postgres | 5432 (internal only) | (upstream) | 5 schemas: auth, orders, wallet, notifications, reviews |

Total image footprint: ~280 MB. The whole stack idles around 700 MB of RAM.

## Domains & TLS

`SITE_ADDRESS` in `.env` holds a **comma-separated list**; Caddy obtains a
separate Let's Encrypt certificate for each name and serves the same site on
all of them:

```
SITE_ADDRESS=yukbor.duckdns.org, 51-250-17-150.sslip.io
```

This is why adding a bought domain later is a one-line change, not a
migration: append the name, `docker compose -f docker-compose.prod.yml up -d
caddy`, done. Nothing already pointed at the old names stops working.

`yukbor.duckdns.org` is a free DNS record we control (via duckdns.org), not a
domain we own. Buy `yukbor.uz` (or similar) before treating this as a
permanent address.

## Data

- **Postgres 16-alpine**, single database `yukbor`, 5 schemas (one per
  service, `pkg/db` migration runner tracks applied versions in
  `public.schema_migrations`).
- **8 migrations applied**: `0001_auth` … `0008_wallet_settings`.
- **No published port** — reachable only on the Docker compose network.
  `docker compose exec postgres psql -U yukbor -d yukbor` from the host.
- **Current size**: ~8.5 MB (seed + test data from development).
- **No backup is configured.** `docs/DEPLOY.md` has the one-line `pg_dump`;
  there is no cron running it. This is the single biggest operational gap —
  fix before this holds anything that matters.

## Secrets

Generated once by `scripts/server-setup.sh`, live in `/opt/yukbor/.env`
(mode 600, gitignored, never committed):

| Variable | Purpose |
|---|---|
| `POSTGRES_PASSWORD` | Postgres auth. **Never regenerate after first start** — it would lock every service out of the existing data volume. |
| `JWT_SECRET` | HS256 signing key, shared by all services and the WebSocket. |
| `INTERNAL_TOKEN` | Shared secret for service-to-service `/internal/*` calls. |
| `ADMIN_USERNAME` / `ADMIN_PASSWORD` | Dashboard sign-in (`POST /auth/admin/login`) — separate from the app's phone/OTP flow. |
| `ADMIN_TOKEN` | A long-lived admin JWT for scripting; not used by the dashboard itself. |
| `TEST_PHONES` / `TEST_OTP_CODE` | See "Known deviations from a secure default" below. |
| `OTP_RATE_LIMIT` | Codes per phone per 10 min; `0` disables the limit. |
| `SITE_ADDRESS` / `ACME_EMAIL` | Caddy's certificate config. |

A bcrypt hash was tried in `.env` once and broke the deploy twice — Compose
interpolates `$` in both `.env` and `env_file` values. There is currently no
bcrypt hash in the config (dashboard auth is username/password checked
server-side, not basic-auth), but if one is ever needed again it must go into
a Caddy config snippet mounted as a file, never an environment variable.

## Known deviations from a secure default

Both are **temporary, explicit, and logged on every service start** — not
accidental:

- **`TEST_PHONES=*`** — the fixed code `0000` is currently accepted for *any*
  phone number, because no SMS gateway is wired yet
  (`internal/auth/otp.go`'s `SMSSender` is still `LogSender`). This means
  **phone verification is effectively off**: anyone can register or sign in
  as any number. `auth` logs `PHONE VERIFICATION IS DISABLED` on every
  startup while this is set.
- **`OTP_RATE_LIMIT=0`** — no limit on OTP requests per phone. Logged as `OTP
  rate limiting is OFF` on every startup.

**Before real users touch this system**: implement `EskizSender` (or Play
Mobile) behind the existing `SMSSender` interface, then set
`TEST_PHONES=` (empty) and `OTP_RATE_LIMIT=3`.

## Firewall

`ufw`: only 22 (SSH), 80, 443 open, both IPv4 and IPv6. Postgres and the
gateway are not exposed by `ufw` *or* by Docker port publishing — the gateway
binds `127.0.0.1:8080` in `docker-compose.prod.yml`, so there is no
port-publishing mistake that could expose it even if the firewall were
disabled.

## Operating

```bash
# ship a change: pulls, rebuilds, migrates, restarts, verifies /health
cd /opt/yukbor && ./scripts/deploy.sh

# rebuild the dashboard only (after a frontend-only change)
./scripts/build-dashboard.sh

# logs
docker compose -f docker-compose.prod.yml logs -f gateway
docker compose -f docker-compose.prod.yml logs caddy --tail=50   # TLS issues

# psql without exposing a port
docker compose -f docker-compose.prod.yml exec postgres psql -U yukbor -d yukbor

# backup (not automated — run manually until a cron exists)
docker compose -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U yukbor yukbor | gzip > backup-$(date +%F).sql.gz

# smoke test / full mobile API contract check
./scripts/smoke.sh https://yukbor.duckdns.org
./scripts/test-mobile-api.sh https://yukbor.duckdns.org
```

`scripts/build-dashboard.sh` runs the Vite build in a throwaway
`node:20-alpine` container (no Node installed on the host) as the invoking
UID/GID — running it as root once left `dist/` and `node_modules/`
root-owned and broke every subsequent build with a silent permission error.
The script now refuses to start if either directory isn't writable.

## API surface

- **OpenAPI spec**: `docs/openapi.yaml` — 30 paths, 33 operations, 23
  schemas, mechanically checked against the routes the Go services actually
  register (zero drift either direction).
- **Interactive docs**: `https://yukbor.duckdns.org/docs` (Swagger UI,
  vendored — not CDN-loaded — with "try it out" live against production).
- **Contract test**: `scripts/test-mobile-api.sh` — 85 assertions covering
  every endpoint the iOS app calls, happy paths and refusals, run against a
  live deployment. Currently 85/85 passing.

## What is NOT automated

- **No CI.** Deploys are `git push` + SSH + `./scripts/deploy.sh`, by hand.
- **No database backups.** See "Data" above.
- **No monitoring/alerting.** Health is checked by running
  `scripts/smoke.sh`, not by anything watching continuously.
- **No log aggregation.** `docker compose logs` on the box is the only view.
- **No autoscaling** — one server, one instance of each service. Fine at
  hackathon scale; would need the notifications WebSocket hub (currently
  in-memory, single-replica by design) reworked before running >1 instance.

## History

Repo was rewritten from `adam-algogroup/yukbor-backend` (public, now
deletable) to `Nodirxoja/yukbor-backend` — commit authorship rewritten to
`Nodirxoja <nodmex2004@gmail.com>`, `Co-Authored-By` trailers stripped. See
`git log` for the full build history: five phases from an
empty-database skeleton to this deployment, then dashboard and mobile-API
hardening.
