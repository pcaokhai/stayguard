# 14 — Demo Today, Production v1 Within 24 Hours (FAST MODE)

Version 1.0 · 2026-10-01 · Owner: Khai
Supersedes docs/07 (delivery plan) and docs/11 (workflow) while FAST MODE is on. Inputs: docs/12 (Gemini review), docs/13 (Codex review), code at commit eee1789.

## 1. Where the code really is

Done and tested on `main`: scaffold and CI (SG-001), contract pipeline (SG-002), database with RLS and idempotency (SG-003), pricing engine (SG-101), sessions (SG-102), room map read API (SG-201), check-in (SG-203), extras and check-out (SG-205). About 7,400 lines of Go, 94 test files.

Not started: everything in `web/` beyond the shell; payments, payment confirmation, housekeeping, owner overview (17 operations still return 501 from `server_stubs.go`).

**Blocking gap found in review:** `createDemoSession` creates an empty trial tenant. With no buildings, rooms, unit types or services, the room map is empty, so no screen can be demonstrated until seeding exists (task A1).

## 2. Decisions (FAST MODE)

| Topic | Decision |
| --- | --- |
| Backend architecture | Keep as is. No rewrite, no flattening of existing code. New read endpoints may be thin |
| Money, pricing, server time, payment-event-only settlement, tenant scope, idempotency on money writes | Keep strict (CLAUDE.md §4) |
| National ID | Not collected in the UI |
| Real-time | Polling every 3 s; SSE not built |
| Transfer confirmation | Real bank-reconciliation provider webhook in production (P3 option a); simulator in demo; both feed one settlement handler |
| Housekeeping | Rooms in TO_CLEAN are the list; mark clean sets VACANT; no task table |
| Owner overview | Revenue today, transfers, cash, occupancy, latest payments; alerts empty for now |
| Language | Vietnamese only for the demo and v1; strings in a message file so English is cheap later |
| Trials | Each demo session gets a tenant seeded from `contracts/fixtures/demo-tenant-seed.json`; no expiry clean-up job yet |
| Permissions per building, shift close and reconciliation, role picker polish, English, SSE, E2E suite | After v1 (§6) |
| Process | Push straight to `main` (no PRs); CI runs on every push. No brainstorming, worktrees, written plans, ADRs, release notes or bug log. Progress = the checkboxes in this file (tick in the task's commit); `git log` is the history |
| Feature flags | Existing flags stay (all on in demo and production). New endpoints get no new flag; only DEMO_MODE gates demo-only routes |
| Review | Khai reads every diff that touches money, payments, sign-in or tenant scope. UI diffs: skim, then judge on screen |
| Docs | Claude reads CLAUDE.md, its service CLAUDE.md and this file only. docs/01–13 are reference, opened only when a task names a section |
| CI | Pull requests: lint, unit tests, web build, secret scan. Main only: integration tests, contracts, licences |

## 3. How to run the sessions

Two Claude Code sessions in parallel, one terminal each, both committing and pushing to `main` (they touch different directories; pull before each task):

- Session API: A1 → A2 → A3 → A4
- Session WEB: W1 → W2 → W3 → W4 → W5 (on mocks until the matching API task is merged)
- You: read the diffs that touch money or tenant scope, tick boxes, then run I1.

Start each task in a fresh context (`/clear`). Paste the task text as the prompt.

## 4. Demo — today

### [x] A1 Seed the demo tenant (API) — blocking
Files: new seed package under `api/internal/app` (or `adapter/postgres`), the session creation path, the seed JSON.
What: when `createDemoSession` creates a trial tenant, in the same transaction insert from `contracts/fixtures/demo-tenant-seed.json` (embed the file): one property, buildings A and B with floors and rooms, unit types with rate plans (validate with the existing rate-plan validation), services with stock, and the sample occupancy (active stays created with the server clock minus fixed offsets, rooms in TO_CLEAN and MAINTENANCE). Use the existing repositories.
Tests: one integration test: a new demo session sees 35 rooms in 2 buildings, the expected status counts, 5 services; two sessions do not see each other's rooms.
Verify: `make test-api` and the integration test by name; `make up` with the demo override, create a session with curl, list rooms.

### [x] A2 Payments: create, read, simulate, settle (API) — SG-301 and SG-302 without SSE
What: `createPayment` (CASH settles immediately; TRANSFER returns PENDING with bill code and VietQR payload for the exact balance, tenant account only), `getPayment`, `simulatePaymentReceived` (DEMO_MODE only) feeding one settlement handler that stores the payment event, deduplicates on (provider, external id), matches bill code and amount, then in one transaction sets payment PAID, invoice PAID, room TO_CLEAN. Wrong amount: MISMATCH, invoice stays OPEN. Shape the handler so P3 only adds an adapter: input is a normalised event (provider, external id, amount, transfer content, received at, tenant), and the bill code is found inside the transfer content case-insensitively with spaces and dashes removed (banks often alter the note). No new flag; the simulator route exists only with DEMO_MODE.
Tests: settle happy path; duplicate event changes nothing; wrong amount gives MISMATCH; no other code path sets a transfer to PAID; QR payload CRC check against a known sample.
Verify: tests by name; curl through check-in → check-out → create transfer → simulate → get payment shows PAID.

### [x] A3 Housekeeping (API)
What: `listHousekeepingTasks` returns rooms in TO_CLEAN (task id = room id); `completeHousekeepingTask` sets VACANT, idempotent; roles OWNER or HOUSEKEEPING. `reportRoomUsage` stays 501. No new flag.
Tests: one use-case test for complete (role check, idempotent).

### [x] A4 Owner overview (API)
What: `getOwnerOverview` for the tenant-local day: paid revenue, transfers received, cash received, revenue per building, occupancy, latest 10 payments; alerts empty. OWNER only. No new flag.
Tests: one integration test with seeded payments checks each total.

### [x] W1 Web foundation (WEB)
What: add Tailwind v4, TanStack Query, `qrcode`; theme tokens from the design source; `t()` helper with `messages/vi.json`; API client wrapper with token and Idempotency-Key helpers; `/vi` layout; mock mode still works. Role picker at `/vi` (three roles, creates a demo session, routes to rooms, owner or housekeeping).
Verify: `npm run build`, `npm run lint`; role picker matches `ChonVaiTro.png`.

### [x] W2 Room map (WEB)
What: `/vi/rooms` with building tabs, status counters, room tiles (vertical layout as in the design), End shift button hidden for now; vacant room → check-in, occupied → stay. Desktop width uses the grid of `SoDoMayTinh.png` without the side panel.
Verify: matches `Main.png` at 390 px; works on mocks and on `make up` after A1.

### [x] W3 Check-in, stay details with extras sheet, check-out (WEB)
What: `/vi/checkin`, `/vi/stay` (running total from the API, refetch every 60 s; extras bottom sheet), `/vi/checkout` (bill lines, cash or transfer). Idempotency-Key per action.
Verify: matches `NhanPhong.png`, `ChiTiet.png`, `ThemDichVu.png`, `TraPhong.png`; flow works against `make up`.

### [x] W4 QR and paid (WEB)
What: `/vi/pay` renders the QR from the payload, polls `getPayment` every 3 s, shows "Giả lập tiền về" only in demo mode; `/vi/paid` shows amount, time, transaction id, room now to clean; print uses the browser dialog.
Verify: matches `ThanhToanQR.png`, `DaThanhToan.png`; transfer flow ends on Paid without a manual refresh.

### [x] W5 Housekeeping and owner (WEB)
What: `/vi/housekeeping` list with one Mark clean button per room; `/vi/owner` cards and latest payments (alerts section hidden when empty; Staff permissions button hidden).
Verify: matches `BuongPhong.png`, `TongQuanChu.png` minus hidden parts.

### [x] I1 Integrate and publish the demo (you)
1. `docker compose -f deploy/compose.yaml -f deploy/compose.demo.override.yaml up --build`, then `make migrate`.
2. Walk the script below on a real phone and on a laptop. Fix only what breaks the script.
3. Deploy the same image with the same environment to the demo host (VPS with Caddy, or Cloud Run with a managed PostgreSQL). Do not put real data in it.

Demo script (about 3 minutes): Lễ tân → sơ đồ phòng → nhận phòng A102 theo giờ → mở phòng A101 đang có khách, thêm 2 nước → trả phòng, xem dòng tính tiền → chuyển khoản, QR hiện đúng số tiền và tài khoản chủ → giả lập tiền về → tự chuyển "Đã thanh toán", phòng sang Cần dọn → đổi vai Buồng phòng, dọn A101 → đổi vai Chủ, xem doanh thu vừa tăng.

## 5. Production v1 — within 24 hours after the demo

Order: P4 → P2 → P1 → P3 → P5 → P6 (the webhook needs the public HTTPS server).

"Production v1" here means: one real guesthouse, its own staff on their own phones, real money flowing, on a server you control. It does not mean the full PRD.

### [ ] P1 Real sign-in (API + WEB) — required
Staff sign in with guesthouse code, user name and a PIN (hashed with a slow password hash, rate-limited per account). Reuse the existing session tokens. Owner creates staff accounts through a seed file for now (no admin screen). DEMO_MODE off. Tests: wrong PIN, lockout, token expiry, a user of tenant A cannot sign in to tenant B.

### [ ] P2 Tenant setup from a file (API) — required
A `stayguard tenant import <file>` subcommand creates a tenant with its buildings, rooms, prices, services, bank account and staff from a JSON file in the format of the demo seed. You fill it with the customer's real data (kept out of git).

### [ ] P3 Bank transfer confirmation through a provider webhook (API) — required, option (a)

Before coding (Khai, outside the repo, start today because it may take time):
1. Choose a Vietnamese bank-reconciliation provider that sends a webhook for each incoming transfer. Check its current pricing, supported banks, and how it authenticates webhooks.
2. The provider account and the linked bank account must belong to the guesthouse owner, because the money goes to the owner's account.
3. Get the webhook secret or API key, and the provider's test or sandbox way to send a sample transaction.

Task for Claude Code:
- Read the provider's current webhook documentation (the human pastes the link). Do not guess its payload or auth.
- Contract: change `receiveBankWebhook` to `POST /v1/webhooks/bank/{hookId}`, where `hookId` is a random per-tenant id, so the tenant is known before any check. Run `make gen`.
- Store per tenant: `hookId` and the provider secret (encrypted with the existing encryptor). Add them to the P2 tenant import file.
- Handler: look up the tenant by `hookId` → verify the provider's authentication exactly as documented (HMAC signature over the raw body, or secret token header) with a constant-time compare → reject on failure with 401 and no detail → map the payload to the A2 normalised event → call the A2 settlement handler → answer the status code the provider expects (and the same answer for duplicates so it stops retrying).
- Only incoming credits to the tenant's account are processed; outgoing or unrelated transfers are stored as UNMATCHED and ignored.
- Log provider event id and result; never log the raw body, account numbers or the secret.
Tests: valid signature settles; bad signature 401 and nothing stored; replayed event settles once; unknown hookId 404; amount mismatch gives MISMATCH; unmatched content stored as UNMATCHED.
Provider chosen: SePay. Use its Test mode (sandbox): create a test bank account and a test webhook with HMAC-SHA256, then simulate transactions from the dashboard. Read the current docs for exactly what is signed and the header name before coding.
Test in three layers, no real money needed for the first two:
1. Unit tests: a saved sample payload from the docs, signed in the test with a test secret.
2. SePay Test mode → local stack through a tunnel (cloudflared or ngrok): simulate an incoming transfer whose content contains the bill code shown on the QR screen; the screen turns Paid by itself. Also simulate a wrong amount and an outgoing transfer.
3. Go-live check on the server after P4, in Live mode: one 2,000đ transfer to the owner account with the bill code.

### [ ] P4 Server hardening — required
VPS with Caddy and HTTPS; the API connects as the non-superuser application role (RLS on; remove ALLOW_PRIVILEGED_DB); `DATA_ENCRYPTION_KEY` and database password from a file outside git; daily `pg_dump` to off-server storage and one restore rehearsal; DEMO_MODE off; finished flags on; container restart policy.

### [ ] P5 Roles — required, simple
Role checks as already implemented: receptionists can act in every building; owner sees overview; housekeeping sees only housekeeping. Per-building permissions come after v1. Confirm with the customer that this is acceptable for the first week.

### [ ] P6 Check before handing over
One automated smoke test of the demo script against the deployed stack; the full integration suite (`make test-api-int`) green; manual walkthrough on the receptionist's real phone; QR scanned with two banking apps against the real account; backup restored once.

## 6. After v1 (in this order)

Shift close and cash reconciliation (SG-503, SG-504) · per-building permissions (SG-501, SG-502) · stay time edits with owner alerts · English · trial expiry clean-up and rate limits (SG-601) · SSE · the rest of docs/08. Return to the docs/11 workflow for these.

## 7. Honest risks

- The plan assumes both Claude Code sessions work without long stalls. A1 and A2 are the critical path; if A2 slips, the demo can show cash payment only.
- Reproducing the designs in Tailwind is the largest unknown on the web side; accept small visual differences today.
- P3 depends on the provider's account approval and bank linking. Start the sign-up today; if it is not active within 24 hours, the code is ready and go-live waits only for activation.
- Real guest data must not enter the system before P1 and P4 are done.
