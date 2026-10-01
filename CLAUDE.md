# CLAUDE.md — StayGuard (FAST MODE)

StayGuard: anti-loss guesthouse app. Go API (chi, sqlc, PostgreSQL with RLS) serves a Next.js static export from one container.

**Current mode: FAST MODE** (docs/14). Goal: a polished demo today, a production v1 for one guesthouse within the following 24 hours. docs/14 overrides docs/07 and docs/11 while FAST MODE is on. Do only the task you were given from docs/14 §4 or §5.

## 1. Repository map

```
api/        Go backend (domain → app → adapter); migrations in api/migrations   → api/CLAUDE.md
web/        Next.js static export; generated client and mocks                    → web/CLAUDE.md
contracts/  openapi.yaml (source of truth), pricing vectors, demo-tenant-seed.json
deploy/     Dockerfile, compose.yaml
docs/       14 = current plan; assets/design = screen designs (source + PNG)
```

## 2. Commands

`make up` / `make down` (stack on :8080), `make migrate`, `make gen` (after any contract change), `make test-api`, `make test-api-int` (only for the areas in §4), `make test-web`, `make lint`. Web dev: `cd web && npm run dev` (mocks: `NEXT_PUBLIC_MOCK=1`). Show only the last 20 lines of command output.

## 3. Workflow in FAST MODE

1. Read this file, your service CLAUDE.md, the task in docs/14, and only the files the task names (docs/01–13 only when a task cites a section). No brainstorming, no worktrees, no written plan; the task list in docs/14 is the plan.
2. `git pull`, work on `main`, one commit per task: `feat(api|web): <summary> (<task id>)`, then push. No PRs.
3. Tests first only for the areas in §4. Everything else: build, typecheck, lint, and the manual check written in the task.
4. Before saying done: run the task's verification and paste the last lines of real output (`verification-before-completion`).
5. Bug you cannot explain in 10 minutes: `systematic-debugging`.
6. Tick the task's checkbox in docs/14 in the same commit. Nothing else to track. No ADR, release notes or bug log.
7. Final report ≤ 8 lines: what changed, what you ran, what the human must check by hand.

## 4. Keep strict (these are the product; never cut)

1. Money is `money.Vnd` (whole VND). No floats.
2. Prices come from `domain/pricing`; never compute a price in handlers or the web app.
3. Check-in, check-out and payment times come from the server clock.
4. A transfer becomes PAID only through the payment-event handler (simulator in demo, provider webhook in production). No endpoint or UI may mark a transfer paid by hand.
5. QR always pays the tenant's own account; no request field can change the account.
6. Every query is tenant-scoped; never weaken RLS or bypass the unit of work.
7. Writes that create stays, invoices or payments keep `Idempotency-Key`.
8. Do not collect national ID numbers in the UI (the field stays optional and hidden).
Tests are required for changes in: pricing, checkout/invoice, payments and settlement, tenant scoping, sign-in and roles.

## 5. Allowed shortcuts in FAST MODE

- New read-only endpoints: thin `handler → app service → repo`; no new domain types unless a rule needs one.
- Payment status: polling every 3 s (`getPayment`); SSE is not built.
- Housekeeping: rooms with status TO_CLEAN are the task list; no task table.
- Owner overview: totals and latest payments; alerts list may be empty.
- Web: Vietnamese only, but every string goes through the `t()` helper so English is a file later.
- Existing feature flags stay and are on in demo and production; new endpoints get no new flag (only DEMO_MODE gates demo routes).
- No PR size limit. No visual regression, Lighthouse, mutation or chaos runs.

## 6. Do not

- Rewrite or "flatten" existing api code, rename packages, or change the stack.
- Change `contracts/openapi.yaml` without running `make gen` in the same commit.
- Add dependencies beyond those named in the task.
- Commit secrets, real guest data, real bank accounts or customer names.
- Open generated code, lockfiles, node_modules, .next or golden-cases.json in full; search with grep instead.

## 7. Glossary

Tenant (one guesthouse or one trial) · Building · Room (table `units`) · Unit type (Standard/VIP, holds the rate plan) · Stay · Rental type HOURLY/OVERNIGHT/DAILY · Quote · Invoice (frozen quote) · Bill code (transfer note) · Payment (CASH/TRANSFER) · Payment event · Role OWNER/RECEPTIONIST/HOUSEKEEPING.
