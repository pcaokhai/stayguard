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
SePay secret with `sepay set-secret`, and runs the specs. Files run in name order (data-creating, then `ui-lint`, then the `z*` files that restart the API and use up the sign-in limit). Every spec file has the id first in the test title.

## Output

- `results-<date>.csv`: `id, title, status (pass/fail/skip), duration_s, evidence, notes`. Notes hold the first lines of the error, or the reason for a skip.
- `evidence-<date>/<id>/`: screenshots (taken at the end of every UI case), `api.log` (every API call and webhook the case made, secrets and ID numbers masked), `error.txt` on failure. Videos and zips are git-ignored.
- `shots-<date>/index.html`: the visual sweep contact sheet (git-ignored; open it locally).

Scope: portfolio demo, no real customer. Rows the checklist marks Bỏ qua are never written by the sync, and nothing needing a real phone, printer or bank is automated.
The CSV ends with a `# demo-check` section: one line per suite (spec file) with status, pass, fail, skip and seconds. Open bugs are in `bugs.md`.
One stack per clone: `scripts/rehearse-env.sh` gives the main clone (the directory named `stayguard`) the project `stayguard-rehearse` and port 18090, and every other
clone its own project `stayguard-rehearse-<dir>` and port (18100 + a number from the path), with its own `deploy/.env.rehearse`. Set `REHEARSE_PROJECT` / `REHEARSE_PORT` to override.
One run at a time per stack: `make rehearse-test`, `make demo-check`, `make rehearse` and `make rehearse-down` take `deploy/.rehearse.lock` and refuse (exit 3) while another holds it,
printing what holds it, its pid and since when. The destructive specs (API restart, late webhook) therefore never overlap another run. The runner exits 4, not just warns, when the
image it would test is not this commit (build failed, or the containers run an older image); the CSV records the commit.

## Case ids

The ids are the ones in `checklist.xlsx` (Mã). The title of each test starts with the id, and `scripts/rehearsal-sync.py` matches on it.
Automated: TT-01 to TT-14, TT-17, TT-20, TT-23 to TT-27 (money); CA-01 to CA-07, CA-10 (shift); DP-01, DP-02, DP-04 to DP-06, DP-09
(rooms); DN-01 to DN-07, DN-09, DN-12, DN-13 (roles); GT-02, GT-03, GT-05 (via DN-12), GT-06, GT-08 (guest ID); TD-02, TD-06;
BM-01 to BM-04; VH-05, VH-06; CD-10; LO-01 to LO-05, LO-07 to LO-10; CD-01 to CD-05, CD-08, CD-09; TD-01, TD-03, TD-04, TD-05, TD-07, TD-08; GD-03 to GD-07; GT-01, GT-04, GT-07, GT-09; DN-08, DN-10, DN-11.

Proposed new rows (not in the sheet yet):

| Id | Case | File |
| --- | --- | --- |
| UI-01 | UI lint: every screen of every role, vi/en, 390/1280: placeholders, raw i18n keys, undefined/NaN, raw action codes, empty actor, console errors, 4xx/5xx, horizontal scroll | `ui-lint.spec.ts` |
| CA-12 | A float left in the drawer is not counted twice by the owner overview | `shift.spec.ts` |
| DN-15 | A cookie-authenticated write without `X-Requested-With` is rejected (CSRF) | `z2-roles.spec.ts` |
| DN-16 | Hitting the sign-in rate limit is not shown as a locked account | `z2-roles.spec.ts` |
| TD-09 | "Cần xử lý" on Transactions counts only transfers still waiting | `alerts.spec.ts` |
| BM-06 | SePay's real retry behaviour (manual) | `z3-time.spec.ts` |
| VH-08 | The backdate helper refuses to run outside the rehearsal stack | `z3-time.spec.ts` |

Also in this run: `coverage.md` (every checklist row, automated or why not; `scripts/rehearsal-coverage.py`), `sync-preview.md` (what the sync would change, made on a copy;
the real workbook is changed only after Khai approves), `baseline/` (visual baselines VS-01 to VS-12, not approved) and `bill-lines.spec.ts` (BL-01 to BL-05, a stay for each bill-line
kind; `ui-lint` visits the owner stay page, the front-desk stay page, checkout and receipt of each).

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

`scripts/rehearsal-sync.py` reads the newest `results-*.csv` and, for rows whose Mã matches, keeps one line "QA tự động <date>: <pass|fail> …,
chưa thử tay; bằng chứng <path>" in Ghi chú, bằng chứng (the tester's own text stays). A failure also sets Trạng thái to Lỗi and Ngày thử; a pass
writes Đạt with the note "QA tự động <date>" (Demo scope: Khai decided), except the ids in `needs-eyes.txt` (GD-03 to GD-07, UI-01, TD-05, TD-08), which only get a note
asking for a person to look; a skip writes nothing, and rows set to Bỏ qua are never written. Rows with a cell reading "manual" or "[thủ công]" in the notes are left alone,
formulas are never overwritten, and the sheet XML is edited in place so styles and validation stay. `--selftest` checks it.

## Visual sweep

`RH_SHOTS=1 make rehearse-test` (or `scripts/rehearsal-shots.sh` after sourcing the `RH_ENV_OUT` file) uses agent-browser to capture the
main screens of the owner, manager, receptionist, housekeeping and signed-out views at 390, 834 and 1280 px, in vi and en, and writes
`shots-<date>/index.html`. There are no assertions: it is for looking.
