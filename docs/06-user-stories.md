# User Stories

Version 1.0 · 2026-09-30 · Owner: Tech lead

Format: the 3 C's (Card, Conversation, Confirmation) and INVEST. Design: screens on the design canvas (link in `docs/README.md`), cited as screen numbers from `docs/06` of the design plan (0 role picker, 1 room map, 2 check-in, 3 occupied room, 3b extras, 4 check-out, 5 QR, 6 paid, 7 desktop map, 8 housekeeping, 9 owner overview, 10 staff permissions, 11 close shift, 12 shift review).

Legend: `Lane` API, WEB or PLAT · points use the scale in docs/07 §2 (1 point ≈ one hour of tech-lead review and integration) · `Slice` groups the API and WEB stories that share a contract · `Depends` are story ids · `Traces` are FR and NFR ids (docs/02 §2).

Roles: Owner, Receptionist, Housekeeping, Prospect (a person trying the demo), Tech lead.

Every AC becomes a test whose name ends in `<story>_AC<n>`.

## E0 — Platform and foundations (Sprint 0)

### SG-001 Repository scaffold, tooling and CI
Lane PLAT · 3 pts · Slice none · Depends: none · Traces: NFR-08, NFR-09, NFR-10
As the tech lead, I want a repository skeleton with tooling and CI, so that every lane starts from the same green baseline.
1. `make up`, `make down`, `make test`, `make lint`, `make fmt`, `make contracts` and `make e2e` exist; unimplemented ones print a clear "not yet, story SG-xxx" message and exit non-zero.
2. `api/` builds a Go module with a health handler; `GET /healthz` returns 200 and a request log line with the fields in docs/02 §7.6.
3. `web/` builds a Next.js static export placeholder; `make up` serves it through the API container.
4. CI on pull requests runs lint, unit tests, gitleaks (full history) and a licence check; a deliberately planted fake secret makes CI fail (verified once, then removed).
5. Dependency versions are pinned; Dependabot or Renovate is configured; Apache-2.0 `LICENSE` and `NOTICE` exist (ADR-002).
6. Read-deny rules for generated, vendored and build directories are configured for Claude Code (verify the syntax against current Claude Code documentation) and `.gitignore` covers them.

### SG-002 Contract pipeline
Lane PLAT · 3 pts · Slice none · Depends: SG-001 · Traces: NFR-01, NFR-07
As a lane owner, I want contracts linted, diffed and turned into code, so that API and web build in parallel without drift.
1. `make contracts` runs Spectral lint, oasdiff against `main`, JSON Schema compilation and `generate_vectors.py --check`; any failure fails CI.
2. Go strict server stubs are generated from `openapi.yaml`; an unimplemented operation is a compile error in `api/`.
3. A typed TypeScript client and MSW handlers are generated from the same spec; `web/` can run with `NEXT_PUBLIC_MOCK=1` without an API.
4. A breaking change to a published operation without a new major version is rejected by oasdiff in CI (demonstrated by a test branch).
5. Generated files carry a header saying they are generated and are excluded from review size limits.
6. A spike records which OpenAPI 3.1 features the generators handle (oapi-codegen added only initial 3.1 support recently); if a feature used in `openapi.yaml` is unsupported, the contract is downgraded to 3.0.3 in a contract PR and ADR-003 is updated.

### SG-003 Database foundation
Lane API · 3 pts · Slice none · Depends: SG-001 · Traces: FR-14, NFR-02, NFR-07, NFR-10
As the tech lead, I want migrations, tenancy and idempotency in place, so that every later story inherits safe persistence.
1. goose migrations create the tables of docs/05 needed by Sprint 0 and 1, each with an RLS policy on `tenant_id`.
2. A test fails when any table in schema `app` lacks an RLS policy.
3. The application role has no BYPASSRLS and cannot run DDL; a separate maintenance role exists for trial cleanup.
4. A unit of work sets the tenant for the transaction; a query without a tenant set returns zero rows, not an error and not another tenant's rows.
5. The idempotency store replays an identical request and returns 409 `IDEMPOTENCY_KEY_REUSED` for the same key with a different body.
6. `audit_logs` rejects UPDATE and DELETE via trigger, proven by a test; `GET /readyz` reports 503 until migrations are applied.

### SG-004 Web foundation
Lane WEB · 2 pts · Slice none · Depends: SG-001 · Traces: FR-13, NFR-05, NFR-06, NFR-11
As a developer, I want the web shell, design tokens and internationalisation, so that screens are consistent and translated from day one.
1. Routes are locale-prefixed (`/vi/...`, `/en/...`) with a client-side redirect from `/` using stored or browser language; the spike outcome is recorded in ADR-011.
2. A CI check fails when `vi` and `en` message files differ in keys.
3. Design tokens (colour, type, spacing, radius, motion) live in one file; every room status has a colour token and a text label.
4. `Intl` helpers format VND as `40.000đ` in `vi` and `₫40,000` in `en`, covered by unit tests.
5. An axe check runs against the shell in both languages with zero serious violations; touch targets ≥ 44 px.
6. The generated client is wired through one module; a lint rule forbids raw `fetch` calls to `/v1`.

## E1 — Core domain (Sprint 0)

### SG-101 Pricing engine
Lane API · 5 pts · Slice none · Depends: SG-001 · Traces: FR-03, FR-04, NFR-01
As the owner, I want every stay priced by rules I can configure, so that staff cannot argue or miscalculate.
1. All 25 cases and 2 error cases in `contracts/pricing/golden-cases.json` pass, including cap flags and line codes.
2. The rate plan is validated against `rate-plan.schema.json`; an invalid plan is rejected with field-level errors.
3. Property tests with a fixed seed (logged on failure) show that price never decreases as check-out moves later, never exceeds the cap where a cap applies, and is always an integer.
4. The calculation is a pure function: the package imports no I/O packages (import-boundary test) and receives instants as arguments.
5. Bill assembly (stay quote plus extras, deposit, balance due, refund due) matches `billCases` including the refund case.
6. Changing a tenant's rate plan does not change the price of a stay that already holds a snapshot (test with two versions).

### SG-102 Sessions, identity and tenant context
Lane API · 3 pts · Slice none · Depends: SG-002, SG-003 · Traces: FR-01, FR-13, NFR-03, NFR-10
As a prospect, I want to start as a chosen role without signing up, so that I can try the product in seconds.
1. `POST /v1/demo/sessions` returns a session for the chosen role and locale on the seeded demo tenant when `DEMO_MODE` is on, and 404 `DEMO_DISABLED` when it is off.
2. Tokens are 256-bit random, stored only as SHA-256 hashes, expire after `SESSION_TTL_HOURS`, and an expired token returns 401 `SESSION_EXPIRED`.
3. `GET /v1/me` returns user, tenant and building access; `PUT /v1/me/locale` persists the language.
4. Middleware resolves session to tenant and user, sets the tenant for the transaction, and rejects unknown tokens with 401 `UNAUTHENTICATED`.
5. A role or building check helper (`Authorizer`) exists with table-driven tests covering the matrix in docs/04 §2.3 for every role and level.

## E2 — Room map and front-desk stays (Sprint 0 to 1)

### SG-201 Rooms and buildings read API
Lane API · 2 pts · Slice S1 · Depends: SG-003, SG-102 · Traces: FR-02, NFR-03, NFR-04
As a receptionist, I want to see rooms by building with live status, so that I know what can be rented.
1. `GET /v1/buildings` returns only buildings with access VIEW or EDIT, with my level and status counters.
2. `GET /v1/buildings/{buildingId}/rooms` returns every room with status, unit type name in both languages and an active stay summary including elapsed minutes and running total.
3. OVERDUE is derived: an overnight or daily stay past its expected end shows OVERDUE, an hourly stay never does (test with a fixed clock).
4. A user with access NONE receives 403 `BUILDING_FORBIDDEN`; another tenant's room id returns 404.
5. For 200 rooms, p95 server time is ≤ 150 ms on the reference laptop with a warm database (benchmark recorded in the PR).

### SG-202 Room map screen
Lane WEB · 3 pts · Slice S1 · Depends: SG-002, SG-004 · Traces: FR-02, NFR-05, NFR-06
As a receptionist, I want a clear room map on my phone and on the desk computer, so that I can act in one tap.
1. Screens 1 and 7 render from the API (or mocks) with building tabs, status counters and room tiles that show number, status text and note without truncating "Maintenance" in English.
2. A building with view-only access shows faded tiles, a lock label and a banner, and tiles are not actionable.
3. Tapping a vacant room opens check-in, an occupied room opens its details; the elapsed time updates every minute without refetching all rooms.
4. Initial JS for the route is ≤ 200 KB gzip and Lighthouse performance is ≥ 90 on the reference profile.
5. Vitest and Playwright cover both languages; axe reports zero serious violations.

### SG-203 Check-in API
Lane API · 3 pts · Slice S2 · Depends: SG-101, SG-201 · Traces: FR-03, FR-04, NFR-01, NFR-07
As a receptionist, I want to check a guest in, so that the stay clock and price start on the server.
1. `POST /v1/rooms/{roomId}/stays` creates an ACTIVE stay, sets the room OCCUPIED, records `check_in_at` from the server clock and snapshots the rate plan.
2. Two concurrent requests for the same vacant room produce one 201 and one 409 `ROOM_NOT_VACANT` (concurrency test).
3. The same Idempotency-Key and body returns the original response; a different body returns 409 `IDEMPOTENCY_KEY_REUSED`.
4. An ID number, when supplied, is stored encrypted and returned masked; logs contain no ID number (log capture test).
5. The deposit is stored on the stay and returned on every stay response; recording it as drawer cash is added by SG-503 AC6, not here.
6. `GET /v1/stays/{stayId}` returns a live quote equal to the pricing engine's result for the same instants.

### SG-204 Check-in screen
Lane WEB · 2 pts · Slice S2 · Depends: SG-002, SG-004 · Traces: FR-03, NFR-06
As a receptionist, I want a short check-in form, so that a guest is in within a minute.
1. Screen 2 shows the three rental types with prices from the unit type, a selected state that does not rely on colour alone, and inputs with visible labels.
2. Validation errors (missing name, invalid phone) appear next to fields in the active language.
3. Submitting sends one request with a generated Idempotency-Key, and a network retry does not create a second stay (mock and integration test).
4. A `ROOM_NOT_VACANT` response shows a translated message and returns to the room map.
5. The screen states that check-in time is recorded by the system.

### SG-205 Extras and check-out API
Lane API · 3 pts · Slice S3 · Depends: SG-203 · Traces: FR-05, FR-06, NFR-01, NFR-07
As a receptionist, I want to add extras and check out, so that the bill is complete and final.
1. `GET /v1/services` lists extras with stock; adding more than the stock returns 409 `INSUFFICIENT_STOCK`.
2. `POST /v1/stays/{stayId}/extras` adds items, decrements stock atomically and updates the quote; repeating with the same key changes nothing.
3. `POST /v1/stays/{stayId}/checkout` records `check_out_at` from the server, freezes the quote into an invoice with a bill code, and returns the same invoice on repeat.
4. Checked-out stays reject extras with 409 `STAY_NOT_ACTIVE`.
5. The invoice equals the matching `billCases` when built from the same inputs; deposit larger than the total yields `refundDue`.
6. The room stays OCCUPIED until the invoice is paid (docs/02 §6.1).

### SG-206 Stay details, extras sheet and check-out screens
Lane WEB · 3 pts · Slice S3 · Depends: SG-002, SG-004 · Traces: FR-04, FR-05, FR-06, NFR-06
As a receptionist, I want to see the running total, add extras and check out, so that the guest can pay right away.
1. Screen 3 shows time in room, running total, extras and the balance after deposit, and refreshes the total each minute from the server value.
2. Screen 3b is a bottom sheet over the details, with plus and minus controls, stock, and a total on the confirm button; stock limits disable the plus control.
3. Screen 4 shows every bill line with quantities and the grace-period explanation, and offers Cash or Bank transfer.
4. English text in buttons and lines never wraps onto a second line inside a button and never overflows a tile (visual regression in both languages).
5. Vitest covers total rendering and error mapping; Playwright covers the path from details to check-out on mocks.

## E3 — Payments (Sprint 1 to 2)

### SG-301 Payment creation and QR payload
Lane API · 3 pts · Slice S4 · Depends: SG-205 · Traces: FR-07, NFR-07
As a receptionist, I want to start a cash or transfer payment, so that the guest pays the owner directly.
1. `POST /v1/invoices/{invoiceId}/payments` with TRANSFER returns a PENDING payment with a bill code and a VietQR (EMVCo) payload for the exact balance, built from the tenant's account only; no request field can change the account.
2. The payload's CRC and structure are validated by unit tests against known-good samples; the manual scan check in A-04 is recorded in the PR (two bank apps).
3. Starting a new transfer expires the previous PENDING one; an invoice already PAID returns 409 `INVOICE_ALREADY_PAID`.
4. CASH marks the payment PAID at once, records PAYMENT_IN (and REFUND_OUT when a deposit is returned), and moves the room to TO_CLEAN with a housekeeping task in the same transaction.
5. The QR payload and account number never appear in logs (log capture test); the response masks the account number.
6. Expiry after `PAYMENT_EXPIRY_MINUTES` is applied on read and on the next event, with a fixed-clock test.

### SG-302 Payment event handler, simulator and SSE
Lane API · 3 pts · Slice S4 · Depends: SG-301 · Traces: FR-08, NFR-07
As an owner, I want a transfer counted only when the money arrives, so that staff cannot mark it paid.
1. One handler processes events from the simulator and, later, a provider: it stores the event, deduplicates on `(provider, external_id)`, matches bill code and amount, and settles in one transaction (PAID, room TO_CLEAN, housekeeping task).
2. A duplicate event is acknowledged and changes nothing; a wrong amount sets `MISMATCH`, keeps the invoice OPEN and creates an owner alert.
3. `POST /v1/demo/payments/{paymentId}/simulate` exists only with `DEMO_MODE`, feeds the same handler, and is absent (404) otherwise; no other code path sets a transfer to PAID (architecture test).
4. `GET /v1/payments/{paymentId}/events` sends the current state first, a heartbeat every 15 s, honours `Last-Event-ID`, and closes after a terminal state; a slow client does not block others (goroutine leak test).
5. From simulate to `PAYMENT_PAID` reaches a connected client within 1 s at p95 on the reference laptop.

### SG-303 QR and paid screens
Lane WEB · 3 pts · Slice S4 · Depends: SG-002, SG-004 · Traces: FR-07, FR-08, NFR-05, NFR-06
As a receptionist, I want the QR and a clear waiting state, so that I know when the guest has paid.
1. Screen 5 renders the QR from the payload in the browser (library lazy-loaded), shows amount, masked account, account name and bill code, and never shows the full account number.
2. Status uses SSE with reconnect and falls back to polling every 3 s after two failed connections; the screen changes to Paid without user action.
3. The waiting state is announced through a live region and does not rely on colour alone.
4. "Simulate payment received" appears only when the API reports demo mode; the button text stays on one line in both languages.
5. Screen 6 shows amount, time received, transaction id and that the room is now To clean; Print receipt opens the browser print dialog with a receipt stylesheet.

## E4 — Housekeeping and owner (Sprint 2)

### SG-401 Housekeeping API
Lane API · 2 pts · Slice S5 · Depends: SG-302 · Traces: FR-09, NFR-03
As housekeeping, I want to see rooms to clean and mark them done, so that rooms return to service.
1. `GET /v1/housekeeping/tasks` lists OPEN tasks only for buildings where the caller has at least VIEW.
2. Completing a task sets the room VACANT and stamps who and when; completing a done task returns the same result (idempotent).
3. Completing requires EDIT on the room's building and the HOUSEKEEPING or OWNER role; otherwise 403.
4. `POST /v1/rooms/{roomId}/usage-reports` creates an `UNUSED_ROOM_REPORT` alert for the owner with the reporter and note.
5. Every completion and report writes an audit row.

### SG-402 Housekeeping screen
Lane WEB · 2 pts · Slice S5 · Depends: SG-002, SG-004 · Traces: FR-09, NFR-06
As housekeeping, I want a simple list with one big button per room, so that I can use it one-handed.
1. Screen 8 lists rooms to clean with checkout time and a Mark clean button of at least 44 px; done rooms update in place.
2. The remaining count in the heading updates immediately and is announced to assistive technology.
3. "Report a used room" asks for an optional note and confirms in the active language.
4. View-only users see the list without action buttons.

### SG-403 Owner overview API
Lane API · 2 pts · Slice S6 · Depends: SG-302 · Traces: FR-10, NFR-03, NFR-04
As an owner, I want today's numbers and anything unusual in one call, so that I can check from my phone.
1. `GET /v1/owner/overview` returns revenue, transfers received, cash expected, revenue per building, occupancy, alerts and the latest payments for the tenant-local date.
2. Revenue equals the sum of paid invoices for that day; a test with seeded data proves each figure independently of the implementation.
3. Non-owners receive 403 `ROLE_FORBIDDEN`.
4. Sample alerts from the fixture appear in trial tenants; real alerts from SG-302, SG-401 and SG-503 appear in time order.
5. The query plan for 10,000 payments uses the indexes in docs/05 §6 (EXPLAIN in the PR).

### SG-404 Owner overview screen
Lane WEB · 2 pts · Slice S6 · Depends: SG-002, SG-004 · Traces: FR-10, NFR-06
As an owner, I want a glanceable overview with alerts I can open, so that I can spot losses early.
1. Screen 9 shows revenue, transfers received, cash expected, per-building revenue, occupancy, alerts and latest payments in both languages.
2. Each alert kind has a translated message built from structured values, with no text coming from the API.
3. Opening a cash alert goes to the shift review; buttons to Staff permissions and the room map are on one line.
4. Empty states (no alerts, no payments) have text, not blank areas.

## E5 — Access control and shifts (Sprint 2 to 3)

### SG-501 Building permissions API
Lane API · 3 pts · Slice S7 · Depends: SG-102, SG-201 · Traces: FR-11, FR-14, NFR-03
As an owner, I want to decide per person and per building who can only view and who can edit, so that staff act only where they work.
1. `PUT /v1/owner/staff/{userId}/building-permissions/{buildingId}` sets NONE, VIEW or EDIT and writes an audit row (who, whom, building, old level, new level).
2. `GET /v1/owner/staff-permissions` returns every staff member with their level per building.
3. Every endpoint in docs/04 §2.3 enforces role and building access; a generated table test proves each operation for every role and level, including 403 `BUILDING_FORBIDDEN` and 403 `ROLE_FORBIDDEN`.
4. Revoking access takes effect on the next request without re-login (test changes the level between two calls with one token).
5. A new building starts with no access for anyone except the owner; the owner cannot lock themselves out.
6. Trial tenant seed matches the fixture: receptionist EDIT on A and VIEW on B.

### SG-502 Staff permissions screen
Lane WEB · 2 pts · Slice S7 · Depends: SG-002, SG-004 · Traces: FR-11, NFR-06
As an owner, I want to change access with three buttons per building, so that setup takes seconds.
1. Screen 10 shows one card per staff member with a None, View, Edit control per building; the selected state does not rely on colour alone.
2. Each change saves at once with visible confirmation; a failed save reverts the control and shows a translated message.
3. Non-owners cannot reach the screen; the room map for a changed user reflects the new level on their next load.
4. Buttons stay on one line in English and Vietnamese.

### SG-503 Shift close and cash reconciliation API
Lane API · 3 pts · Slice S8 · Depends: SG-301, SG-501 · Traces: FR-12, FR-14, NFR-07
As an owner, I want each shift closed against counted cash, so that shortages are visible with a reason and a name.
1. `GET /v1/shifts/current` opens a shift lazily with opening float equal to the previous shift's float left, and returns expected cash from cash entries.
2. `POST /v1/shifts/current/payouts` records cash paid out with a description; `POST /v1/shifts/current/close` accepts counts by denomination, computes the counted total and difference, and locks the shift.
3. A non-zero difference without a reason returns 422 `CASH_REASON_REQUIRED`; with a reason it creates a `CASH_SHORT` or `CASH_OVER` alert and an audit row.
4. Closing twice returns 409 `SHIFT_ALREADY_CLOSED`; repeating the same request with the same key returns the same result.
5. `GET /v1/owner/shifts/{shiftId}` returns the review: cash payments, counted, difference, reason and this month's count of shifts with differences and total short for that staff member.
6. Check-in now records the deposit as a DEPOSIT_IN entry on the caller's open shift (extending SG-203 in the same transaction), and refunds appear as REFUND_OUT; expected cash matches the worked example in docs/08 §6.

### SG-504 Close-shift and shift-review screens
Lane WEB · 3 pts · Slice S8 · Depends: SG-002, SG-004 · Traces: FR-12, NFR-06
As a receptionist and as an owner, I want to close a shift with a cash count and review differences, so that gaps are explained.
1. Screen 11 shows expected cash breakdown, denomination counters with plus and minus, live counted total, and the difference in a state that shows text ("Short", "Over", "Balanced") as well as colour.
2. The reason field appears and is required only when the difference is not zero.
3. The End shift button on the room map opens the screen; after closing, figures are read-only.
4. Screen 12 shows result, expected vs counted, the staff reason, cash bills of the shift, and the staff member's monthly history.
5. Every button and label stays on one line in both languages; tests cover both.

## E6 — Trials and packaging (Sprint 2 to 3)

### SG-601 Trial tenants
Lane API · 3 pts · Slice S9 · Depends: SG-102, SG-003 · Traces: FR-01, FR-15, NFR-02
As a prospect, I want my own sample data, so that what I click does not affect anyone else.
1. A session request without `tenantId` clones `contracts/fixtures/demo-tenant-seed.json` into a new trial tenant expiring after `TRIAL_TTL_HOURS`; with `tenantId` it joins that tenant in another role.
2. Two trials running concurrently never see each other's data (isolation suite from SG-604 passes on both).
3. Expired trials are removed lazily on the next trial creation with a bounded batch, using the maintenance role.
4. Creation is rate-limited to `TRIAL_RATE_PER_IP_PER_HOUR` with 429 and `Retry-After`; more than `TRIAL_MAX_ACTIVE_TENANTS` active trials returns 429 with a distinct code.
5. The sample state matches the fixture (occupied, overdue, to-clean, maintenance rooms, sample alerts, permissions).

### SG-602 Role picker and language switch
Lane WEB · 2 pts · Slice S9 · Depends: SG-002, SG-004 · Traces: FR-01, FR-13, NFR-11
As a prospect, I want to pick a role and a language on the first screen, so that I start in two taps.
1. Screen 0 lists the three roles with one-line descriptions and a VI and EN switch, and starts a session that lands on the matching first screen.
2. Switching language keeps the route and state; the choice is stored and sent with `PUT /v1/me/locale` after sign-in.
3. Role switching from any screen reuses the same trial tenant.
4. There is no "demo" label anywhere in the UI; the footer text is limited to what the tech lead approves.

### SG-603 Deployment packaging
Lane PLAT · 3 pts · Slice none · Depends: SG-001 · Traces: NFR-09
As the tech lead, I want one small container and two ways to run it, so that I can self-test for free and show prospects a fast instance.
1. A multi-stage Dockerfile builds the web export, embeds it in the Go binary and produces an image ≤ 60 MB that runs as a non-root user.
2. Documented deploy to Cloud Run with Neon works from a clean checkout; the first-request latency after idle is measured and recorded in the PR (A-08).
3. A Docker Compose file with PostgreSQL and Caddy runs the same image on a VPS with HTTPS and automatic restart; migrations run on start.
4. Config is validated at start: missing `DATA_ENCRYPTION_KEY` or database URL stops the process with a clear message.
5. Runbook for backups and restore is in `docs/runbooks/` and a restore was rehearsed once.

### SG-604 End-to-end journeys and isolation suite
Lane PLAT · 2 pts · Slice none · Depends: SG-603, SG-504, SG-602 · Traces: NFR-02, NFR-03, NFR-06
As the tech lead, I want the critical journeys and the isolation checks automated, so that a public release is safe.
1. Playwright runs the five journeys in docs/08 §5 against `make up` with mocks off, in both languages.
2. The isolation suite calls every operation of the contract with two tenants and asserts 404 for cross-tenant ids.
3. The permission matrix table test runs against the running stack, not only in unit tests.
4. A release checklist run (docs/11 §6) is attached to the v0.1.0 pull request.
5. The two-minute video script and the walkthrough steps are stored in `docs/assets/`.

---

# Production v1.1 (from docs/15)

Format as above. Each story is a vertical slice: the API lane implements the operations, the WEB lane the screens in docs/15 §4 at all three breakpoints. AC numbers become test names.

## E7 — Sign-in and accounts

### SG-701 PIN sign-in, lockout and PIN change
Lane API+WEB · 5 pts · Depends: SG-102 · Screens: P1, P2, P3, PC Đăng nhập, P23
1. signIn returns a session for a valid guesthouse code, user name and PIN; any wrong part returns the same 401 PIN_INVALID.
2. The fifth wrong PIN within 15 minutes locks the account until `lockedUntil`, returns ACCOUNT_LOCKED and creates an ACCOUNT_LOCKED alert.
3. A session from a one-time PIN can only call changeMyPin until the PIN is changed (403 PIN_CHANGE_REQUIRED); runs and repeated digits are rejected.
4. PINs are stored with a slow hash; no response, log or audit entry contains a PIN.
5. Sign-in is rate-limited per IP and per guesthouse code; signOut revokes the session.

### SG-702 Responsive shell and account page
Lane WEB · 3 pts · Depends: SG-701 · Screens: DOC Quy tắc responsive, P23, owner sidebar, TAB samples
1. One layout component renders the phone top bar, tablet icon rail and desktop grouped sidebar (Monitor, Finance, Operations, People, Settings) from the same route.
2. Pages pass visual checks at 390, 834 and 1280 px in vi and en with no sideways page scroll.
3. Account page changes language (setMyLocale), opens My schedule, changes PIN and signs out.

### SG-703 Tenant import and SePay CLI
Lane API · 3 pts · Depends: SG-1001 · Doc: docs/runbooks/sepay-handover.md
1. `stayguard tenant import --file` creates tenant, buildings, rooms, rates, services, bank account and staff in one transaction and prints one-time PINs.
2. `stayguard sepay webhook|set-secret|status --tenant` work as the runbook describes; set-secret reads the secret from a hidden prompt only.
3. receiveBankWebhook resolves the tenant by hookId, verifies the HMAC over the raw body in constant time, settles through the existing handler, and answers duplicates with the same 2xx.
4. Every CLI change writes an INSTALLER audit entry visible to the owner.

## E8 — Front desk extras

### SG-801 Edit check-in time and move room
Lane API+WEB · 3 pts · Screens: P4, P5
1. editCheckInTime accepts up to 60 minutes later than recorded, never in the future, with a reason code and note; otherwise 422 CHECKIN_EDIT_OUT_OF_RANGE.
2. The quote is recomputed and a STAY_TIME_EDITED alert records old time, new time, actor and reason.
3. moveStay keeps check-in time and extras, prices the whole stay with the new room type and sets the old room TO_CLEAN; the target must be VACANT (409 ROOM_OCCUPIED).

### SG-802 Stay history and receipt
Lane API+WEB · 3 pts · Screens: P9, PC Lịch sử (lễ tân), P8
1. listStays takes a single date or a range; receptionists are limited to the last `frontDeskHistoryDays` days (422 outside it) in buildings they can view; owners and managers may use any range.
4. Every row shows guestId indicators (ID number on file, front photo, back photo) and nothing more; the screens offer previous and next day, a date picker and Today and Yesterday.
2. Search matches room code, guest name and phone; results page with a cursor.
3. getReceipt returns the frozen invoice with property name, address and payments; the print layout fits 80 mm.

### SG-803 Payment states on screen
Lane WEB · 2 pts · Screens: P6, P7
1. A MISMATCH payment shows due, received and remaining, offers a QR for the remainder or cash, and never offers to mark it paid.
2. An EXPIRED payment offers a new QR or cash and explains that old-code transfers are still recorded.

### SG-804 Cleaning and damage reports by any role
Lane API+WEB · 3 pts · Screens: P37, P47, P48, P49, PC Dọn phòng
1. completeHousekeepingTask allows OWNER, MANAGER, RECEPTIONIST and HOUSEKEEPING with EDIT; records who and when.
2. Tapping a TO_CLEAN room on any room map opens the clean screen; housekeeping's list sorts by longest waiting and colours by waiting time.
3. reportDamage creates a ticket and alert; LOCK_ROOM sets MAINTENANCE (409 ROOM_OCCUPIED if a guest is in it).

### SG-805 Guest ID capture and protected viewing
Lane API+WEB · 5 pts · Screens: Demo 2, PC Nhận phòng, Demo 3, Sơ đồ máy tính, P42, PC Chi tiết lượt ở, P59, PC Xem ảnh CCCD
1. Check-in and setGuestIdNumber store the number encrypted only with consent (422 ID_CONSENT_REQUIRED otherwise); uploadGuestIdPhoto accepts JPEG or PNG up to 5 MB, strips metadata, re-encodes and encrypts.
2. Every stay response to RECEPTIONIST or HOUSEKEEPING contains only `guestId` indicators; no field, log line or error ever contains the number or an image (log capture test and contract test).
3. getGuestIdRecord returns the masked number and photo metadata to OWNER and MANAGER only (403 for other roles, table test).
4. revealGuestIdNumber, getGuestIdPhoto (view and download) and both deletes write GUEST_ID audit entries; responses carry `Cache-Control: no-store`; photos are never served by a public or pre-signed URL.
5. A daily job deletes numbers and photos past `idRetentionDays` after check-out and records the deletion.
6. UI: front desk sees capture tiles and indicators only; owner sees the masked number with Show and Hide, thumbnails with View, Download and Delete, and a viewer with front and back tabs.

## E9 — Owner monitoring

### SG-901 Overview by building
Lane API+WEB · 3 pts · Screens: PC Tổng quan, TAB Tổng quan, Demo 9
1. getOwnerOverview returns per-building status counts, occupancy and revenue today, and an attention list (overdue rooms, long waits to clean, mismatches, unmatched transfers, cash shortages, pending leave, open tickets).
2. Each building row links to the owner room map for that building; each attention item links to the screen that resolves it.

### SG-902 Alerts and activity log
Lane API+WEB · 3 pts · Screens: P10, P13, P36, PC Cảnh báo, PC Nhật ký
1. listAlerts filters unread and kind; markAlertRead is per alert; money and stay-time alerts never clear themselves.
2. listAuditLogs takes a date range, actor, category and text search; the UI offers quick ranges and a calendar range picker.
3. The log cannot be edited or deleted by any role (test against the database grants).

### SG-903 Transactions and linking unmatched transfers
Lane API+WEB · 3 pts · Screens: P12, P43, PC Giao dịch, PC Gán tiền
1. listTransactions shows cash and bank items with MATCHED, MISMATCH, UNMATCHED or CASH.
2. linkTransferToInvoice is OWNER only, accepts only bank-reported events, settles the invoice, and is irreversible (409 EVENT_ALREADY_LINKED).
3. Candidate invoices are unpaid ones, those matching the amount listed first.

### SG-904 Owner stay history and timeline
Lane API+WEB · 2 pts · Screens: P42, PC Lịch sử lượt ở, PC Chi tiết lượt ở
1. getStayTimeline lists check-in, edits, extras, moves, check-out, payments, links and cleaning in time order from the audit log.

### SG-905 Closed shifts list
Lane API+WEB · 1 pt · Screens: P25, PC Đối soát ca
1. listClosedShifts filters by month, person and differences only; each item opens the shift review.

## E10 — Setup

### SG-1001 Property and receiving accounts
Lane API+WEB · 3 pts · Screens: P15, P38, PC Nhà nghỉ và ngân hàng, PC Thêm tài khoản
1. Property name, address, phone and QR expiry are editable by the owner.
2. createBankAccount stores the account encrypted as PENDING; makeDefault requires CONNECTED; the default cannot be removed; all three need the owner PIN.
3. QR always uses the default account (existing rule, now across many accounts).

### SG-1002 Buildings, floors and rooms
Lane API+WEB · 3 pts · Screens: P16, P26, P33, P34, P35, PC Tòa và phòng
1. createBuilding and createFloor can generate rooms; createRooms supports a single code or a range and reports the codes created.
2. updateRoom sets type, features, maintenance with reason and date, or retires; a room with a guest returns 409 ROOM_OCCUPIED for type change or retire.
3. New buildings grant access to nobody but the owner.

### SG-1003 Rates editing with preview
Lane API+WEB · 2 pts · Screens: P17, PC Bảng giá
1. updateRatePlan saves a new version; stays keep their snapshot (existing SG-101 AC6).
2. previewPrice prices sample stays with the draft plan using the same pricing engine.

### SG-1004 Items and stock
Lane API+WEB · 5 pts · Screens: P18, P40, P44, P45, P46, P28, PC Dịch vụ và kho and related
1. createService records price, unit cost and an OPENING movement.
2. updateService never changes stock; restock adds an IN movement with unit cost.
3. listStockMovements pages the history with filters; the management page shows stock, price, latest cost, margin and 7-day sales.
4. removeService deletes an item without sales, otherwise sets stop selling; the result says which.
5. createStocktake posts COUNT movements for differences and alerts the owner.

## E11 — People

### SG-1101 Staff with positions, app access and contracts
Lane API+WEB · 5 pts · Screens: P19, P20, P21, P39, PC Nhân viên and related
1. createStaff stores position, app access and contract; a one-time PIN is returned only when app access is not NONE.
2. MANAGER can do everything in docs/15 §2 and gets 403 on the listed exclusions (table test).
3. removeStaff needs the owner PIN, deactivates, signs out and keeps history; 409 SHIFT_OPEN if a shift is open.
4. The staff table shows position and status, has a sticky name column and scrolls sideways on narrow screens.

### SG-1102 Roster and leave decisions
Lane API+WEB · 5 pts · Screens: P50, PC Lịch ca và nghỉ
1. getRoster returns assignments, leave and uncovered shifts for a range; putRoster applies set and remove atomically.
2. copyRosterWeek copies the previous week without overwriting approved leave.
3. approveLeave and declineLeave (with reason) notify the requester; approving a cancel request restores the shift.
4. A person's scheduled shift is used when they open a shift at the desk.

### SG-1103 My schedule and leave
Lane API+WEB · 3 pts · Screens: P51, P52, P57, P58, PC Lịch và nghỉ (lễ tân)
1. Staff see their week and leave balance; createLeaveRequest rejects overlaps (409 LEAVE_OVERLAP).
2. cancelMyLeave cancels PENDING at once and turns APPROVED into CANCEL_REQUESTED.

### SG-1104 Payroll
Lane API+WEB · 5 pts · Screens: PC Bảng lương
1. getPayroll computes earned pay from contract and roster (monthly pro-rated by standard shifts, per shift, or hourly) plus fixed allowance; numbers are whole VND with a stated rounding rule.
2. Bonus and deduction are owner inputs; the product never deducts cash shortages automatically.
3. markPayrollPaid freezes lines and posts STAFF_PAY expense lines; paid lines cannot be edited.
4. The table scrolls sideways with a sticky name column and a totals row.

## E12 — Finance and maintenance

### SG-1201 Maintenance tickets
Lane API+WEB · 3 pts · Screens: P53, P54, PC Bảo trì, PC Phiếu bảo trì
1. Tickets show code, room, issue, reporter, status, expected date and cost; totals show open tickets, locked rooms, month cost and tickets without cost.
2. updateTicket to DONE posts a MAINTENANCE expense for the completion month and unlocks the room; only OWNER edits costs (Q-04).

### SG-1202 Expenses
Lane API+WEB · 3 pts · Screens: P55, P56, PC Chi phí, PC Thêm chi phí
1. getExpenseMonth returns categories with source and items; automatic lines come from payroll, tickets and stock; recurring templates are added on the first day of each month.
2. Manual lines can be created, edited and deleted; automatic lines return 409 EXPENSE_AUTOMATIC.
3. Drawer payouts stay in shift reconciliation and never appear as expenses (test with a payout in the month).

### SG-1203 Income and cost report
Lane API+WEB · 3 pts · Screens: P11, PC Báo cáo thu chi
1. getIncomeCostReport accepts from and to months (max 24) and returns revenue, expenses, profit, margin, occupancy and the breakdowns in docs/15 §3 rule 18.
2. Figures reconcile: revenue equals paid invoices in range, expenses equal the expense month totals (integration test with seeded data).

## E13 — Quality

### SG-1301 Common states
Lane WEB · 2 pts · Screens: P29 to P32
1. Offline, server error (with trace id), no access and empty list states are shared components used by every list and form.
2. Retrying a write after offline reuses its Idempotency-Key.
