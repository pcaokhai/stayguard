# Rehearsal checklist, automated

`make rehearse-test` runs the rehearsal checklist with no tokens and no clicking: a fresh guesthouse on the rehearse stack
(compose project `stayguard-rehearse`, `http://localhost:18090`), the Playwright specs in `web/e2e/rehearsal/`, and a CSV
plus evidence. One run takes about 5 to 8 minutes. Never real money, never real keys: payments arrive as signed SePay webhooks
made with the guesthouse's own test secret and account number.

```
make rehearse-test                      # whole checklist
RH_ONLY='TT-0' make rehearse-test       # only the cases whose title matches (regular expression)
RH_SHOTS=1 make rehearse-test           # then the visual sweep too
RH_ENV_OUT=/tmp/rh.env make rehearse-test   # only prepare the guesthouse; `. /tmp/rh.env` and run one spec by hand
```

Each run starts or reuses the stack, stops the `jobs` service (so the alerts the specs count are raised only by the specs; it is
started again at the end), imports a guesthouse `rh<hex>` with the installer commands (owner, manager, receptionists r1 to r12, a housekeeper, 120 rooms), sets the
SePay secret with `sepay set-secret`, and runs the specs. Every spec file has the id first in the test title.

## Output

- `results-<date>.csv`: `id, title, status (pass/fail/skip), duration_s, evidence, notes`. Notes hold the first lines of the error, or the reason for a skip.
- `evidence-<date>/<id>/`: screenshots (taken at the end of every UI case), `api.log` (every API call and webhook the case made, secrets and ID numbers masked), `error.txt` on failure. Videos and zips are git-ignored.
- `shots-<date>/index.html`: the visual sweep contact sheet (git-ignored; open it locally).

## Case ids

The ids follow the order of the checklist brief. They are provisional until the real `checklist.xlsx` is committed here: rename a
case by changing the id at the start of its test title, and `scripts/rehearsal-sync.py` matches on that id.

| Prefix | Area | File |
| --- | --- | --- |
| TT-01 to TT-15 | Money: exact, unmatched, link (exact, different amount, twice, receptionist), short then remainder QR, overpaid, outgoing, duplicate, cash refund on the shift, resume payment, reopened checkout deposit, finished stay read-only, bank time and negative refund lines | `money.spec.ts` |
| SH-01 to SH-08 | Shift: opening float, payout note, close exact / short / with unpaid invoices, owner refund "by owner", one expected-cash figure | `shift.spec.ts` |
| RM-01 to RM-05 | Rooms: cleaning by receptionist and owner, to-clean actions, damage lock, maintenance cost to expense | `rooms.spec.ts` |
| RL-01 to RL-09 | Roles: no-permission screen, manager limits, lockout, session revocation (PIN change, lock, remove), second tab, CSRF, rate limit | `roles.spec.ts` |
| GI-01 to GI-03 | Guest ID: indicators only, masked number and audited reveal, no number in logs | `guestid.spec.ts` |
| AL-01 to AL-05 | Alerts resolve when fixed; "Cần xử lý" counts only open items | `alerts.spec.ts` |
| TM-01 to TM-06 | Time-based rules without waiting, late webhook, the backdate guard | `time.spec.ts` |
| OP-01 to OP-03 | Restart keeps sign-in, `/readyz`, no secrets in logs | `ops.spec.ts` |

## Time-based cases (never wait)

`scripts/rehearsal-backdate.sh <partial|unpaid> <billCode> <minutes>` with `RH_TENANT=<guesthouse code>` moves one invoice's records back in
the rehearsal database only: `partial` moves the bank money's `received_at` (PAYMENT_PARTIAL after 15 minutes), `unpaid` moves the
invoice's `created_at`, the check-out time (PAYMENT_UNPAID or REFUND_PENDING after 30 minutes). It refuses unless the compose project is
`stayguard-rehearse`, the database container really belongs to that project, and the guesthouse is a rehearsal one (`rehearse` or `rh` + 6 hex
digits); TM-06 checks the refusals. The specs then run `stayguard jobs run` once and assert exactly one alert, and none on a second run.

The late-webhook case (TM-04) stops the API, restarts it and delivers a transfer signed 6 minutes earlier: `SEPAY_TIMESTAMP_TOLERANCE`
is 300 s by default, so it is refused with 401 and nothing is paid; the same transfer signed 4 minutes earlier is accepted.

## Cases that stay manual

| Id | Why |
| --- | --- |
| TM-05 | SePay's real retry behaviour (how often and for how long it retries, whether each retry is re-signed) needs SePay itself and a Test-mode transfer. |
| Real money path | The 2,000 d transfer in `docs/runbooks/sepay-handover.md`, and the Cloudflare tunnel reaching the stack. |
| Real device checks | Camera capture of ID photos on a phone, Safari and iOS behaviour, printing. |

## Sync to the spreadsheet

`scripts/rehearsal-sync.py` reads the newest `results-*.csv` and fills the Status, Date and Notes columns of `docs/rehearsal/checklist.xlsx`
for rows whose ID matches (the sheet needs a header row with ID, Status, Date and Notes). It never changes a row that has a cell reading
"manual", never overwrites a formula, and edits the sheet XML in place so styles and validation stay. `scripts/rehearsal-sync.py --selftest` checks it.

## Visual sweep

`RH_SHOTS=1 make rehearse-test` (or `scripts/rehearsal-shots.sh` after sourcing the `RH_ENV_OUT` file) uses agent-browser to capture the
main screens of the owner, manager, receptionist, housekeeping and signed-out views at 390, 834 and 1280 px, in vi and en, and writes
`shots-<date>/index.html`. There are no assertions: it is for looking.
