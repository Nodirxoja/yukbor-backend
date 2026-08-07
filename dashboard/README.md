# YUK BOR — Admin Dashboard

Back-office dashboard: live orders on a map, orders list, registered users,
and money statistics (credited to executors + platform service fees), all on
one screen. Plan: `../docs/DEVELOPMENT_PLAN.md` §11.

Stack: Vite + React + TypeScript · Radix UI (`@radix-ui/themes`) · Leaflet/OSM.

Design (plan §11): **Apple-style glass** via Radix's built-in
`panelBackground="translucent"` — frosted, backdrop-blurred panels over one
global gradient backdrop (`src/styles.css`, the only global CSS). All
components are stock Radix UI Themes; no custom component designs, no emojis
(icons only from `@radix-ui/react-icons`).

```bash
npm install
npm run dev        # http://localhost:5173 — renders with mock data by default
```

- **Mock mode (default)**: `VITE_USE_MOCKS` unset/`true` — the whole dashboard
  renders without a backend (same pattern as the iOS mock services).
- **Live mode**: `VITE_USE_MOCKS=false npm run dev` — calls the gateway via the
  `/api` dev proxy (`vite.config.ts` → `http://localhost:8080`). Requires the
  admin endpoints (`GET /admin/stats|orders|users`) and an admin JWT in
  `VITE_ADMIN_TOKEN`.

Structure:

```
src/
├── api/types.ts        wire types — mirror pkg/models 1:1
├── api/client.ts       fetch wrapper with mock-mode switch
├── mocks/data.ts       demo users/orders/transactions + stats computation
├── components/
│   ├── StatsCards.tsx  orders / users / credited / fees / escrow
│   ├── OrdersTable.tsx per-leg status badges, row ↔ map selection
│   ├── UsersTable.tsx  role + verificationStatus badges
│   └── OrdersMap.tsx   Leaflet map, pickup markers + route lines
└── App.tsx             layout + 10s polling
```
