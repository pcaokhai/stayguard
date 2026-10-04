# CLAUDE.md — StayGuard (SHIP MODE)

StayGuard: anti-loss guesthouse app. Go API (chi, sqlc, PostgreSQL with RLS) serves a Next.js static export from one container.

**Current mode: SHIP MODE** — a 36–48 hour production sprint (docs/14). Do only the task you were given from docs/14 §5. Product rules: docs/15. UI kit and motion: docs/16. API: `contracts/openapi.yaml` 1.1.0.

## 1. Repository map

```
api/        Go backend (domain → app → adapter); migrations in api/migrations   → api/CLAUDE.md
web/        Next.js static export; shadcn/ui kit, generated client and mocks     → web/CLAUDE.md
contracts/  openapi.yaml (source of truth), pricing vectors, fixtures
deploy/     Dockerfile, compose files
scripts/    ui-shots.sh (agent-browser screenshots), licence check
docs/       14 plan · 15 product rules and routes · 16 UI kit · assets/design (PNG + markup per board) · runbooks · archive
```

## 2. Commands

`make up` / `make down` (stack on :8080), `make migrate`, `make gen` (after any contract change), `make test-api`, `make test-api-int` (gates and money areas), `make test-web`, `make lint`. Web: `cd web && npm run dev` (mocks: `NEXT_PUBLIC_MOCK=1`), `npx playwright test e2e/layout.spec.ts`. Visual check: `scripts/ui-shots.sh <route> <Board> [WxH]`. Show only the last 20 lines of command output.

## 3. Workflow

1. Read this file, your service CLAUDE.md and your task in docs/14. Open docs/15 or docs/16 sections only when the task needs them; never read docs/archive.
2. Each session has its own clone. `git pull --rebase`, work on `main`, one commit per task: `feat(api|web): <summary> (<task id>)`, then `git pull --rebase && git push`. No PRs, no written plans.
3. Tests only for the areas in docs/14 §6 (write those first). Everything else: build, typecheck, lint, the task's check.
4. UI tasks: compare every board in the task with agent-browser at 390, 834 and 1280 px, in Vietnamese (`/vi`, `<Board>.png`) and English (`/en`, `<Board>EN.png`) (docs/16 §6), and report match or differences.
5. Before saying done: run the task's verification and paste the last lines of real output.
6. Bug you cannot explain in 10 minutes: stop guessing, add logging or a failing test, then fix (`systematic-debugging`).
7. Tick the task's checkbox in docs/14 in the same commit. Change docs only when you change a rule or the contract.
8. Final report ≤ 8 lines: what changed, what you ran, boards match/differences, what Khai must check by hand.

## 4. Keep strict (never cut)

1. Money is `money.Vnd` (whole VND). No floats.
2. Prices come from `domain/pricing`; never compute a price in handlers or the web app.
3. Check-in, check-out and payment times come from the server clock.
4. A transfer becomes PAID only through the payment-event handler (SePay webhook or the demo simulator). The only other path is the owner linking a bank-reported event (`linkTransferToInvoice`). No UI or endpoint marks a transfer paid by hand.
5. QR always pays the tenant's default, SePay-connected account; no request field can change the account.
6. Every query is tenant-scoped; never weaken RLS or bypass the unit of work. New tables: RLS + grants + isolation fixture in the same migration.
7. Writes that create stays, invoices, payments or other money records keep `Idempotency-Key`.
8. Guest ID number and photos are optional, need consent, are encrypted, and are write-only for front desk and housekeeping; only OWNER and MANAGER read them, through audited endpoints (docs/15 rules 21–25). Never log them.
9. PINs, secrets, account numbers and QR payloads never appear in logs, errors or fixtures.

## 5. Allowed shortcuts

- New read endpoints may be thin `handler → app service → repo`; no new domain types unless a rule needs one.
- Payment status by polling every 3 s; no SSE.
- No new feature flags; a page appears in navigation only when its task is merged. Existing flags stay on.
- Use the reserved migration number from the task; out-of-order is fine in development.
- Web: every string through `t()`; each UI task fills `messages/vi.json` and `messages/en.json` together (English text from the `EN` boards).

## 6. Do not

- Rewrite existing api code, rename packages or change the stack.
- Hand-build a UI primitive that shadcn/ui or a docs/16 library provides.
- Change `contracts/openapi.yaml` without `make gen` in the same commit, or change an operation another lane is building without saying so in your report.
- Add dependencies beyond docs/16 and the task.
- Commit secrets, real guest data, real bank accounts or customer names.
- Open generated code, lockfiles, node_modules, .next or golden-cases.json in full; grep instead.
- Add a directory to `.gitignore` (a line ending in `/`) without a matching `Read(./dir/**)` deny rule in `.claude/settings.json`; `scripts/check-claude-deny.sh` fails CI otherwise.

## 7. Glossary

Tenant (one guesthouse or one trial) · Building · Room (table `units`) · Unit type (holds the rate plan) · Stay · Rental type HOURLY/OVERNIGHT/DAILY · Quote · Invoice (frozen quote) · Bill code (transfer note) · Payment (CASH/TRANSFER) · Payment event · Shift · Alert · Ticket (maintenance) · Roles OWNER/MANAGER/RECEPTIONIST/HOUSEKEEPING; app access NONE for staff without sign-in.
