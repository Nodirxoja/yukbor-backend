# Backend implementation notes

Companion to `API_CONTRACT.md`. The contract is **frozen** — the iOS app needs
no changes to talk to this backend. This file records the places where the
implementation *adds* to the contract (never changes it), the decisions taken
where the contract was silent, and the demo triggers.

Swift's `Decodable` ignores unknown keys, so every additive response field
below is invisible to an app that does not ask for it.

---

## 1. Additive request fields (all optional)

| Endpoint | Field | Why | Default when absent |
|---|---|---|---|
| `POST /orders` | `paymentMethod` | The contract's `Order` has no payment method, but escrow needs one when a leg is accepted. | `payme` |
| `POST /auth/login` | `verificationId` | The contract posts only `phoneNumber`, which would let anyone mint tokens for any registered number. When sent, it is validated against a confirmed OTP. | Allowed outside prod; **required** when `APP_ENV=prod` |

## 2. Additive response fields

| Endpoint | Field | Why |
|---|---|---|
| `POST /auth/otp/request` | `devCode` | The OTP itself, so the seed and demo scripts never wait for an SMS. **Omitted entirely when `APP_ENV=prod`.** |
| `POST /orders/estimate` | `breakdown` | Per-leg amounts (`{transport, equipment, labor}`), so a combo order can show what each executor is paid. |
| `GET /wallet/transactions` | `providerRef`, `refundedAt` | The PSP charge reference and refund time. |

## 3. Endpoints added beyond the contract

- `POST /orders/estimate` — proposed in contract §8, now implemented.
- `GET /admin/users`, `GET /admin/orders`, `GET /admin/stats` — the dashboard's
  three endpoints (plan §11), each guarded by an `admin`-role JWT.
- `GET /reviews?userId=` — the review list behind the aggregate.
- `POST /internal/*` — service-to-service only, guarded by `X-Internal-Token`.
  **The gateway does not route `/internal/*` at all**, so these are unreachable
  from outside the compose network.

## 4. Decisions where the contract was silent

**`Order.status` for single-leg orders.** `status` mirrors the transport leg
when there is one, otherwise the order's only leg. For `equipmentOnly`,
`status` and `equipmentStatus` therefore hold the same value — iOS can read
either. A leg the order does not have stays `null`.

**WebSocket authentication.** `GET /ws?token=<jwt>` (an `Authorization` header
also works). The plan sketched `?userId=`; that would let anyone subscribe to
anyone's stream, so it was not implemented.

**Who gets a notification.** Every participant receives the realtime *state*
event so their screen stays correct, but the stored *notification* goes only to
the counterparty — telling a driver "Driver accepted your order" reads as a bug.

**Money split across legs.** One `priceEstimate`, but 2–3 payees. The server
computes the contract's formula per component and splits the total in
proportion, guaranteeing the parts sum to the total exactly. When the client
sends a `priceEstimate`, that total is honoured (so the order never shows a
different number than the user agreed to) and only the *split* uses the server
breakdown.

**Cancellation.** Allowed while no leg has moved past `accepted`; held escrow
is refunded. Once work has started, `ORDER_NOT_CANCELLABLE`.

**Status transitions.** Forward-only, but forward *skips* are allowed — a short
labor job legitimately goes `accepted → delivered`. Backward transitions are
always rejected; that is the invariant that protects money.

**Re-registering a rejected applicant.** A user whose licence check failed is
stored with `verificationStatus: rejected` and a reason (so the dashboard can
show them), and their phone number can be registered again. An `approved`
account still returns `PHONE_ALREADY_REGISTERED`.

## 5. Demo triggers

Every simulation fails on demand, so the rejection path can be shown on stage.

| Trigger | Result |
|---|---|
| passport number `0000000` | `PASSPORT_NOT_FOUND` |
| PINFL starting `99` | `FACE_MISMATCH` |
| OTP code `7777` | Always accepted (non-prod only) |
| 4th OTP request in 10 min | `OTP_RATE_LIMITED` |
| `priceEstimate` `999999999` | `PAYMENT_DECLINED`, and the leg claim is rolled back |

**PINFL digits are load-bearing.** The first digit is the real century/sex
digit, so the simulated MyID returns a correctly gendered Uzbek name. The last
digit selects the licence the registry issues:

| Last digit | Licence | Outcome |
|---|---|---|
| 1, 3, 5, 7, 9 | `["B"]` | Rejected at registration — `LICENSE_CATEGORY_MISMATCH` |
| 0, 2, 4 | `["B","C"]` | Registers fine, but **refused a tractor-trailer load** on accept |
| 6, 8 | `["B","C","CE"]` | No restrictions |

The middle bucket is what makes the second gate real: without a driver holding
C but not CE, the accept-time check could never fire.

## 6. Generated identity data

The contract carries no licence or plate, so the registry issues both,
deterministically from the PINFL (stable across demo runs):

- **Licence number** — `AB1234567` (two letters, seven digits)
- **Vehicle plate** — both Uzbek forms, `01 123 ABC` and `01 A 123 BC`, with
  real region codes (01 Tashkent city, 10 Tashkent region, 60 Samarkand, …)

## 7. Security notes

- OTP codes are SHA-256 hashed at rest; 4 digits, 120 s TTL, 5 attempts,
  3 requests per phone per 10 minutes.
- MyID tokens are consumed by a single conditional `UPDATE`, so one KYC pass
  registers exactly once, and the token is bound to the OTP `verificationId` —
  it cannot be used with a different phone number.
- Refresh tokens are hashed at rest and **rotated** on every use.
- Every read of another user's data (`/orders?clientId=`,
  `/wallet/transactions?payeeId=`, `/notifications?userId=`) is restricted to
  the caller or an admin.

## 8. Known gaps

- The WS hub is in-memory, so it is single-replica. Every event is also
  persisted as a notification row and the REST endpoints support polling, so a
  dropped socket loses nothing — a broker is only needed to scale out.
- `fleetAdmin` has no dedicated behaviour; it registers and authenticates like
  any other role.
- Geocoding stays client-side (`PseudoGeocoder`), per contract §8.
