# Session API-2

Paste everything below the line into a fresh Claude Code session started in the `stayguard-api2` clone.

---

You are session **API-2** of the StayGuard production sprint (SHIP MODE). You own `api/`: stays, shifts and cash, owner monitoring, housekeeping and maintenance, roster and leave, payroll, expenses and reports.

Read CLAUDE.md, api/CLAUDE.md and docs/14-production-sprint.md §1–6 now. Then do these tasks from docs/14 §5, in this order, one task at a time, using `/clear`-style focus: forget the previous task's details before starting the next.

Before L-B1: `git pull --rebase` until the A0 commit from API-1 is on main (contract 1.1.0 wired, `make gen` done). Use only your reserved migration numbers (0013–0016, 0019, 0020). L-A1 to L-A4 are built in parallel by API-1; where you need their tables (users, building access), code against the ports and pull before testing.

1. **L-B1** — Stays: edit time, move, history, receipt, alerts (API-2) — migration 0013
2. **L-B2** — Shifts and cash (API-2) — migration 0014
3. **L-B3** — Owner monitoring (API-2) — migration 0015
4. **L-B4** — Cleaning by any role, damage reports, tickets (API-2) — migration 0016
5. **F-A3** — Roster and leave (API-2) — migration 0019
6. **F-A4** — Finance (API-2) — migration 0020

Rules for this session:
- You are one of four parallel sessions; this clone is only yours. Before each task `git pull --rebase`; after it, one commit `<type>(<api|web>): <summary> (<task id>)` that also ticks the task in docs/14, then `git pull --rebase && git push`. If the rebase conflicts in a file another lane owns, keep their version and redo your part.
- Start every task by re-reading its section in docs/14 §5. Read docs/15 or docs/16 sections only when the task needs them. Never read docs/archive.
- Keep the strict rules in CLAUDE.md §4. Write the tests listed in docs/14 §6 for your task first.
- Before ticking a task, run its verification and paste the last lines of real output in your report.
- If you are blocked for more than 15 minutes (missing decision, external account, failing dependency from another lane), write the blocker in your report, leave the box unticked, and move to the next task that does not depend on it.
- After each task, report in at most 8 lines: what changed, what you ran, what Khai must check by hand, and (UI tasks) each board as match or differences in vi and en.
- Continue with the next task without waiting for me unless the task says Khai must decide or review first.

When the list is done, run the full verification for your lane once more (`make test-api`, `make test-api-int`, `make lint`) and report the result.
