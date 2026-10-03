# Demo script: StayGuard in 3 minutes

Audience: a guesthouse owner. Every step uses behaviour that exists on `main`. Walked in the running app on 2026-10-03 against `make demo-reset` (details in docs/runbooks/demo.md).

Legend: ✅ walked in the running app and/or covered by a named automated test · ⚠️ not walked in the running app (reason given).

## Set up

1. `make demo-reset` (about a minute; it prints the address, `http://localhost:18200` in the main clone). Sign in on `/vi` with guesthouse code **`demo`**.
2. Two laptops or two browser profiles: one for the receptionist, one for the owner (one sign-in per person).
3. Accounts (demo only, from the runbook): **`linh`** receptionist, PIN 260814 · **`owner`** owner, PIN 482915 · `mina` manager, `viv` receptionist (building B), `hoa` housekeeping.
4. Vacant rooms to use live: **A301**, **A304**, **B101**. Already prepared by the reset: **A106** (short transfer, remainder open), **A202** (QR shown, unpaid), **A201** (overdue).
5. The in-app "Giả lập tiền về" button does not exist on this stack (PIN sign-in). To pay the live QR use a signed bank delivery (`scripts/demo/populate.py` shows the call) or, for a button-driven demo, use the role picker trial stack instead (it has the button; its data is thinner).

## Walkthrough

| Time | Sign in as | Do | Say | Check |
|---|---|---|---|---|
| 0:00 | `linh` (receptionist) | Sign in: code, user name, PIN. The room map opens. | "Staff sign in with their own PIN. Five wrong tries lock the account; too many requests just ask to wait." | ✅ walked; `signInErrors.test` |
| 0:20 | `linh` | Tap **A301** → Nhận phòng. Qua đêm, name, phone, default deposit. The ID number is optional. | "The ID is optional, and only the owner and managers can ever read it." | ✅ walked (390 and 1280); `smoke.spec` step 2; `rehearsal/guestid.spec` GT-02 |
| 0:45 | `linh` | Add extras (water). | "Prices come from the server; the screen never calculates them." | ✅ `smoke.spec` step 3 · ⚠️ not walked on the demo stack |
| 1:00 | `linh` | Trả phòng → **Chuyển khoản**. The QR screen shows the amount, a masked account (`******8888`), the bill code. | "The QR always pays the guesthouse's own account." | ✅ walked (390 and 1280); `smoke.spec` step 4 |
| 1:15 | (the bank) | Send the signed bank delivery for that bill code. The screen turns **Paid** by itself within 3 seconds. | "Only the bank's report marks it paid. Nobody can tick it by hand." | ✅ `smoke.spec` step 5 · ⚠️ not repeated by hand on the demo stack; a real banking app: not verified |
| 1:30 | `linh` | Open **A106** (a short transfer was received): the screen shows what arrived and what is left, with a QR for the remainder on the same bill code. | "A guest who sends too little just tops up. Nothing is lost." | ✅ `smoke.then-partial.spec`; the room is on the map as "Chờ thanh toán · Còn thiếu 70.000đ" (walked) · ⚠️ A106 payment screen itself not opened |
| 1:55 | `linh` | **Ca làm** (Shift): cash expected, count by notes. Closing with a difference needs a reason; unpaid bills are listed. | "Closing short needs a reason, and the owner is told." | ✅ walked: shows "Thiếu …", the unpaid list and the reason rule · did not press close |
| 2:15 | `owner` | Sign in on the second laptop. **Tổng quan**: revenue today, transfers received, cash expected, rooms in use, building cards. | "The owner sees the money as it lands, on any device." | ✅ walked (390 and 1280); `smoke.spec` step 8 |
| 2:30 | `owner` | **Cảnh báo**: unmatched transfer, overpaid, stay time edited, cash short, damage, leave request. | "Problems come to you, and close by themselves when fixed." | ✅ walked; `rehearsal/alerts.spec` TD-06 |
| 2:40 | `owner` | **Nhật ký**: check-ins, payments, the shift close, expenses; filter by person or type. | "Every sensitive action is written; nobody can edit it." | ✅ walked |
| 2:48 | `owner` | **Lượt ở** → open a stay with an ID → **Giấy tờ khách**: `079******789`, press **Hiện** to show it, then look at the log: a line "Xem số giấy tờ khách · A302", no number in it. | "Owner and manager only. Every view is logged." | ✅ walked (number revealed; log line present, number absent); GT-03 |
| 2:56 | `owner` | **Tài khoản** → **Ngôn ngữ**: Tiếng Việt / English. Screens follow. | "Vietnamese and English, per person." | ✅ walked the Account page and the switch control; `layout.spec` renders every route in vi and en · ⚠️ did not press English in this run |

## If something goes wrong on stage

- "Too many attempts": wait the seconds shown, or sign in with another account.
- A transfer does not turn Paid: the screen polls every 3 s; open the room again, it resumes the same bill code. Offline: the screen says so and recovers when the connection returns.
- Use only made-up ID numbers.

## Known issue seen while walking (not fixed here, not mine)

- Owner → Lượt ở → open a stay with an early check-in fee: the bill line shows the raw key `ownerStays.line.EARLY_CHECKIN_HOUR` instead of text (owner stay page, `ownerStays.line.*` messages). Avoid showing that stay, or add the label.

## Screens and GIF

`docs/assets/portfolio/`: `01-room-map` … `08-activity-log`, each at `-390.png` and `-1280.png` (Vietnamese, `make demo-reset` data, receptionist `linh` for 01–04, `owner` for 05–08), and `payment-flow-390.gif` (role picker trial tenant: check-in, check-out, QR, "Giả lập tiền về", Paid). The account number is masked on screen; the QR image encodes the demo account only.
