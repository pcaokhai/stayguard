# CLAUDE.md — StayGuard

StayGuard is an anti-loss guesthouse management app: a Go API (PostgreSQL, multi-tenant) serving a Next.js static web app from one container. Staff check guests in and out, prices are computed on the server, transfers are confirmed only by server events, and owners control access per building.

This file is loaded into every session. Keep it short. Detailed rules live in `docs/`; read the doc that matches your task before you plan. Full workflow text: `docs/11-ai-workflow-and-tracking.md`.

## 1. Repository map

```
api/        Go modular monolith, migrations, generated server stubs   → api/CLAUDE.md   (lane API)
web/        Next.js static export, generated client, MSW mocks        → web/CLAUDE.md   (lane WEB)
contracts/  openapi.yaml, event schemas, pricing vectors, fixtures    (lane PLAT)
deploy/     Dockerfile, compose, Cloud Run notes                      (lane PLAT)
docs/       PRD, architecture, stories, plans, ADRs, bugs, tracking   (lane PLAT structure; every lane edits its own plans)
.github/    CI, PR template, CODEOWNERS                               (lane PLAT)
```

## 2. Which doc to read

| You are about to… | Read first |
| --- | --- |
| Start any story | docs/06 (story and AC), docs/07 §3–4 (slice, sprint, deps) |
| Touch REST or SSE | contracts/openapi.yaml, contracts/events/, docs/04 |
| Touch pricing | contracts/pricing/ (vectors and generator), docs/02 §7.7 |
| Touch tables or migrations | docs/05 |
| Make a design choice | docs/02, docs/adr/README.md |
| Write tests | docs/08 |
| Write any code | docs/10 (summary in §6 below) |
| Track work, log a bug, release | docs/11 |
Precedence when documents disagree: accepted ADR > contracts/ > architecture > stories > this file. Record the conflict as a Ruling in the plan and open a doc-fix PR.

## 3. Commands

`make up` (stack), `make down`, `make test` (all), `make test-api`, `make test-api-int`, `make test-web`, `make lint`, `make fmt`, `make gen` (sqlc and codegen), `make contracts` (lint, diff, vectors), `make migrate`, `make e2e`. SG-001 creates the Makefile; targets not yet implemented print which story adds them and exit non-zero.

## 4. How we work — Superpowers workflow (mandatory)

1. `brainstorming` only if the story leaves a design decision open (outcome: an ADR); otherwise write a one-paragraph understanding citing doc sections.
2. `using-git-worktrees`: one worktree and branch per story, `feat/<ID>-<slug>`, in `../stayguard-worktrees/`.
3. `writing-plans`: save to `docs/plans/<ID>.md`. **Override: plans contain no code** (files, interfaces, tests and verification in words; ≤ 120 lines). This beats the skill's default of complete code in every step.
4. `subagent-driven-development` (default) or `executing-plans` (small or tightly coupled).
5. `test-driven-development`: RED, GREEN, REFACTOR, always; test names end with `<story>_AC<n>`.
6. `verification-before-completion`: run the commands and show the last lines of real output before saying done.
7. `requesting-code-review`, then `receiving-code-review` (docs/10 §12).
8. `finishing-a-development-branch`: update tracking files (§11), rebase, green CI, squash merge with a Conventional Commit title.
Bugs: `systematic-debugging` first; reproduce, root cause, failing regression test, fix, then a bug-log entry (docs/11 §5).

## 5. Parallel work rules

- Contract first: boundary changes start with a `contracts/` PR the tech lead approves; merge order is contracts → providers → consumers → flag on.
- Slices ship together: an API story and a WEB story on one contract, in parallel worktrees, behind one flag `FF_<SLICE>_<NAME>`.
- Lanes own directories (§1). Edit only your lane's directory, your plan, and the tracking files. `dispatching-parallel-agents` only for tasks with disjoint file sets.
- WEB never waits for API: build on mocks generated from the contract; the real API is a config switch.
- Migrations: one owner (API); the number is reserved in the plan.
- Integration checkpoint per slice: `make up` with the flag on and mocks off, run the slice journey.

## 6. Engineering rules (summary — full text in docs/10)

1. Money is whole VND (`int64`); never floats or decimals.
2. Pricing is a pure function; rule changes start in `contracts/pricing/generate_vectors.py`, then regenerate vectors.
3. Times (check-in, check-out, payment) come from the server clock, never from clients.
4. A transfer becomes PAID only in the payment-event handler; no other path, ever.
5. Every query is tenant-scoped; RLS is the second guard, not the first.
6. Authorize every request by role and building (docs/04 §2.3).
7. Writes with `Idempotency-Key` are idempotent; payment events dedupe by `(provider, external_id)`.
8. Hexagonal layering (`domain` → `app` → `adapter`); no framework or I/O in `domain`.
9. Small interfaces at the consumer; constructor injection; no globals or mutable singletons.
10. Errors are classified and mapped in one place; never swallowed; no personal data or secrets in logs.
11. Every I/O has a timeout; every goroutine has an owner and a cancel path.
12. All UI strings come from message files in both languages; accessibility is part of done.
13. Functions ≤ ~30 lines, files ≤ ~300 lines, comments explain why, no magic numbers.
14. Never edit generated code; regenerate it.
15. Conventional Commits with lane scope and story id; PRs < 400 changed lines excluding generated code.

## 7. Definition of Done

- [ ] Every AC is proven by a named test that failed first
- [ ] `make lint`, `make test`, `make contracts` green; security scans clean
- [ ] Logs and metrics added; no personal data logged
- [ ] Docs updated in the same PR; tracking files current (§11)
- [ ] For slices: partner story merged behind the flag; checkpoint passed
- [ ] Review done against docs/10 §12

## 8. Domain glossary (use these names in code)

- **Tenant**: one guesthouse business, or one trial. **Trial**: a tenant with an expiry.
- **Property, Building**: a site and its buildings. **Room**: a rentable space (domain type `Room`; table `units`, because other verticals have other unit kinds). **Unit type**: Standard or VIP, holds the rate plan.
- **Rate plan**: versioned prices and windows; a **snapshot** is copied onto each stay.
- **Stay**: one guest's occupancy. **Rental type**: HOURLY, OVERNIGHT, DAILY. **Quote**: the price of a stay at an instant, with lines.
- **Invoice**: the frozen quote at check-out. **Bill code**: short reference used as the transfer note.
- **Payment**: an attempt to settle an invoice (CASH or TRANSFER). **Payment event**: an inbound report from the bank provider or the simulator.
- **Shift**: a user's working period. **Cash entry**: one movement of drawer cash. **Float**: cash left in the drawer between shifts. **Reconciliation**: comparing counted and expected cash.
- **Housekeeping task**: a room to clean. **Alert**: something the owner should look at.
- **Building access**: NONE, VIEW or EDIT per user per building. **Role**: OWNER, RECEPTIONIST, HOUSEKEEPING.
- **Lane**, **slice**, **flag**: see docs/07.

## 9. Things Claude must not do

- Change `contracts/` inside a feature branch, or hand-edit generated code
- Add a dependency or a new tool without naming it in the plan and the PR
- Weaken, skip or delete tests, gates or lint rules to get green
- Commit secrets, real personal data, real bank accounts or customer names (this repository becomes public)
- Mark a transfer paid by any path except the payment-event handler; add any endpoint that takes an account number for the QR
- Use floats for money, read the clock inside pricing, or trust a time sent by a client
- Write code inside a plan, or rewrite an accepted ADR (supersede it)
- Edit another lane's directory, push to `main`, force-push, or open a PR from the private repository to the public one
- Invent library versions in documents; pin real ones in code and record them in the PR

## 10. Token discipline

Full rules: docs/11 §4. In short: one story per session; `/clear` between stories and `/compact` near 60% context; search with `grep -n` and view line ranges instead of opening big files; never open generated code, lockfiles, `node_modules`, `dist` or `golden-cases.json` in full; cite doc sections instead of pasting them; keep command output to `tail -n 20` or the failing part; run narrow tests while iterating and the full suite once at the end; delegate exploration to subagents with a return format of at most 150 words; do not restate diffs; the final report is at most 10 lines.

## 11. Tracking files (every story)

Update in the story's PR: `docs/progress.md` (your row), `docs/release-notes.md` (Unreleased entry for user-visible features), `docs/adr/` (new or superseding ADR for any hard-to-reverse decision), `docs/bugs/` (entry with root cause, fix, options and trade-offs for every bug fixed), and the plan's Status line.
