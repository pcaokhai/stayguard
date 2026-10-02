# 14 — Production Sprint (SHIP MODE, 36–48 hours)

Version 2.0 · 2026-10-02 · Owner: Khai
This file is the plan, the task list and the progress tracker. It replaces the demo plan (archived as `archive/14-demo-plan.md`). Product rules: docs/15. UI kit and motion: docs/16. API: `contracts/openapi.yaml` 1.1.0.

## 1. Where we start

Done on `main` (commit f64c380): demo flow end to end (room map, check-in, extras, check-out, QR and simulated payment, housekeeping, owner overview), pricing engine with golden vectors, sessions, RLS, idempotency, encryptor, audit writer. 28 operations, 8 migrations.

Not done: everything that makes it a real product for paying guesthouses: real sign-in, staff, real bank confirmation, settings, shifts, owner monitoring, and the production screens on the design canvas (121 Vietnamese boards in `docs/assets/design/screens/`).

## 2. How a solo startup ships this in 36–48 hours

**Goal of hour 24: the first guesthouse runs on it with real money. Goal of hour 48: the rest of the designed product is live behind it.** Anything not green by hour 44 moves to next week; it never blocks what is already live.

What we keep (quality that protects money and trust):

1. The strict rules in CLAUDE.md §4: money as whole VND, prices only from `domain/pricing`, server time, transfers paid only by bank events, tenant scope through RLS, idempotent money writes, guest ID data owner-only.
2. Contract first: `contracts/openapi.yaml` 1.1.0 already holds every operation, so WEB lanes build on generated mocks from hour 2 while API lanes fill the real handlers.
3. Automated tests only where a bug costs money or leaks data (§6). One end-to-end smoke test of the money path runs before every deploy.
4. Visual check of every screen at 390, 834 and 1280 px against the design PNGs with agent-browser (docs/16 §6).
5. Khai reads every diff that touches money, sign-in, roles, tenant scope or guest ID. Everything else: judge on screen.

What we cut (and why it is safe to cut now):

| Cut | Why |
| --- | --- |
| Pull requests, story points, sprint ceremonies | One person decides; `main` plus CI is the review |
| ADRs, progress.md, release notes, bug log, written plans | `git log` and the checkboxes here are the history; docs change only when a rule or the contract changes |
| Feature flags for new work | Nav items appear only when their slice is merged; existing flags stay on |
| Coverage gates, mutation, chaos, Lighthouse, pixel-perfect visual regression | Replaced by the targeted tests in §6 and the agent-browser check |
| Integration suite on every push | Runs at the two gates and nightly; unit tests and build run on every push |
| Hand-written UI components | shadcn/ui, Motion and the libraries in docs/16 |
| docs 07, 08, 09, 11, 12, 13 | Archived under `docs/archive/` as reference |

## 3. Sessions (lanes)

Run four Claude Code sessions, one terminal each, **each in its own clone** of the repository (`stayguard-api1`, `stayguard-api2`, `stayguard-web1`, `stayguard-web2`) so they never share a working tree. All work on `main`: `git pull --rebase` before each task, one commit per task, `git pull --rebase && git push` after it. With three sessions, run API-2 tasks after API-1 in the same session. The prompts are in §9.

| Session | Owns | Tasks |
| --- | --- | --- |
| API-1 | `api/` auth, staff, setup, bank, CLI | A0, L-A1 … L-A4, F-A1, F-A2 |
| API-2 | `api/` stays, shifts, owner monitoring, housekeeping, finance | L-B1 … L-B4, F-A3, F-A4 |
| WEB-1 | `web/` front desk, housekeeping, staff self-service | W0, L-W1 … L-W5, F-W1, F-W2 |
| WEB-2 | `web/` owner and settings | L-W6 … L-W9, F-W3 … F-W5 |
| Khai | accounts, server, reviews, gates | S0, G1, G2 |

Rules that keep four sessions from colliding:
- **Migration numbers are reserved per task** (below). If an earlier number is not merged yet, keep going; A0 enables out-of-order migrations in development, and both gates apply them all in order.
- WEB-2 starts after W0 is pushed (shared shells and components). Shared component changes after W0: WEB-1 owns `src/components/ui` and `src/components/shell`; WEB-2 asks in its final report instead of editing them.
- WEB lanes never wait for API: mocks come from the contract (`npm run gen`), and demo handlers in `src/mocks/setup` get realistic data for each new screen.
- Each task prompt: `Read CLAUDE.md, <service>/CLAUDE.md and task <ID> in docs/14. Do it exactly; one commit; tick the box.` Start each task with `/clear`.

## 4. Timeline and gates

| Hours | What |
| --- | --- |
| 0–2 | **Gate 0.** S0, A0, W0 |
| 2–20 | Launch tasks (L-…) in parallel |
| 20–24 | **Gate 1: go-live** with the first guesthouse (G1) |
| 24–44 | Fast-follow tasks (F-…), each deployed when green |
| 44–48 | **Gate 2:** full sweep, tag `v1.1.0`, handover (G2) |

If launch tasks slip, the go-live minimum is: L-A1, L-A2 (staff and PIN reset only), L-A3, L-B1, L-B2, L-W1 … L-W4, L-W6 (overview only). Everything else follows without blocking go-live.

## 5. Tasks

Each task: what to build (operations and design boards), migration number if any, required tests, and how to verify. Board names are file stems in `docs/assets/design/screens/` (index: `docs/assets/design/INDEX.md`). "Verify UI" always means docs/16 §6 at 390, 834 and 1280 px, in normal and reduced motion, **in both languages**: `/vi/…` against `<Board>.png` and `/en/…` against `<Board>EN.png`. Every UI task adds its strings to both `messages/vi.json` and `messages/en.json`, copying the English text from `source/<Board>EN.dc.html`.

### Gate 0

#### [ ] S0 Accounts and server (Khai, start immediately)
SePay account for the first guesthouse owner (Test mode first, bank linking can take time); VPS with Docker and a domain (Caddy for HTTPS); `deploy/.env.prod` outside git with `DATA_ENCRYPTION_KEY`, database password, `DEMO_MODE=0`; daily `pg_dump` to off-server storage.

#### [x] A0 Contract 1.1.0 wired (API-1) — no migration
`make gen` with the 1.1.0 contract; new operations return 501 through the existing stub path; `Role` gains MANAGER in domain and authorizer (no new permissions yet); goose migrate allows out-of-order in development (`goose.WithAllowOutofOrder`) but production `migrate` still applies in order. Verify: `make test-api`, `make lint`, `make up` boots, a new operation answers 501.

#### [x] W0 UI kit, shells and motion foundation (WEB-1)
Follow docs/16 §1–5 exactly: shadcn/ui init for Tailwind v4 and the listed components; theme variables mapped from `src/styles/globals.css`; Motion with `MotionConfig reducedMotion="user"` and the motion tokens; sonner Toaster; Be Vietnam Pro via `next/font`. Shells: phone bottom tab bar per role plus the owner More sheet (boards DieuHuongMobile, MenuChu, TongQuan), phone top bar with back for sub-pages and flows (no tab bar there), tablet icon rail, owner grouped sidebar (Monitor, Finance, Operations, People, Settings — show only items whose pages exist), front-desk top bar. Common states (MatMang, LoiMayChu, KhongCoQuyen, DanhSachTrong). Move `src/app/vi/*` to `src/app/[locale]/*` with `generateStaticParams` for `vi` and `en` (the Go server already serves the whole export); `messages/en.json` filled for every existing screen (from the EN boards); a missing key fails the build check `npm run check:i18n` that W0 adds. Replace hand-made buttons, cards, badges and the extras sheet in existing screens with the kit, keeping behaviour. Add `web/e2e/layout.spec.ts`: every route at 390, 834, 1280 has no page-level sideways scroll. `npm run gen` for new mocks.
Verify: build, lint, `npx playwright test e2e/layout.spec.ts`, agent-browser shots of `/vi/rooms` vs `Main`, `SoDoMayTinh`, `SoDoPhongTab`.

### Launch (hours 2–20)

#### [x] L-A1 Sign-in with PIN (API-1) — migration 0009
`signIn`, `signOut`, `changeMyPin`; users get `app_access`, `status`; `pin_credentials` (slow hash, failed count, `locked_until`, `must_change`); `tenants.guesthouse_code`. Lockout after 5 wrong PINs for 15 minutes plus ACCOUNT_LOCKED alert (write it to `audit_logs` until L-B1 adds alerts, then switch). Rate limit per IP and per guesthouse code. Demo sessions unchanged behind DEMO_MODE.
Tests (required): SG-701 AC1–AC5, tenant A user cannot sign in to tenant B.

#### [x] L-A2 Staff, roles and building access (API-1) — migration 0010
`listStaff`, `createStaff`, `updateStaff`, `resetStaffPin`, `lockStaff`, `unlockStaff`, `removeStaff` (owner PIN, 409 SHIFT_OPEN once L-B2 exists), `listStaffPermissions`, `setBuildingPermission`; `staff_profiles` with position and contract; building access table replaces `permissions.Derived`; MANAGER exclusions from docs/15 §2.
Tests (required): one table test over every operation × role (OWNER, MANAGER, RECEPTIONIST, HOUSEKEEPING) asserting allow or 403; building VIEW vs EDIT on a write.

#### [ ] L-A3 Tenant import, SePay and bank accounts (API-1) — migration 0011
`stayguard tenant import --file`, `stayguard sepay webhook|set-secret|status` (docs/runbooks/sepay-handover.md); `bank_accounts` (one default, others PENDING), `getProperty`, `updateProperty`, `listBankAccounts`, `createBankAccount`, `makeDefaultBankAccount`, `removeBankAccount`, `getSepayStatus`; `receiveBankWebhook` at `/v1/webhooks/bank/{hookId}` with HMAC per SePay's current docs (Khai pastes the link), mapped to the existing settlement handler; QR uses the default account.
Tests (required): docs/archive/14-demo-plan.md P3 test list; QR account comes only from the default account; secret never in logs.

#### [ ] L-A4 Setup: rooms, rates, extras basics (API-1) — migration 0012
`createBuilding`, `updateBuilding`, `createFloor`, `createRooms`, `updateRoom`, `listRatePlans`, `updateRatePlan`, `previewPrice`, `createService`, `updateService`, `restockService` (OPENING and IN movements; stock never edited directly).
Tests: `previewPrice` uses `domain/pricing` (one golden case through the endpoint); room with a guest cannot change type or retire.

#### [x] L-B1 Stays: edit time, move, history, receipt, alerts (API-2) — migration 0013
`editCheckInTime`, `moveStay`, `listStays` (date or range, receptionist window `frontDeskHistoryDays`, `guestId` indicators all false until F-A2), `getReceipt`, `getStayTimeline`; `alerts` table and the alert writer (STAY_TIME_EDITED, PAYMENT_MISMATCH, UNMATCHED_TRANSFER); `stay_edits`.
Tests (required): SG-801 AC1–AC3 (the edit changes the price through pricing; out-of-range 422), receptionist window 422.

#### [ ] L-B2 Shifts and cash (API-2) — migration 0014
`getCurrentShift`, `recordCashPayout`, `closeShift`, `getShiftReview`, `listClosedShifts`; a shift opens on the first cash action of a signed-in receptionist; expected cash = float + cash taken − payouts; closing locks figures and alerts on any difference.
Tests (required): expected cash with deposits, refunds and payouts; a closed shift cannot change; difference creates an alert.

#### [ ] L-B3 Owner monitoring (API-2) — migration 0015
`getOwnerOverview` v2 (`buildings`, `attention`), `listAlerts`, `markAlertRead`, `listTransactions`, `linkTransferToInvoice`, `listAuditLogs`.
Tests (required): link only by OWNER, only for bank events, once (409), settles the invoice through the settlement code path.

#### [ ] L-B4 Cleaning by any role, damage reports, tickets (API-2) — migration 0016
`completeHousekeepingTask` for OWNER, MANAGER, RECEPTIONIST, HOUSEKEEPING with EDIT; `reportDamage` (LOCK_ROOM sets MAINTENANCE, 409 if occupied); `listTickets`, `getTicket`, `updateTicket` (costs by OWNER; DONE unlocks the room; expense posting comes in F-A4).
Tests: role and building checks; lock refused with a guest.

#### [x] L-W1 Sign-in and account (WEB-1)
Boards: DangNhap, DangNhapPC, DoiPin, KhoaTaiKhoan, TaiKhoan. PIN with InputOTP; lockout countdown; must-change flow; the demo role picker shows only when demo mode is on. Motion: field error shake, button press.
Verify UI; wrong PIN five times on mocks shows the locked screen.

#### [x] L-W2 Room maps (WEB-1)
Boards: Main, SoDoMayTinh, SoDoPhongTab, SoDoPhongChu, SoDoPhongChuPC. Building chips with sliding selection, status counters with rolling numbers, tile status cross-fade and pulse, desktop and tablet side panel, ID indicator chips in the panel, tapping TO_CLEAN opens cleaning. Owner variant has no End shift or Payout.

#### [ ] L-W3 Stay flows (WEB-1)
Boards: NhanPhong, NhanPhongPC, ChiTiet, ThemDichVu, ThemDichVuPC, SuaGio, ChuyenPhong, TraPhong, TraPhongPC, ThanhToanQR, ThanhToanPC, ChuyenKhoanLech, QRHetHan, DaThanhToan, BienLai. The guest ID block in check-in stays hidden until F-W1. Paid moment: QR shrinks, tick draws, `navigator.vibrate(15)` where supported. Receipt prints at 80 mm. Amounts never animate.

#### [ ] L-W4 Shift and history (WEB-1)
Boards: ChiTrongCa, ChiTrongCaPC, GiaoCa, GiaoCaPC, LichSuLuotO, LichSuLeTanPC. Date bar with previous and next day, calendar popover, Today and Yesterday; ID columns (all "No" until F-A2).

#### [ ] L-W5 Cleaning and reports (WEB-1)
Boards: PhongCanDon, PhongCanDonPC, BuongPhongP, PhongCanDonBP, BaoHuHong, BaoPhongDung. Cleaned card slides out, counter ticks down, toast; optimistic update with rollback on error.

#### [ ] L-W6 Owner shell, overview, alerts, activity log (WEB-2)
Boards: TongQuan (phone), TongQuanTab, TongQuanPC, CanhBao, CanhBaoPC, NhatKy, NhatKyChonNgay, NhatKyPC. The old demo layout TongQuanChu is retired. Building status bars animate width on load; rolling KPI numbers; new-alert badge bounce.

#### [ ] L-W7 Money and shifts for the owner (WEB-2)
Boards: GiaoDichPC, LichSuGiaoDich, GanPhieu, GanPhieuPC, LichSuLuotOPC, ChiTietLuotO, ChiTietLuotOPC (ID block hidden until F-W1), DanhSachCa, DoiSoatCa, DoiSoatCaPC. Tables with TanStack Table, sticky first column, cards on phone.

#### [ ] L-W8 People (WEB-2)
Boards: NhanVien, NhanVienPC, ThemNhanVien, ThemNhanVienPC, PinMotLan, PinMotLanPC, XoaNhanVien, XoaNhanVienPC, PhanQuyen, PhanQuyenPC. One-time PIN shown once with copy button and a 24 h countdown.

#### [ ] L-W9 Settings (WEB-2)
Boards: CaiDat, CaiDatNhaNghi, CaiDatNhaNghiPC, ThemNganHang, ThemNganHangPC, ToaPhong, ToaPhongPC, ThemPhong, ThemTang, ThemToa, SuaPhong, BangGia, BangGiaPC, DichVuKho, DichVuKhoPC, ThemMatHang, ThemMatHangPC, SuaMatHang, SuaMatHangPC. Rate editor shows `previewPrice` live (debounced 300 ms).

### Gate 1 — go-live (hours 20–24)

#### [ ] G1 Go-live (Khai, one session helps)
1. `make test-api` and `make test-api-int` green; web build green; `e2e/smoke.spec.ts` (sign in → check-in → extras → check-out → transfer → SePay Test webhook → Paid → clean → shift close) green against the staging stack.
2. Server: non-superuser DB role (RLS on), no `ALLOW_PRIVILEGED_DB`, secrets from `deploy/.env.prod`, restart policy, HTTPS. Restore yesterday's backup once into a scratch database.
3. `stayguard tenant import` with the customer's real data (file outside git); set SePay secret; Test-mode transfer, then one live 2,000đ transfer; scan the QR with two banking apps.
4. Staff get one-time PINs; walk the receptionist through on their own phone.

### Fast-follow (hours 24–44)

#### [ ] F-A1 Stock complete (API-1) — migration 0017
`listStockMovements`, `removeService` (stop selling when sold), `createStocktake` with alerts; `soldLast7Days`, `latestUnitCost` on services.

#### [ ] F-A2 Guest ID (API-1) — migration 0018
`setGuestIdNumber`, `uploadGuestIdPhoto`, `getGuestIdRecord`, `revealGuestIdNumber`, `getGuestIdPhoto`, `deleteGuestIdPhoto`, `deleteGuestIdNumber`; `createStay` accepts `idConsent`; indicators on stays and lists; daily retention job; separate repository so front-desk queries cannot read the data.
Tests (required): SG-805 AC1–AC5, including a log-capture test proving no number or image bytes reach logs.

#### [ ] F-A3 Roster and leave (API-2) — migration 0019
`getRoster`, `putRoster`, `copyRosterWeek`, `listLeaveRequests`, `approveLeave`, `declineLeave`, `getMyRoster`, `listMyLeaveRequests`, `createLeaveRequest`, `cancelMyLeave`; scheduled shift used when a shift opens.

#### [ ] F-A4 Finance (API-2) — migration 0020
`getPayroll`, `updatePayrollLine`, `markPayrollPaid`, `getExpenseMonth`, `createExpense`, `updateExpense`, `deleteExpense`, `getIncomeCostReport`; automatic expense lines from payroll, tickets (DONE) and stock (IN); monthly recurring copy job.
Tests (required): payroll per pay type with the rounding rule; report totals reconcile with paid invoices and expense lines.

#### [ ] F-W1 Guest ID UI (WEB-1)
Show the check-in ID block, indicators everywhere, owner ID panel (masked number, Show and Hide, thumbnails), viewers XemGiayTo and XemGiayToPC; images fetched with no-store and revoked object URLs on close.

#### [ ] F-W2 My schedule and leave (WEB-1)
Boards: LichCuaToi, XinNghi, NghiCuaToi, HuyNghi, LichNghiLeTanPC.

#### [ ] F-W3 Stock pages (WEB-2)
Boards: ChiTietMatHang, ChiTietMatHangPC, NhapThemHang, NhapThemHangPC, XoaMatHangPC, KiemKho, KiemKhoPC.

#### [ ] F-W4 Maintenance and finance pages (WEB-2)
Boards: BaoTri, BaoTriPC, BaoTriChiTiet, BaoTriChiTietPC, ChiPhi, ChiPhiPC, ThemChiPhi, ThemChiPhiPC, BangLuongPC, BaoCao, BaoCaoPC. Charts with shadcn Chart (Recharts), bars grow on load.

#### [ ] F-W5 Roster for the owner, final language sweep (WEB-2)
Boards: LichCa, LichCaPC. Then open every route in `/en` and `/vi` at 390 and 1280 px and fix any text that overflows or is missing.

### Gate 2 (hours 44–48)

#### [ ] G2 Sweep and tag (Khai, one session helps)
Integration suite, smoke test, agent-browser sweep of every route at three widths (store the shots under `.shots/`, not in git), fix only what is wrong, deploy, tag `v1.1.0`, write the open issues as unchecked items under "Next week" below.

## 6. Tests that are required (and only these)

| Area | What must be tested | Where |
| --- | --- | --- |
| Pricing | Golden vectors (existing) + one through `previewPrice` | domain, endpoint |
| Money paths | Checkout invoice, settlement (existing), link transfer, shift expected cash, payroll, report reconciliation | app, integration |
| Tenancy | RLS coverage test (existing, automatic for new tables) | integration |
| Sign-in and roles | PIN, lockout, role × operation table | app |
| Guest ID | Owner-only access, no logging, retention | app, integration |
| End to end | `e2e/smoke.spec.ts` money path on the deployed stack | Playwright |
| UI | No sideways scroll at 390, 834, 1280 (`e2e/layout.spec.ts`); agent-browser visual check per task | Playwright, agent-browser |

Everything else: build, typecheck, lint and the task's manual check.

## 7. Next week (not in this sprint)

Advance bookings, OTA sync, e-invoices, automatic stay declaration, OCR of ID cards, multi-property dashboard, SSE instead of polling, trial clean-up job for the public demo.

## 8. Honest risks

- Four parallel sessions are the plan's engine; one long stall on API-1 (sign-in or SePay) delays go-live. Mitigation: the go-live minimum in §4.
- SePay activation and bank linking depend on the provider; start S0 now. Code is testable in Test mode without real money.
- Guest ID is personal data: F-A2 must not ship until SG-805 tests pass and the consent wording (docs/15 Q-06) is confirmed.
- 121 boards in 48 hours is only possible because components come from libraries; accept small spacing differences, never wrong data or missing states.

## 9. Session prompts

Paste one prompt per terminal, each in its own clone. The session works through its tasks in order and stops at the end of the list or when blocked. Khai reviews money, sign-in, roles, tenant scope and guest ID diffs as they land.

The full prompts (API-1, API-2, WEB-1, WEB-2) are in `docs/sessions/`. Each one names every task and every board, Vietnamese and English.
