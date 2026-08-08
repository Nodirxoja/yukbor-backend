# YUK BOR — Backend (Go microservices monorepo)

Backend for the YUK BOR platform connecting clients with truck drivers,
special-equipment providers, and labor teams in Uzbekistan.

- **Plan**: [`docs/DEVELOPMENT_PLAN.md`](docs/DEVELOPMENT_PLAN.md)
- **Contract**: [`docs/API_CONTRACT.md`](docs/API_CONTRACT.md) — the iOS ⇄ backend contract, frozen
- **Implementation notes**: [`docs/BACKEND_NOTES.md`](docs/BACKEND_NOTES.md) — where the backend
  *adds* to the contract, decisions taken where it was silent, and the demo triggers
- **Infrastructure**: [`docs/INFRASTRUCTURE.md`](docs/INFRASTRUCTURE.md) — what is actually deployed,
  where, and how to operate it
- **API reference**: [`docs/openapi.yaml`](docs/openapi.yaml), served interactively at
  [`/docs`](https://yukbor.duckdns.org/docs) on the live deployment

## Services

| Service | Port | Responsibility |
|---|---|---|
| gateway | 8080 | reverse proxy — single base URL for iOS, CORS, logging |
| auth | 8081 | OTP, MyID KYC, licence registry, registration, tokens, users |
| orders | 8082 | orders + per-leg lifecycle, pricing, backhaul search |
| wallet | 8083 | escrow per (orderId, payeeId), commission, payouts |
| notifications | 8084 | notification feed + WebSocket hub |
| reviews | 8085 | reviews + rating aggregate |
| dashboard | 5173 (dev) | admin dashboard — map, orders, users, money (`dashboard/`) |

REST/JSON everywhere, including service-to-service (with `X-Internal-Token`).
One Postgres, one schema per service. One Go module; `internal/<svc>` never
imports another `internal/<svc>` — cross-service is HTTP only, so splitting the
repo later costs nothing.

## Quick start

```bash
make up          # postgres + migrations + all six services
make smoke       # /health on every service through the gateway
make seed        # believable demo users and orders, via the real endpoints
make demo        # narrated walkthrough of the whole system (press enter to step)
make dashboard   # admin dashboard at http://localhost:5173
```

`make seed` writes `dashboard/.env.local`, so the dashboard shows live data
immediately after. Without it the dashboard runs on mock data.

```bash
make build test  # compile + unit tests
make reset       # start over, dropping the database volume
```

Requires Go 1.25+ and Docker. Migrations are applied by a one-shot `migrate`
service that every other service waits on, tracked in `public.schema_migrations`
— a schema change never needs `docker compose down -v`.

## Verification flows (Uzbekistan, contract v1.1)

Registration for all roles: phone OTP → **MyID KYC** (`POST /auth/myid/verify`,
passport + selfie) → `POST /auth/register` with the short-lived
`myIdVerificationToken` → `verificationStatus = approved`.

Hackathon rule: **the system must feel whole** — every external integration
(MyID, licence registry, payments, SMS) is a high-fidelity simulation behind
the interface the real client will use later. The flows, tokens, TTLs, rules,
and failure modes actually run; only the upstream is imitated. Each simulation
fails on demand so rejection paths can be demoed:

- MyID: passport `0000000` → `PASSPORT_NOT_FOUND`; PINFL `99…` → `FACE_MISMATCH`.
- Licence registry (enforced for drivers/equipment providers): the PINFL's last
  digit decides the licence — odd → `["B"]` (registration rejected), 0/2/4 →
  `["B","C"]` (registers, but refused a tractor-trailer load), 6/8 →
  `["B","C","CE"]`. The registry also issues a licence number and an Uzbek
  plate, both derived from the PINFL so demo runs are reproducible.
- Payments: `999999999` → `PAYMENT_DECLINED`, and the leg claim is rolled back.
- SMS: the code is logged, returned as `devCode` outside prod, and `7777`
  always works — on-stage registration never stalls on delivery.

Full trigger table in [`docs/BACKEND_NOTES.md`](docs/BACKEND_NOTES.md) §5.

## Where things live

```
cmd/<service>/          entry points (one binary each) + cmd/migrate
dashboard/              admin dashboard (Vite + React + Radix UI + Leaflet)
internal/<service>/     handlers, stores, business logic
pkg/models/             wire types — MUST match iOS Core/Model/*.swift 1:1
pkg/db/                 pgx pool + migration runner
pkg/httpx/              error envelope + codes, auth middleware, JSON helpers
pkg/jwtx/               HS256 issue/verify (stdlib only)
pkg/svc/                service-to-service client
migrations/             plain SQL, numbered, applied once each
scripts/                smoke.sh · seed.sh · demo.sh · wslisten/
```

## Things worth knowing

**Legs are rows, not columns.** Accept, status and escrow all operate per leg,
so claiming one is a single conditional `UPDATE` — two drivers racing the same
leg cannot both win. The API layer flattens legs back into the contract's
`assignedDriverId` / `equipmentStatus` / … fields.

**Money is int64 UZS end to end**, decimal strings on the wire, never floats.
A combo order carries one `priceEstimate` but pays 2–3 executors: the server
splits it per leg in proportion to the pricing formula and guarantees the parts
sum to the total exactly. If escrow cannot be opened, the leg claim is rolled
back — a leg never sits assigned with no money behind it.

**Tariffs and commission live in tables** (`orders.tariffs`,
`wallet.settings`), read per request, so they can be changed live without a
rebuild.

**Checking the realtime channel** takes five seconds, no iOS app needed:

```bash
go run ./scripts/wslisten -token "$ACCESS_TOKEN"
```
