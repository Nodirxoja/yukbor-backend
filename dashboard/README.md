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

- **Live mode (default)**: calls the gateway via the `/api` dev proxy
  (`vite.config.ts` → `http://localhost:8080`) and shows the sign-in screen.
  Sign in with an `admin`-role account.
- **Mock mode**: `VITE_USE_MOCKS=true npm run dev` — renders the whole
  dashboard from fixtures with no backend, skipping sign-in.

Mocks are opt-IN deliberately. With the reverse default, a production build
that simply lacked the flag shipped fabricated orders and bypassed login
entirely — `scripts/build-dashboard.sh` now sets the flag explicitly AND fails
the build if a mock record reaches `dist/`.

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
