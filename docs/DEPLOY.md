# Deploying

Target: a plain Ubuntu server. Everything runs in Docker; the only thing
exposed to the internet is Caddy, which terminates TLS.

```
internet ──443──▶ caddy ──▶ gateway:8080 ──▶ auth / orders / wallet
                    │                          notifications / reviews
                    └──▶ /srv/dashboard             │
                         (static SPA)          postgres (no published port)
```

## First deploy

```bash
# on the server
sudo apt-get update && sudo apt-get install -y git
git clone https://github.com/Nodirxoja/yukbor-backend.git /opt/yukbor
cd /opt/yukbor

./scripts/server-setup.sh <site-address> [acme-email]
./scripts/deploy.sh --no-pull
```

`server-setup.sh` generates `.env` with real random secrets, creates the
dashboard's basic-auth password, and opens 22/80/443. It is safe to re-run: an
existing `.env` is never overwritten, because changing `POSTGRES_PASSWORD`
after the data volume exists would lock the services out of their own database.

## Shipping a change

```bash
cd /opt/yukbor && ./scripts/deploy.sh
```

Pulls, rebuilds, runs migrations (a one-shot container every service waits on),
restarts, and verifies `/health` locally and over TLS.

## The site address, before you own a domain

iOS App Transport Security refuses plain HTTP, so the app needs real HTTPS from
day one. Until a domain is bought, use an **sslip.io** name — it resolves any
`a-b-c-d.sslip.io` to `a.b.c.d`, and Let's Encrypt issues certificates for it:

```
./scripts/server-setup.sh 51-250-17-150.sslip.io
```

Moving to a real domain later: point an A record at the server, then

```bash
sed -i 's|^SITE_ADDRESS=.*|SITE_ADDRESS=api.yukbor.uz|' .env
docker compose -f docker-compose.prod.yml up -d caddy
```

Caddy fetches the new certificate on start. Nothing else changes.

## Environments and the demo affordances

`APP_ENV` decides whether the demo shortcuts exist:

| | `dev` | `prod` |
|---|---|---|
| `devCode` in the OTP response | returned | **absent** |
| Master OTP code `7777` | accepted | **rejected** |
| `verificationId` on `/auth/login` | optional | **required** |

Production runs `prod`. A public server in `dev` would let anyone read an
OTP out of the API response and sign in as any user, including the admin.

`scripts/seed.sh` needs `devCode`, so seeding a production box is a deliberate
two-step: flip to `dev`, seed, flip back. `scripts/seed-prod.sh` does exactly
that and leaves the box in `prod`.

## Secrets

`.env` is generated on the server and gitignored. It holds
`POSTGRES_PASSWORD`, `JWT_SECRET`, `INTERNAL_TOKEN`, the dashboard's bcrypt
password hash, and `ADMIN_TOKEN`.

The dashboard's admin JWT is **not** built into the front-end bundle. A Vite
`VITE_*` value is inlined into publicly served JavaScript, so anyone could read
it and call `/admin/*`. Instead Caddy holds the token and injects
`Authorization: Bearer …` server-side, behind basic auth — the browser never
sees it. The dashboard is built with no token at all.

## Operating

```bash
docker compose -f docker-compose.prod.yml ps
docker compose -f docker-compose.prod.yml logs -f gateway
docker compose -f docker-compose.prod.yml logs caddy --tail=50   # TLS problems

# psql, without exposing a port
docker compose -f docker-compose.prod.yml exec postgres psql -U yukbor -d yukbor

# backup
docker compose -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U yukbor yukbor | gzip > backup-$(date +%F).sql.gz
```

## Sizing

The whole stack idles in roughly 700 MB: Postgres ~120 MB, each Go service
~15–25 MB, Caddy ~20 MB. A 2 vCPU / 2 GB box is enough to run it; building the
images wants more, so on a small server build elsewhere or add swap.
