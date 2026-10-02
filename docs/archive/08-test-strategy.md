# Test Strategy and Scenarios

Version 1.0 · 2026-09-30 · Owner: Tech lead

## 1. Principles

- Test first: every story starts with failing tests (Superpowers `test-driven-development`).
- Acceptance criteria are the spec: test names end with `<story>_AC<n>`, for example `TestCheckIn_RoomNotVacant_SG203_AC2`.
- Domain invariants are asserted, not just examples: money is integer, price is monotonic, cap respected, one settlement per event, no cross-tenant read.
- Determinism: the clock, ids and random seed are injected; no `sleep` for synchronisation; failures print the seed.
- Fakes over mocks: in-memory repositories and a fake payment source; mocks only where an interaction is the behaviour.

## 2. Pyramid

| Level | Scope | Tools | Lane | When |
| --- | --- | --- | --- | --- |
| Unit | Domain and use cases with fakes | Go `testing`, go-cmp | API | Every commit |
| Property | Pricing invariants; cash arithmetic | Go property-style tests with a seeded generator | API | Every commit |
| Golden | `contracts/pricing/golden-cases.json` in Go; generator `--check` | Go, Python | API, PLAT | Every commit |
| Integration | Repositories, RLS, migrations, idempotency, settlement | Testcontainers PostgreSQL | API | Every commit (tagged, parallel) |
| Contract (provider) | Real responses validated against the OpenAPI spec | Spec validator in Go tests | API | Every commit |
| Contract (consumer) | UI runs on MSW mocks generated from the same spec | Vitest, MSW | WEB | Every commit |
| Component | Components, hooks, i18n, formatters | Vitest, Testing Library | WEB | Every commit |
| Visual | Screens in both languages for text overflow and button wrapping | Playwright screenshots | WEB | PR |
| End-to-end | Five journeys on the real stack | Playwright | PLAT | PR and nightly |
| Accessibility | axe on every screen and the keyboard path of the journeys | Playwright with axe-core | WEB | PR |
| Chaos and resilience | Faults during settlement and streaming | Testcontainers with Toxiproxy | API | Nightly |
| Performance | Room map latency, SSE latency, bundle size | Go benchmark, k6 or equivalent, size limit | API, WEB | PR for budgets, nightly for load |
| Security | Secrets, dependencies, static analysis, authorization matrix | gitleaks, govulncheck, gosec, npm audit, table test | PLAT | Every commit |

## 3. Quality gates

| Gate | Threshold |
| --- | --- |
| Go coverage | ≥ 85% overall, ≥ 95% in `domain/` |
| Mutation testing on `domain/pricing` | Survivors reviewed; target score ≥ 80% (if the chosen tool does not work on the toolchain, record a manual mutation review in the PR) |
| Web coverage | ≥ 80% lines in features and hooks |
| Contract checks | Spectral clean; oasdiff no breaking change; every operationId has a provider test |
| Pricing vectors | 25 of 25 cases and generator `--check` |
| Static analysis and lint | Zero findings at error level |
| Secrets | gitleaks: zero findings in full history |
| Dependencies | No high or critical advisory (govulncheck, npm audit) at release |
| Accessibility | axe: zero serious or critical violations; touch targets ≥ 44 px |
| Web performance | Initial JS ≤ 200 KB gzip on the room map route; Lighthouse performance ≥ 90 |
| API performance | Room map p95 ≤ 150 ms (200 rooms); simulate-to-event p95 ≤ 1 s |
| End-to-end | Five journeys green in `vi` and `en` |
| Isolation | Isolation suite green: every operation × two tenants |

## 4. Test data rules

Fixtures come from `contracts/fixtures/` and `contracts/pricing/`; they contain no real people, accounts or ID numbers. Builders create stays, invoices and shifts with sensible defaults and an injected clock fixed at 2026-10-05T10:00:00+07:00 unless a test says otherwise. Tests never depend on another test's data; each integration test creates its own tenant.

## 5. Critical end-to-end journeys

| # | Journey | Covers |
| --- | --- | --- |
| J1 | Prospect starts as Front desk, checks in a vacant room hourly, adds extras, checks out, pays by QR, simulates payment, sees Paid and the room To clean | FR-01 to FR-08 |
| J2 | Housekeeping marks the room clean; the room is vacant on the front-desk map | FR-09 |
| J3 | Owner opens the overview and sees the revenue, transfer and sample alerts; opens Staff permissions and changes a level | FR-10, FR-11 |
| J4 | Receptionist with view-only access on Building B cannot act; owner grants edit; the receptionist can act on the next load | FR-02, FR-11 |
| J5 | Receptionist adds a payout, closes the shift with a gap and a reason; owner opens the alert and the review | FR-12 |

Every journey runs in Vietnamese and English.

## 6. Scenarios

Each scenario: Objective, Starting conditions, Role, Steps (action → observable result), Expected outcomes. Stories in brackets.

**TS-01 Cash check-out with refund** (SG-203, SG-205, SG-301)
Objective: a bill with a deposit larger than the total returns money correctly. Starting: vacant Standard room, open shift, tenant clock fixed. Role: Receptionist with EDIT. Steps: check in hourly with deposit 100,000 → 201, room OCCUPIED; advance clock 45 minutes; check out → invoice total 80,000, refund due 20,000; pay by cash → payment PAID. Expected: room TO_CLEAN with an open housekeeping task; cash entries DEPOSIT_IN 100,000 and REFUND_OUT 20,000.

**TS-02 QR payment happy path** (SG-301, SG-302, SG-303)
Objective: a transfer is paid only by the event. Starting: invoice OPEN with balance 40,000. Role: Receptionist. Steps: create TRANSFER payment → PENDING with a QR payload and bill code; open the event stream → current state PENDING; simulate payment → event PAYMENT_PAID within 1 s. Expected: invoice PAID, room TO_CLEAN, housekeeping task created, screen shows Paid without action.

**TS-03 Duplicate event** (SG-302)
Objective: retries do not double count. Starting: PENDING payment. Steps: send the same event (same external id) five times concurrently. Expected: one settlement, four acknowledged duplicates, one cash-neutral result, one payment_events row.

**TS-04 Wrong amount** (SG-302)
Objective: a mismatched transfer never settles. Steps: send an event with the right code and amount 30,000 instead of 40,000. Expected: payment MISMATCH, invoice OPEN, owner alert created, staff cannot mark it paid by any endpoint.

**TS-05 Concurrent check-in** (SG-203)
Objective: one guest per room. Steps: two receptionists post check-in for the same vacant room at the same time. Expected: exactly one 201 and one 409 `ROOM_NOT_VACANT`; one ACTIVE stay.

**TS-06 View-only building** (SG-201, SG-501, SG-202)
Role: Receptionist with EDIT on A and VIEW on B. Steps: open Building B → tiles faded with lock label and banner; call `createStay` for a B room → 403 `BUILDING_FORBIDDEN`. Expected: no state change; the audit log has no entry.

**TS-07 Immediate revocation** (SG-501)
Steps: with one token, call `createStay` on Building A → 201; owner sets the receptionist's A access to NONE; call again with the same token → 403. Expected: building A disappears from `listBuildings` for that user.

**TS-08 Tenant isolation** (SG-003, SG-604)
Steps: create two trial tenants; take a room, stay, invoice and payment id from tenant 1; call every operation that accepts those ids with tenant 2's token. Expected: 404 for all, and no row of tenant 1 is visible in any list.

**TS-09 Shift close, balanced (worked example)** (SG-503)
Starting: opening float 500,000. Three check-ins with deposits of 100,000 each (DEPOSIT_IN 300,000). Bills paid in cash: 140,000 with deposit 100,000 (PAYMENT_IN 40,000); 80,000 with deposit 100,000 (REFUND_OUT 20,000); 320,000 with deposit 100,000 (PAYMENT_IN 220,000). Payout for ice 30,000. Expected cash: 500,000 + 300,000 + 40,000 + 220,000 − 20,000 − 30,000 = 1,010,000. Steps: count 2 × 500,000 and 1 × 10,000 → counted 1,010,000. Expected: difference 0, no reason required, no alert, shift locked.

**TS-10 Shift close with a gap** (SG-503, SG-504)
Steps: same shift; count 1 × 500,000, 4 × 100,000, 1 × 50,000, 1 × 10,000 = 960,000; close without reason → 422 `CASH_REASON_REQUIRED`; close with reason → 200. Expected: difference −50,000, `CASH_SHORT` alert, audit row, owner review shows the reason, six cash lines and this month's history; a second close returns 409.

**TS-11 Language switch keeps place** (SG-004, SG-602)
Steps: open check-in for a room in `vi`; switch to `en`. Expected: same route and same entered values; all strings English; money formatted `₫`; no button text wraps.

**TS-12 Stream failure falls back to polling** (SG-303)
Steps: start a transfer; block the event stream twice; simulate payment. Expected: the screen shows Paid within 4 s via polling.

**TS-13 Idempotent retry** (SG-203, SG-205)
Steps: post check-in, drop the response, repeat the same request with the same key. Expected: one stay, identical response; a different body with the same key returns 409 `IDEMPOTENCY_KEY_REUSED`.

**TS-14 Trial expiry and cleanup** (SG-601)
Steps: create a trial; advance the clock past `TRIAL_TTL_HOURS`; create another trial. Expected: the expired tenant and its data are gone, the new tenant is intact, the session of the expired tenant returns 401.

**TS-15 Keyboard and screen reader on payment** (SG-303, SG-004)
Steps: complete J1 with keyboard only and a screen reader. Expected: focus order follows the visual order; the waiting and paid states are announced; axe reports no serious violations.

## 7. Chaos and resilience suite

| Scenario | Injection | Invariant |
| --- | --- | --- |
| Database restart during settlement | Restart PostgreSQL while an event is processed | Never PAID without a stored event; no duplicate cash entries; retry settles once |
| Latency on the database | Toxiproxy latency 500 ms | Requests time out per `DB_STATEMENT_TIMEOUT`; no goroutine leak; readiness turns red |
| Event flood | 100 concurrent duplicate events | One settlement; p95 handler time within budget |
| Slow SSE client | Client reads one byte per second | Other clients unaffected; the stream is closed after the write deadline |
| API restart with open streams | Kill and restart the container | Clients reconnect with `Last-Event-ID` or poll; no lost settlement |
| Trial creation storm | 100 requests from one IP | Limited per IP; global cap enforced; existing trials unaffected |
