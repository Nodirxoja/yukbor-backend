# YUK BOR — Backend (Go microservices monorepo)

Backend for the YUK BOR platform connecting clients with truck drivers,
special-equipment providers, and labor teams in Uzbekistan.

- **Plan**: [`docs/DEVELOPMENT_PLAN.md`](docs/DEVELOPMENT_PLAN.md) — read this first
- **Contract**: [`docs/API_CONTRACT.md`](docs/API_CONTRACT.md) — the iOS ⇄ backend contract, source of truth

## Services

| Service | Port | Responsibility |
|---|---|---|
| gateway | 8080 | reverse proxy — single base URL for iOS, CORS, logging |
| auth | 8081 | OTP, MyID KYC, registration, tokens, users |
| orders | 8082 | orders + per-leg lifecycle, backhaul search |
| wallet | 8083 | escrow per (orderId, payeeId) |
| notifications | 8084 | notification feed + WebSocket hub |
| reviews | 8085 | reviews + rating aggregate |
| dashboard | 5173 (dev) | admin web dashboard — map, orders, users, money stats (`dashboard/`, Vite + Radix UI) |

REST/JSON everywhere, including service-to-service (with `X-Internal-Token`).
One Postgres, one schema per service. One Go module; `internal/<svc>` never
imports another `internal/<svc>` — cross-service = HTTP only.

## Quick start

```bash
make build        # compiles all services (stdlib only — no deps to fetch)
make test         # unit tests
make up           # postgres + all six services via docker compose
curl localhost:8080/health
```

Every contract endpoint is already routed and returns `501 NOT_IMPLEMENTED`
until implemented — see `TODO(day-N)` markers matching the plan's milestones.

## Where things live

```
cmd/<service>/          entry points (one binary each)
dashboard/              admin dashboard (Vite + React + Radix UI + Leaflet), mock mode by default
internal/<service>/     handlers + business logic
pkg/models/             wire types — MUST match iOS Core/Model/*.swift 1:1
pkg/httpx|jwtx|config/  shared toolkit (error format, JWT HS256, env)
migrations/             plain SQL, one file per schema, auto-applied by compose
```

## Verification flows (Uzbekistan, contract v1.1)

Registration for all roles: phone OTP → **MyID KYC** (`POST /auth/myid/verify`,
passport + selfie proxied to the MyID B2B API) → `POST /auth/register` with the
short-lived `myIdVerificationToken` → `verificationStatus = approved`.

Hackathon rule: **the system must feel whole** — every external integration
(MyID, license registry, payments, SMS) is a high-fidelity simulation behind
the interface the real client will use later. Flows, tokens, rules, and
failure modes actually run; only the upstream is imitated. Each simulation
has deterministic failure triggers so rejection paths can be demoed on stage
(plan §10 has the full table):

- MyID: passport `0000000` → `PASSPORT_NOT_FOUND`; PINFL `99...` → `FACE_MISMATCH`.
- License registry (enforced for drivers/equipment): PINFL ending in an odd
  digit → only category B → `LICENSE_CATEGORY_MISMATCH`, registration rejected;
  even digit → B/C/CE → approved, proceed to truck selection.
- Payments: amount `999999999` → `PAYMENT_DECLINED`; otherwise simulated
  charge with a provider reference, real escrow ledger.
