# Demo data: two paths, and the payment simulator

There are two ways to get a guesthouse to show. They are different stacks with different data; pick by what you want to show.

| | Role picker (trial tenant) | `make demo-reset` |
| --- | --- | --- |
| Server | `DEMO_MODE=1` (the public demo, `deploy/compose.demo.override.yaml`) | `DEMO_MODE=0`, real PIN sign-in, own compose project `stayguard-demo` on `http://localhost:18200` |
| Web build | `NEXT_PUBLIC_DEMO_MODE=1` (the default) | `NEXT_PUBLIC_DEMO_MODE=0` |
| Sign in | pick a role on `/vi`; each visitor gets a new empty-ish trial tenant (24 h) | guesthouse `demo` plus user name and PIN |
| Data | `DemoSeeder` (`api/internal/app/demo_seed.go`, `contracts/fixtures/demo-tenant-seed.json`): 35 rooms, rate plans, services, 11 sample stays, one extras line, two unmatched bank transfers (one unread alert, one read), one closed morning shift with the drawer counted exactly | the installer import (`scripts/demo/tenant.json`) plus a worked day (`scripts/demo/populate.py`) |
| Staff and PINs | none (roles only) | owner, manager, two receptionists, housekeeping |
| Payment simulator | yes | **no** |

## `make demo-reset`

Wipes the stack `stayguard-demo` (never `stayguard-rehearse`; the stack is its own compose project and database volume), builds it,
migrates, imports the guesthouse with the installer command (`stayguard tenant import`), stores a SePay test secret
(`stayguard sepay set-secret`), then works the guesthouse through the public API. Nothing is written to the database directly.
Run it again any time: everything is deleted first (about a minute). The stack is per clone, named like the rehearse stack (`scripts/rehearse-env.sh`): the main clone (directory `stayguard`) uses project `stayguard-demo` on port 18200 (database 18201); any other clone uses `stayguard-demo-<directory>` and a port from 20000 up made from its path, and the script prints the address. `DEMO_PROJECT`, `DEMO_PORT` and `DEMO_DB_PORT` move the ports.

It prints the sign-in details at the end:

| User | PIN | Role |
| --- | --- | --- |
| `owner` | 482915 | Owner |
| `mina` | 739106 | Manager (both buildings) |
| `linh` | 260814 | Receptionist, edits building A, views B |
| `viv` | 731902 | Receptionist, edits building B, views A |
| `hoa` | 846205 | Housekeeping |

Guesthouse code `demo`. The PINs are for the demo only (the import's one-time PINs are changed by the script).

What it creates, for the day it runs:

- 2 buildings, 35 rooms (A: 3 floors of 6, B: 6 + 6 + 5; the top floor is VIP), two room types with rates, 5 extras with stock.
- Rooms in use: hourly, overnight and daily stays in both buildings; `A201` overdue (its check-in time was edited, which alerts the owner).
- Bills: one paid by cash (room waits to be cleaned), one paid by a signed SePay transfer and cleaned by housekeeping, one with the QR shown and nothing paid yet, one short transfer (the remainder is open), one overpaid transfer, one transfer with no bill code (unmatched), a cash refund of an over-large deposit.
- A damage report that locks `A205` with a repair ticket in repair, expenses of the month (rent recurring, electricity, water, supplies), a roster for the week, a pending leave request, and one closed shift with a small cash shortage.
- Alerts you will see: unmatched transfer, overpaid, stay time edited, cash short, damage reported, leave requested, SePay updated. The short-transfer, unpaid-bill and refund alerts come from timers: they appear after 15 or 30 minutes when `stayguard jobs run` runs (it is not started by this stack).

The rooms and codes are fixed, so a script can refer to them (vacant for a live demo: for example `A301`, `A304`, `B101`).
`scripts/demo/populate.py` prints a `warning:` line for any scenario step a rule refused; a clean run prints none.

## The in-app payment simulator

- **What it does:** `POST /v1/demo/payments/{paymentId}/simulate` plays the bank. For a pending TRANSFER payment it creates a
  payment event with the exact amount and the bill code (provider `simulator`) and runs the same settlement handler as the SePay
  webhook (`Payments.Simulate`, `Payments.settle`). Roles: front desk with EDIT on the building. A payment that is not a pending
  transfer is returned unchanged. A simulator event is never money: the owner cannot link it to an invoice.
- **Needs both switches:** the server must run with `DEMO_MODE` on (otherwise the route answers 404 `DEMO_DISABLED`), and the web
  build must have `NEXT_PUBLIC_DEMO_MODE=1` (the default; production builds and `make smoke` set it to 0).
- **When the button shows:** the web shows "Giả lập tiền về" / "Simulate payment received" on the QR screen only after the demo
  **role picker**: the picker sets a per-tab marker (`stayguard.demo` in `sessionStorage`, `web/src/lib/session.ts`) and the button checks it.
  A PIN sign-in never sets it, so on `make demo-reset` there is no button even if the server had `DEMO_MODE` on. Pay by a signed
  bank delivery there (see `docs/demo-script.md`).
- **If a PIN-signed demo should get the button:** the web needs a signal from the server. The smallest change is a boolean
  `demoMode` on `Me` (or on the sign-in response) set from `DEMO_MODE`, and `isDemo()` returning true when it is set. Not built.

## Which screens use which path

| Screen | Role picker trial tenant | `make demo-reset` |
| --- | --- | --- |
| Room map, check-in, extras, check-out, QR payment, cleaning | yes (with the simulator) | yes (signed webhooks) |
| Sign-in, account, PIN change, staff list, access | no (roles only) | yes |
| Owner overview, stays and history | partly (sample stays, no invoice) | yes |
| Transactions (Giao dịch) | two sample bank transfers (unmatched, so the owner can link them) | yes |
| Alerts (Cảnh báo) | one unread alert (unmatched transfer) and one read | yes |
| Shift review (Đối soát ca) | one closed shift (morning, no difference) | yes |
| Activity log | only what the visitor does | yes |
| Expenses, payroll, roster, reports, maintenance | empty | yes |
| Guest ID viewer | only after the visitor enters an ID number | yes |
