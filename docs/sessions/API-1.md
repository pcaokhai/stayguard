# Session API-1

Paste everything below the line into a fresh Claude Code session started in the `stayguard-api1` clone.

---

You are session **API-1** of the StayGuard production sprint (SHIP MODE). You own `api/`: contract wiring, sign-in, staff and roles, tenant import, SePay and bank accounts, setup, stock and guest ID.

Read CLAUDE.md, api/CLAUDE.md and docs/14-production-sprint.md §1–6 now. Then do these tasks from docs/14 §5, in this order, one task at a time, using `/clear`-style focus: forget the previous task's details before starting the next.

Before A0: Khai has applied the docs commit locally without pushing it. A0 runs `make gen` against contract 1.1.0, so push the docs commit and A0 together (CI checks that generated code is current). For L-A3, ask Khai for the SePay webhook documentation link before writing the signature check; do not guess the payload or header. For F-A2, stop after the tests pass and ask Khai to review before pushing (guest ID is personal data).

1. **A0** — Contract 1.1.0 wired (API-1) — no migration
2. **L-A1** — Sign-in with PIN (API-1) — migration 0009
3. **L-A2** — Staff, roles and building access (API-1) — migration 0010
4. **L-A3** — Tenant import, SePay and bank accounts (API-1) — migration 0011
5. **L-A4** — Setup: rooms, rates, extras basics (API-1) — migration 0012
6. **F-A1** — Stock complete (API-1) — migration 0017
7. **F-A2** — Guest ID (API-1) — migration 0018

Rules for this session:
- You are one of four parallel sessions; this clone is only yours. Before each task `git pull --rebase`; after it, one commit `<type>(<api|web>): <summary> (<task id>)` that also ticks the task in docs/14, then `git pull --rebase && git push`. If the rebase conflicts in a file another lane owns, keep their version and redo your part.
- Start every task by re-reading its section in docs/14 §5. Read docs/15 or docs/16 sections only when the task needs them. Never read docs/archive.
- Keep the strict rules in CLAUDE.md §4. Write the tests listed in docs/14 §6 for your task first.
- Before ticking a task, run its verification and paste the last lines of real output in your report.
- If you are blocked for more than 15 minutes (missing decision, external account, failing dependency from another lane), write the blocker in your report, leave the box unticked, and move to the next task that does not depend on it.
- After each task, report in at most 8 lines: what changed, what you ran, what Khai must check by hand, and (UI tasks) each board as match or differences in vi and en.
- Continue with the next task without waiting for me unless the task says Khai must decide or review first.

When the list is done, run the full verification for your lane once more (`make test-api`, `make test-api-int`, `make lint`) and report the result.
