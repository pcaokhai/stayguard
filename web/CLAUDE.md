# web — CLAUDE.md

Next.js static export (App Router) for staff, owner and housekeeping in Vietnamese and English, on phone and desktop. Lane: **WEB**. Owns `web/**`. Read the root `CLAUDE.md` first.

## Commands

`make test-web` (Vitest), `make lint`, `make fmt`, `make gen` (client and MSW handlers from the contract), `make e2e` (Playwright, after `make up`). Run with mocks: set `NEXT_PUBLIC_MOCK=1`. While iterating, run a single test file and pipe to `tail -n 20`. Created by SG-001, SG-002 and SG-004.

## Layout

```
web/
  src/app/[locale]/...    routes; locale prefix /vi or /en; entity ids in query strings (ADR-011)
  src/features/           rooms, stays, payments, housekeeping, owner, permissions, shifts, session
  src/components/ui/      shared presentational components
  src/lib/                api (generated client wrapper), format (Intl helpers), i18n, tokens
  src/api/generated/      typed schema from the contract (openapi-typescript), committed, Read-denied
  src/mocks/generated/    MSW handlers from the contract (orval), committed, Read-denied
  src/mocks/setup/        hand-written worker and test wiring; `NEXT_PUBLIC_MOCK=1` build bundles it via the `mock-layer` alias in next.config.ts
  messages/               vi.json and en.json (same keys, checked in CI)
  e2e/                    Playwright journeys J1 to J5
```

## Next.js rules

- Static export only: no server actions, route handlers, middleware, cookies or headers at request time; dynamic segments must be known at build time, so ids go in the query string and are read in small client components inside a `Suspense` boundary.
- Locale routes are static (`generateStaticParams` for `vi` and `en`); `/` redirects on the client by stored or browser language.
- Server state lives in TanStack Query; no `useEffect` for fetching; local UI state stays local; derived values are computed.
- Only the generated client talks to `/v1`; a lint rule forbids raw `fetch` to it. Errors map from `code` to translated messages.
- Payment status: SSE with reconnect, falling back to polling every 3 s after two failures; announce changes in a live region.
- QR: render in the browser from the payload with a lazy-loaded library; never show the full account number.

## UI rules

- All strings come from message files; no sentence built by concatenation; money and dates through the `Intl` helpers.
- Design tokens only for colour, spacing, radius, type and motion; every status has a text label as well as colour; touch targets ≥ 44 px; buttons keep their label on one line in both languages.
- Motion is short and disabled under `prefers-reduced-motion`.
- Sensitive data: never render or store full ID numbers or account numbers; session token only in memory or session storage as decided in SG-004.
- Initial JS on the room map route ≤ 200 KB gzip; lazy-load heavy libraries.

## TypeScript rules

Follow docs/10 §4.3: `strict`, `noUncheckedIndexedAccess`, no `any`, Zod at boundaries, named exports, one component per file, stable keys.

## Tests

Vitest and Testing Library for components and hooks; MSW for contract consumption; Playwright for journeys and screenshots in both languages; axe on every screen. Test names end with the story and criterion. Coverage and gates: docs/08 §3.
