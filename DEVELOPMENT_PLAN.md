# YUK BOR — Backend Development Plan (Hackathon MVP)

Version: 1.1 · Date: 2026-08-07 · Scope: **hackathon MVP**, Go microservices monorepo, REST-only
Companion doc: `API_CONTRACT.md` (iOS ⇄ Backend contract **v1.1**) — the backend must match it 1:1 so the iOS app can swap its mock services for an HTTP client without model changes. v1.1 adds mandatory MyID KYC to registration (`POST /auth/myid/verify` + `myIdVerificationToken` in `/auth/register`).

---

## 1. Goal and guiding constraints

Build the minimal real backend that lets the existing iOS app (currently running on `MockAuthService`, `MockOrderService`, `MockWalletService`, `MockNotificationService`, `MockReviewService`) work end-to-end against HTTP:

- Phone-number registration with SMS OTP for all roles.
- Orders with **three independent legs** (transport / equipment / labor), each with its own executor, status, and escrow transaction.
- Escrow wallet keyed on `(orderId, payeeId)`.
- Notifications with a realtime WebSocket channel.
- Reviews + aggregated rating.
- Backhaul search (open orders near a driver's drop-off point).

Constraints chosen for the hackathon:

- **REST/JSON everywhere** — between iOS and backend *and* between services. No gRPC, no message broker. Internal service-to-service calls are plain HTTP with a shared secret header.
- **One Go module, one repo** (`yukbor-backend`), one binary per service under `cmd/`.
- **One PostgreSQL instance**, one schema per service (`auth`, `orders`, `wallet`, `notifications`, `reviews`) so services stay logically isolated but ops stay trivial.
- **Every external integration is a high-fidelity simulation, not a stub** (see §5, §10): the flows, tokens, rules, and failure modes all actually run — only the upstream (MyID, license registry, payment providers, SMS) is imitated. The demo must look and behave like a whole, finished system; real integrations later replace only the upstream clients behind existing interfaces.

## 2. Architecture overview

```
                        ┌──────────────────────────────────────────────┐
 iOS app ── HTTPS ────▶ │  gateway :8080  (reverse proxy, CORS, logs)  │
                        └───┬────────┬─────────┬──────────┬────────┬───┘
                            ▼        ▼         ▼          ▼        ▼
                         auth     orders    wallet    notifs    reviews
                         :8081    :8082     :8083     :8084     :8085
                            │        │         │          │        │
                            └────────┴────┬────┴──────────┴────────┘
                                          ▼
                                  PostgreSQL :5432
                            (schemas: auth, orders, wallet,
                             notifications, reviews)
```

- **gateway** — a ~100-line `httputil.ReverseProxy` that routes by path prefix (`/auth`,`/users` → auth; `/orders` → orders; `/wallet` → wallet; `/notifications`,`/ws` → notifications; `/reviews` → reviews). Gives iOS a single base URL and one place for CORS/logging. No business logic.
- **JWT validation happens in each service** via the shared `pkg/jwtx` package (HS256, shared secret from env). This avoids making the gateway a smart choke point.
- **Internal calls** (e.g. orders → wallet "release escrow", orders → notifications "emit event") are REST calls with an `X-Internal-Token` header. Fire-and-forget with a short timeout; a failed notification must never fail an order update.

### Service responsibilities

| Service | Port | Owns | Endpoints (from contract) |
|---|---|---|---|
| auth | 8081 | users, OTP, MyID KYC, tokens, verification status | `/auth/otp/request`, `/auth/otp/verify`, `/auth/myid/verify`, `/auth/register`, `/auth/login`, `/auth/refresh`, `/auth/logout`, `GET/PATCH /users/me` |
| orders | 8082 | orders, legs, statuses, backhaul search | `POST/GET /orders`, `/orders/available`, `/orders/{id}`, `/accept`, `/status`, `/cancel`, `/confirm-completion`, `/orders/backhaul` |
| wallet | 8083 | escrow transactions | `POST /wallet/transactions`, `/wallet/transactions/release`, `GET /wallet/transactions` |
| notifications | 8084 | notifications, WebSocket hub | `GET /notifications`, `PATCH /notifications/{id}/read`, `GET /ws?userId=` + internal `POST /internal/events` |
| reviews | 8085 | reviews, rating aggregates | `POST /reviews`, `GET /reviews/rating` |

Why 5 services and not 8: at hackathon scale, each service should map to one bounded context from the contract, nothing smaller. Users live inside auth (they share the identity lifecycle). Reviews stay separate only because their aggregate feeds `User.rating` asynchronously — and it is the easiest service to hand to a second developer.

## 3. Monorepo layout

```
yukbor-backend/
├── go.mod                     # single module: github.com/aventiseld/yukbor-backend
├── Makefile                   # run/build/test/migrate targets
├── docker-compose.yml         # postgres + all 6 binaries
├── Dockerfile                 # one multi-stage Dockerfile, ARG SERVICE
├── .env.example
├── docs/
│   ├── DEVELOPMENT_PLAN.md    # this file
│   └── API_CONTRACT.md        # copy of the iOS contract
├── cmd/                       # one main.go per deployable
│   ├── gateway/  auth/  orders/  wallet/  notifications/  reviews/
├── dashboard/                 # admin web dashboard (Vite + React + Radix UI, §11)
├── internal/                  # business logic, one package per service
│   ├── auth/      handler.go store.go otp.go verification.go
│   ├── orders/    handler.go store.go legs.go backhaul.go
│   ├── wallet/    handler.go store.go
│   ├── notifications/ handler.go hub.go store.go
│   ├── reviews/   handler.go store.go
│   └── gateway/   proxy.go
├── pkg/                       # shared, dependency-light packages
│   ├── httpx/     JSON write/read helpers, error format, middleware
│   ├── jwtx/      HS256 issue/verify (stdlib only)
│   ├── models/    enums + DTOs mirroring the Swift models 1:1
│   └── config/    env loading
└── migrations/                # plain SQL, numbered, one file per schema
```

Rules of the monorepo:

- `internal/<svc>` may import `pkg/*` but **never** another `internal/<other-svc>` — cross-service communication is HTTP only, even in the monorepo. This keeps the future option to split repos.
- `pkg/models` is the single source of truth for enums (`OrderStatus`, `OrderType`, `EquipmentType`, `VehicleType`, `PaymentMethod`, `UserRole`, `Leg`) and must stay field-for-field identical to `Core/Model/*.swift` on iOS.
- Zero heavy frameworks: Go 1.22+ stdlib `net/http` mux (method+path patterns), `database/sql` + `pgx` stdlib driver. The skeleton compiles with stdlib only.

## 4. Data model (PostgreSQL)

One database `yukbor`, five schemas. Key tables:

**auth schema**: `users` (id uuid PK, role, full_name, phone_number unique, email, is_verified, verification_status, rating, ratings_count, created_at), `otp_codes` (verification_id PK, phone, code_hash, expires_at, attempts), `refresh_tokens` (token_hash PK, user_id, expires_at, revoked_at), `verification_documents` (post-MVP: id, user_id, kind passport|driver_license|vehicle_passport, file_url, status).

**orders schema**: `orders` (id, client_id, client_name, type, pickup/dropoff address + lat/lng, scheduled_date, price_estimate numeric, currency, cargo_* columns nullable, equipment_* nullable, labor_* nullable, created_at, updated_at) and `order_legs` (order_id, leg transport|equipment|labor, status, executor_id, executor_name, PRIMARY KEY(order_id, leg)). Modeling legs as rows (not columns) is the one place worth doing properly even at a hackathon — accept/status/escrow all operate per-leg, and `LEG_ALREADY_TAKEN` becomes a simple `UPDATE ... WHERE executor_id IS NULL` returning-row check (no race conditions). The API layer flattens legs back into the contract's `assignedDriverId` / `equipmentStatus` / … fields.

**wallet schema**: `transactions` (id, order_id, order_title, payer_id, payee_id, amount numeric(14,0), platform_commission numeric, payment_method, status held|released|refunded, created_at, released_at, UNIQUE(order_id, payee_id)). Money is `numeric`, serialized as strings — never floats.

**notifications schema**: `notifications` (id, user_id, type, title, body, related_order_id, is_read, created_at).

**reviews schema**: `reviews` (id, order_id, reviewer_id, reviewee_id, rating smallint 1..5, comment, created_at, UNIQUE(order_id, reviewer_id, reviewee_id)).

Backhaul search: haversine distance in SQL over open transport legs within ~15 km of the drop-off point, sorted by distance. Plain `earth_distance`-style formula on lat/lng columns with a bounding-box pre-filter — no PostGIS needed at this scale.

## 5. Registration & verification flows (contract v1.1)

Registration is a three-step flow for **all** roles:

1. `POST /auth/otp/request` → SMS code → `POST /auth/otp/verify` (phone ownership).
2. `POST /auth/myid/verify` — **mandatory MyID KYC**: multipart upload of passport series/number, PINFL, birth date + selfie. The backend proxies to the MyID B2B/B2G API (myid.uz, OAuth2 client-credentials — the iOS app never talks to MyID directly) and returns `{ myIdVerificationToken, isMatched, confidence, verifiedFullName }`. The token is short-lived (~10 min TTL), stored in `auth.myid_verifications`, and bound to the OTP `verificationId` (phone ↔ person binding). Errors: `PASSPORT_NOT_FOUND`, `FACE_MISMATCH`, `MYID_SERVICE_UNAVAILABLE`.
3. `POST /auth/register` with `role` + `myIdVerificationToken`. The server validates and consumes the token (`MYID_TOKEN_EXPIRED_OR_INVALID` otherwise), prefers `verifiedFullName` from MyID over the user-typed name, and sets `verificationStatus = approved` immediately — no manual document moderation.

**MVP (hackathon)** — the endpoints are real, the MyID upstream is mocked:

- `MyIDClient` interface with `MockMyIDClient` (always matches, mirrors the iOS `MockMyIDVerificationService`) — the whole token flow (issue → TTL → consume on register) is implemented for real, so swapping in the production client changes nothing else. Getting MyID partner access (partner cabinet on myid.uz) should start now — it's the longest lead-time item.
- **OTP delivery**: interface `SMSSender` with `LogSender` (prints code to stdout — perfect for demo) and a slot for **Eskiz.uz / Play Mobile** (the two standard SMS gateways for +998 numbers) later. Rate-limit OTP requests per phone (e.g. 3/10min) and hash codes at rest even in MVP — it's cheap.

**The extra check for `driver` / `equipmentProvider` — simulated but ENFORCED in MVP:**

1. **Driver license check** behind `LicenseVerifier`, wired into registration for real: after MyID (simulated) confirms identity, the verifier "queries the registry" by PINFL/license number and the business rules actually run:
   - Map requested vehicle/equipment to a required category set, e.g. `boxTruck/flatbed/refrigerated/tanker/dumpTruck ≥ 3.5t → C`, `tractorTrailer → CE`, self-propelled equipment per its class.
   - License valid + category present → `approved`, proceed to truck/equipment selection.
   - Category missing or license invalid → registration **rejected** (`verificationStatus = rejected` + error code `LICENSE_CATEGORY_MISMATCH`).
   - MVP upstream is `SimulatedLicenseVerifier` with deterministic outcomes (see §10) so both the approval AND the rejection path can be demoed on stage. The production registry client (government e-gov / IIB; fallback OCR vendors like ID Analyzer) replaces only the lookup, never the rules.
2. Keep both integrations behind their interfaces (`MyIDClient`, `LicenseVerifier`) so adding other countries later = new implementation + a `country` discriminator on the request; nothing in handlers changes.

## 6. Order lifecycle & escrow (per leg)

- `POST /orders` creates the order with 1–3 legs derived from `type` + presence of `equipmentRequest`/`laborRequest`; all legs start `published`.
- `POST /orders/{id}/accept {leg, executorId}` — atomic claim of a single leg (`LEG_ALREADY_TAKEN` on conflict). On success, orders-service calls wallet-service internally to create the escrow transaction for that `(orderId, payeeId)` and notifications-service to notify the client.
- `PATCH /orders/{id}/status {leg, status}` — only the assigned executor of that leg; enforce forward-only transitions from the shared `OrderStatus` enum.
- `POST /orders/{id}/confirm-completion` — allowed when every leg is `delivered|completed`; marks all legs `completed` and calls wallet release **once per payee**. Release must be idempotent (already-released → 200 with current state) so retries are safe.
- Cancellation only while no leg has passed `accepted`.
- Price: MVP keeps the client-side formula, but expose `POST /orders/estimate` early (echoing the same formula server-side: weight × tariff + equipment hours × rate + workers × hours × rate, min 100 000 UZS) so tariffs can move server-side without an app release.

## 7. Realtime

Notifications-service runs a WebSocket hub: `GET /ws?token=<jwt>` → one connection per user, events `order.updated`, `order.created`, `transaction.updated`, `notification.created` with the same JSON payloads as REST plus an `event` field. Other services emit via `POST /internal/events {userIds, event, data}`; the hub fans out and also persists `notification.created` rows. iOS falls back to 5–10s polling if the socket drops — the REST endpoints already support that.

## 8. Milestones (4-day hackathon shape)

| Day | Deliverable | Definition of done |
|---|---|---|
| 0 (½ day) | Repo, compose, migrations, gateway, `pkg/*` | `make up` starts postgres + 6 services; `/health` green on all; JWT issue/verify round-trips |
| 1 | auth-service complete (incl. MyID mock flow) + orders create/list/get | iOS passes OTP → MyID verify → register → login, creates an order visible in "Мои заказы" |
| 2 | orders accept/status/cancel/confirm + wallet escrow | full happy path: client creates → driver accepts → status walk → confirm → escrow released; combo order with 2 legs pays 2 payees independently |
| 3 | notifications + WS hub, reviews + rating aggregate, backhaul search, license-check enforcement in registration | push event on every order change; review after completion updates `User.rating`; backhaul returns sorted nearby orders; driver with wrong license category gets rejected on stage |
| buffer | polish + "wholeness" pass | error codes match contract exactly; seed script with demo users/orders; every simulated integration demoed both succeeding and failing; run iOS against real backend |

Team split (if 2 backend devs): dev A takes auth + wallet, dev B takes orders + notifications + reviews; the contract and `pkg/models` are the interface between them.

## 9. Testing & quality bar (hackathon-appropriate)

- Table-driven unit tests only where logic can silently corrupt money or state: leg claim atomicity, status transition rules, escrow create/release idempotency, commission math.
- One `httptest`-based smoke test per service hitting the real router.
- A `scripts/demo.sh` curl walkthrough of the full happy path — doubles as living documentation and pre-demo sanity check.
- Error responses always `{ "error": { "code", "message" } }` with the exact codes from the contract (`OTP_INVALID`, `OTP_EXPIRED`, `PHONE_ALREADY_REGISTERED`, `PASSPORT_NOT_FOUND`, `FACE_MISMATCH`, `MYID_SERVICE_UNAVAILABLE`, `MYID_TOKEN_EXPIRED_OR_INVALID`, `LEG_ALREADY_TAKEN`, `ORDER_NOT_PUBLISHED`, `ORDER_NOT_FOUND`, `USER_NOT_FOUND`).

## 10. Simulated integrations — the system must feel WHOLE

Hackathon rule: **nothing in the demo may look stubbed**. Every external
integration is implemented as a high-fidelity simulation behind the same
interface the real integration will use later. The flow, timing, data shapes,
and failure modes are real; only the upstream is fake. Each simulation must
also be able to *fail on demand* — a demo that can show the rejection path is
far more convincing than one that only ever succeeds.

| Integration | Interface (seam) | MVP simulation — how it looks real | Deterministic failure trigger (for demo) |
|---|---|---|---|
| MyID KYC | `MyIDClient` (`internal/auth/verification.go`) | ~2s artificial latency (matches iOS mock), returns `confidence 0.93–0.99`, `verifiedFullName` derived from passport data; full token TTL/consume flow is real | passport number `0000000` → `PASSPORT_NOT_FOUND`; PINFL starting `99` → `FACE_MISMATCH` |
| Driver license registry | `LicenseVerifier` (same file) | returns a category set by PINFL and validates it against the requested vehicle (trucks → C, tractorTrailer → CE); approved drivers proceed to truck selection | PINFL ending in odd digit → license has only `["B"]` → `LICENSE_CATEGORY_MISMATCH`, registration rejected |
| SMS OTP | `SMSSender` (`internal/auth/otp.go`) | code logged server-side AND (demo trick) accept master code `7777` in non-prod so on-stage registration never stalls | request >3 codes in 10 min → rate-limit error |
| Payments Payme/Click/Uzcard | `PaymentProvider` (wallet) | simulated charge with ~1.5s latency and a fake provider reference (`payme_txn_...`) stored on the transaction; hold/release/refund ledger is fully real | amount `999999999` → `PAYMENT_DECLINED` |
| Price estimate | `POST /orders/estimate` | the exact client formula server-side (weight × tariff + equipment hrs × rate + workers × hrs × rate, min 100 000 UZS), tariffs in a config table so they can be "updated live" during Q&A | — |
| Geocoding | client-side `PseudoGeocoder` stays | already deterministic and demo-safe; a `/geo` proxy (Yandex/2GIS) is a post-hackathon swap | — |
| Push FCM/APNs | WS hub covers realtime | WebSocket events land instantly — indistinguishable from push in a live demo | — |

Also in-scope to make the product feel complete: a **seed script** (`scripts/seed.sh`) that creates believable demo users (client, 2 drivers, equipment provider, labor team) and a handful of orders in different statuses across Tashkent/Samarkand, so every screen of the iOS app has real-looking data the moment the demo starts.

Deferred for real (invisible in a demo, fine to skip): fleetAdmin/admin consoles, truck catalog ("техпаспорт" checks) beyond the license-category gate, real provider settlements.

## 11. Admin dashboard (web frontend)

A back-office dashboard for operating and demoing the platform: everything
visible at once — live orders on a map, orders list, registered users, and
money statistics (credited to executors + platform service fees).

**Stack**: Vite + React + TypeScript, **Radix UI** (`@radix-ui/themes` for the
design system + Radix primitives), **Leaflet/OpenStreetMap** for the map (free,
no API key, works fine for Tashkent/Samarkand; swappable for Yandex/2GIS
later). Lives in the same monorepo under `dashboard/`.

**Design rules (strict)**:

- **Apple-style glass design (glassmorphism), achieved through Radix — not
  hand-rolled.** Radix Themes' built-in `panelBackground="translucent"` gives
  every `Card`/`Table`/panel a frosted, blurred glass surface; the only global
  CSS allowed is the backdrop it needs to read against (one soft gradient
  background) and glass-consistent tuning via Radix theme tokens. Layered
  depth: gradient backdrop → translucent glass panels → content.
- **No custom component designs.** The UI is built entirely from Radix UI
  Themes components (`Card`, `Table`, `Badge`, `Tabs`, `Grid`, `Flex`,
  `Heading`, `Text`, …) with their stock variants, colors, spacing, and
  typography. No hand-rolled components, no per-component style overrides
  beyond Radix theme tokens. If Radix doesn't offer it, the dashboard doesn't
  have it.
- **No emojis anywhere** — labels, badges, and statuses are plain text; icons,
  if ever needed, come only from `@radix-ui/react-icons`.
- The only non-Radix visual element is the Leaflet map canvas itself (there is
  no map in Radix); it sits inside a Radix glass `Card` like every other panel.

**Layout** (single screen, everything at once):

- **Stats row** (cards): total orders · active orders · completed orders ·
  registered users · **credited to executors** (Σ released payouts = amount −
  commission) · **service fees charged** (Σ platformCommission of released) ·
  held in escrow.
- **Map panel**: pickup/dropoff markers for live orders, color-coded by
  status; clicking a marker highlights the order in the list.
- **Orders table**: id, client, type, legs + per-leg status badges, price,
  scheduled date; filter by status/type.
- **Users table**: name, phone, role, verificationStatus badge, rating,
  registration date; filter by role.

**Data**: the dashboard consumes three thin admin endpoints added to the
existing services (all guarded by an `admin`-role JWT, proxied by the
gateway): `GET /admin/users` (auth), `GET /admin/orders` (orders),
`GET /admin/stats` (wallet — aggregates credited/fees/held in SQL). Realtime:
reuse the notifications WS channel or poll every 5–10s (fine for a
back-office screen). The skeleton ships with a **mock-data mode**
(`VITE_USE_MOCKS=true`, on by default) so the dashboard renders fully without
the backend — same swap-the-client pattern as the iOS app.

**Milestone fit**: skeleton is ready now (renders with mocks); wiring to real
endpoints is a half-day on day 3, after the admin endpoints land. In the demo
it doubles as the "mission control" screen while the phone drives the flow.

## 12. Risks & mitigations

- *Leg race conditions* (two drivers accept simultaneously) → single-statement conditional UPDATE, covered by a test on day 2.
- *Money drift* → `numeric` end-to-end, strings over the wire, commission computed server-side only.
- *Contract drift vs iOS* → `pkg/models` reviewed against `Core/Model/*.swift` at day 0 and frozen; any change goes through the contract doc first.
- *WS flakiness during demo* → polling fallback already in the iOS app; keep it.
