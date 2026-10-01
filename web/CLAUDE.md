# web — CLAUDE.md (FAST MODE)

Next.js App Router, `output: 'export'`, served by the Go binary. Read the root CLAUDE.md first. Current tasks: docs/14 §4 (W-tasks).

## Stack (add only these)

Tailwind CSS v4, `@tanstack/react-query`, a QR renderer (`qrcode`), plus what is already in package.json (`openapi-fetch`, msw, orval). No component library unless a task names one.

## Where the design is

- `docs/assets/design/screens/*.png`: what each screen must look like (Vietnamese files have no `EN` suffix).
- `docs/assets/design/source/*.dc.html`: the original markup with inline styles. Read one screen at a time with grep or a line range. Reproduce the layout, colours, spacing and text in Tailwind; it is not React, so do not paste it.
- Colours and radii: put them once in the Tailwind theme (`@theme`) from the source, then reuse.

## Routes (static export: no dynamic segments; ids go in the query string)

`/vi` role picker · `/vi/rooms` room map (`?b=<buildingId>`) · `/vi/checkin?room=` · `/vi/stay?id=` (details and extras sheet) · `/vi/checkout?stay=` · `/vi/pay?payment=` · `/vi/paid?payment=` · `/vi/housekeeping` · `/vi/owner`. Only `vi` is generated now; `en` is added later by adding the locale and a message file. Components that read `useSearchParams` sit inside `Suspense`.

## Rules

- Data only through the generated client in `src/lib/api.ts` and TanStack Query hooks per feature (`src/features/<name>/`). No raw `fetch` to `/v1`.
- Session: the role picker calls `createDemoSession`; keep the token in `sessionStorage`; a client middleware adds the `Authorization` header; 401 sends the user back to `/vi`.
- Writes send an `Idempotency-Key` (one UUID per user action, reused on retry); disable the button while pending.
- Payment status: `useQuery` with `refetchInterval: 3000` until the status is not PENDING.
- Show prices and totals exactly as the API returns them; format with `Intl.NumberFormat('vi-VN')` plus `đ`. Never compute prices in the browser.
- All text through `t('key')` from `messages/vi.json`; no hard-coded Vietnamese in components.
- Mobile first (390 px), and the room map also works at 1280 px. Buttons keep their label on one line; touch targets at least 44 px; every status shows text, not only colour.
- Mock mode (`NEXT_PUBLIC_MOCK=1`) must keep working so UI tasks never wait for the API.

## Done for a screen

`npm run build` and `npm run lint` pass; the screen matches its PNG at 390 px when you compare side by side; the happy path works on mocks and, once the API task is merged, against `make up`.
