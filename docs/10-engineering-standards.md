# Engineering Standards

Version 1.0 · 2026-09-30 · Owner: Tech lead

Rules an experienced engineer applies on a production system that handles money and personal data. Reviewers (human, or the `requesting-code-review` skill) check against this document. "MUST" items block merge.

## 0. Domain non-negotiables

1. **Money is a whole number of VND** (`int64` in Go, `number` limited to safe integers in TypeScript, `BIGINT` in SQL). No floats, no `NUMERIC` decimals. A `Vnd` value type carries arithmetic with overflow checks.
2. **Pricing is a pure function** of (rate plan, rental type, check-in, check-out). No I/O, no clock reads, no randomness. Every rule change updates `contracts/pricing/generate_vectors.py` first.
3. **Times are recorded by the server.** Clients never supply check-in, check-out or payment times.
4. **A transfer is PAID only through the payment-event handler.** No endpoint, UI action or admin path may mark a transfer paid by hand. The receiving account always comes from the tenant.
5. **Tenant isolation on every query.** Repositories take a tenant-scoped transaction; row-level security is the second guard, never the only one.
6. **Authorization on every request** by role and by building (docs/04 §2.3), checked in the application layer against the target resource's building.
7. **Idempotent writes** where the contract declares `Idempotency-Key`; events deduplicated by `(provider, external_id)`.
8. **Audit sensitive actions** (permission changes, shift close, cash payouts, alerts) in the same transaction as the change.
9. **Personal data minimisation:** ID numbers and bank account numbers are encrypted at rest, masked on read, never logged, never in fixtures or screenshots.
10. **Public repository hygiene** (docs/11 §6): no secrets, no real data, no customer names.

## 1. Principles

| Principle | What it means here | Example |
| --- | --- | --- |
| Single Responsibility | One reason to change per type or function | `domain/pricing` prices; it never reads the database |
| Open/Closed | Extend by adding | A new rental type is a new `Strategy`, registered once |
| Liskov Substitution | Implementations honour the port contract, including errors | The simulator and a real provider pass the same `PaymentSource` contract tests |
| Interface Segregation | Small, consumer-owned interfaces | `app/payments` needs `PaymentRepository` with four methods, not the whole store |
| Dependency Inversion | Domain and application depend on ports | Use cases take `Clock`, `IDGenerator`, `Repository` interfaces |
| KISS / YAGNI | Build what the story needs | No plugin system until the second vertical exists |
| DRY with rule of three | Duplicate once, abstract on the third | Two similar DTOs in different contexts stay separate |
| Fail fast | Validate config and input at the edge | Missing encryption key means the service does not start |
| Explicit over implicit | Constructor injection, typed config, no globals | No package-level mutable state |
| Illegal states unrepresentable | Types carry invariants | `Vnd`, `RentalType`, sealed result types; status transitions in one function |

## 2. Architecture rules

- Hexagonal layering in the API: `domain` → `app` (use cases and ports) → `adapter` (http, postgres, payments, clock, ids). Dependencies point inward only. Enforced by an import-boundary test and `depguard`.
- Domain code MUST NOT import chi, pgx, net/http or any framework, and MUST NOT perform I/O.
- One use case is one method taking a command or query struct; the transaction boundary lives in the use case.
- The web app talks to the API only through the generated client; no hand-written fetch calls to `/v1`.
- Every external dependency (database, clock, id generator, payment source, encryption) sits behind a port with a fake for tests.
- No shared libraries with business logic between `api/` and `web/`; the only shared artefacts are generated from `contracts/`.

## 3. Design patterns (use for the named problem)

| Pattern | Problem | Where |
| --- | --- | --- |
| Strategy | Interchangeable algorithms | Pricing per rental type; billing policy per vertical |
| State | Legal lifecycle transitions | Room status, stay, payment, shift |
| Value Object | Invariants on small values | `Vnd`, `RentalType`, `BillCode`, `AccessLevel` |
| Repository + Unit of Work | Persistence with transaction boundaries | `adapter/postgres` |
| Adapter / Port | Isolate frameworks and providers | `PaymentSource`, `Clock`, `Encryptor` |
| Idempotency Key | Safe retries | Command endpoints |
| Specification | Composable predicates | Access checks (role and building), payment matching |
| Observer / Pub-Sub | Fan-out of state changes | SSE hub per payment |
| Circuit breaker, retry with backoff and jitter | Only when a real remote provider is added | `adapter/payments` in production |

Anti-patterns that fail review: god services, anemic "manager" classes, boolean flag parameters that switch behaviour, `util` or `helpers` packages, deep inheritance, swallowed errors, stringly-typed ids and statuses, mutable singletons, business logic in handlers or components.

## 4. Code conventions

### 4.1 All languages

- Names come from the glossary in the root `CLAUDE.md` §8. Functions are verbs, booleans are predicates, collections are plural.
- Functions ≤ ~30 lines, ≤ 4 parameters (use a parameter object beyond that), cyclomatic complexity ≤ 10. Files ≤ ~300 lines, split by responsibility.
- No magic numbers or strings; constants with meaning (`GraceMinutes`, `ProblemRoomNotVacant`).
- Comments explain why (decision, contract reference, story id), never restate code. TODOs need an issue id.
- Exported Go identifiers and shared components have doc comments.
- Formatting is automatic (gofmt and goimports, Prettier); never argue style in review.

### 4.2 Go (API)

- Effective Go and Go Code Review Comments. Tooling: gofmt, goimports, golangci-lint (errcheck, govet, staticcheck, revive, gocyclo, depguard, gosec, bodyclose, contextcheck), govulncheck.
- `ctx context.Context` is the first parameter of anything that blocks; never store contexts in structs.
- Errors: wrap with `%w` and context; typed or sentinel errors; `errors.Is` and `errors.As`; never compare error strings; no `panic` outside `main`.
- Concurrency: every goroutine has an owner and a cancellation path; one closer per channel; shared state behind a mutex or confined to one goroutine; `goleak` in tests of long-running components (SSE hub, rate limiter).
- Constructors `NewX(deps) (*X, error)` validate dependencies; zero values are useful or impossible.
- Interfaces are small and defined at the consumer; accept interfaces, return structs.
- Package names are short and singular; none called `util`, `common` or `helpers`.
- Generated code (oapi-codegen, sqlc) is committed only if the pipeline says so in `api/CLAUDE.md`, never hand-edited.
- Table-driven tests; golden vectors read from `contracts/pricing/golden-cases.json`; property tests with a fixed seed logged on failure.

### 4.3 TypeScript and React (web)

- `strict`, `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`; no `any`; `unknown` plus Zod at trust boundaries (forms, storage).
- Server Components where useful at build time; anything needing browser state is a small client component at the leaf. No `useEffect` for data fetching; TanStack Query owns server state.
- Components are pure functions of props; side effects in hooks named `useX`; one component per file; named exports.
- Derived state is computed, not stored. Keys are stable ids, never array indexes for dynamic lists.
- All user-visible strings come from message files; no string concatenation for sentences; money and dates through `Intl` helpers.
- Styling with Tailwind and design tokens; no inline magic colours; motion only via motion tokens and disabled under reduced motion.
- Accessibility is part of done: semantic elements, labels, focus management, live region for payment status, text alternative to colour for every status.
- Routes use locale prefixes and query strings for entity ids (ADR-011).

### 4.4 SQL

- Explicit column lists; no `SELECT *` in application code. Parameterised queries only (sqlc).
- Lock intentionally (`FOR UPDATE`) and say why in a comment with the story id.
- Every hot-path query has its index listed in `docs/05` §6 and an EXPLAIN in the PR.

## 5. Error handling

- Classify errors: validation (client fault), business outcome (expected, for example a mismatched payment), transient (retryable), fatal (bug or configuration).
- Business outcomes are values, not exceptions, in the domain.
- One mapper in `adapter/http` converts errors to problem+json. Never leak stack traces, SQL or internal ids.
- Retry only transient errors, with backoff and jitter and a cap; never retry a non-idempotent operation without its idempotency key.
- Every caught error is either handled (with a log line that explains the decision) or wrapped and returned.
- The web app maps `code` to a translated message; unknown codes show a generic message and the trace id.

## 6. Observability

- Levels: ERROR needs human action; WARN degraded but handled; INFO business events (one line per command outcome); DEBUG diagnostics, off by default.
- Structured JSON with the standard fields in `docs/02` §7.6; messages are constant strings; data goes in fields.
- Never log ID numbers, bank account numbers, QR payloads, tokens or request bodies.
- Metrics named `sg_<noun>_<unit>`; label cardinality bounded (no ids as labels).

## 7. Security

- Secrets only from environment or secret files; `.env` is git-ignored; `.env.example` holds names, not values; gitleaks in CI and as a pre-commit hook.
- Validate all input at the edge (generated request validation plus domain checks); reject unknown JSON fields.
- Least privilege: the application database role has no `BYPASSRLS`, no superuser, no DDL; audit tables have no UPDATE or DELETE grants; a separate maintenance role handles trial cleanup.
- Dependencies pinned; weekly Dependabot or Renovate; no high or critical advisories at release; licences must be permissive.
- Constant-time comparison for tokens and signatures; sessions stored hashed; cookies (if used) `HttpOnly`, `Secure`, `SameSite`.
- Security headers on the served web app (CSP, HSTS, `X-Content-Type-Options`, `Referrer-Policy`).

## 8. Concurrency and consistency

- One database transaction per command; no network calls inside a transaction.
- Lock the room row for check-in and the invoice row for settlement; test both with concurrent requests.
- Timeouts on every I/O; no unbounded waits; bounded queues and pools with metrics.
- Graceful shutdown: stop accepting, drain in-flight requests and SSE streams within the window, close resources.

## 9. Performance

Measure before optimising; a performance change carries a benchmark or measurement in the PR. No N+1 queries. Web: route-level code splitting, initial JS budget in NFR-05, lazy-load the QR library.

## 10. Testing

See `08-test-strategy.md`. Arrange-act-assert; one behaviour per test; fakes over mocks; no test depends on another test's state; the clock is injected; test names end with the story and criterion id, for example `..._SG101_AC2`.

## 11. Git and pull requests

- Conventional Commits (`feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `perf`, `build`, `ci`) with scope = lane (`api`, `web`, `plat`, `contracts`) and the story id.
- One story per PR; ≤ 400 changed lines excluding generated files; draft PR early for larger stories.
- PR description: what and why, AC to test mapping, screenshots for UI in both languages, risk and rollback note, docs updated.
- Never force-push to `main`; squash merge only.

## 12. Code review checklist

- [ ] Each acceptance criterion is proven by a named test that fails without the change
- [ ] Layering respected; no framework in domain; adapters do not call each other
- [ ] Domain non-negotiables (§0) honoured
- [ ] Authorization and tenant filters present on every new path
- [ ] Errors classified and mapped in the single mapper; none swallowed
- [ ] Concurrency: ownership, cancellation, locking, timeouts
- [ ] Logs and metrics added; no personal data
- [ ] Contract respected; generated code untouched; migrations forward-only with RLS
- [ ] Strings in message files for both languages; accessibility checked
- [ ] Names, function size, comments explain why
- [ ] `docs/progress.md`, `docs/release-notes.md`, ADR and bug log updated as required by docs/11

## 13. Documentation

ADRs for any hard-to-reverse or cross-lane decision. Each service README: purpose, run, config table, ports, troubleshooting. Runbooks for alerts (`docs/runbooks/`, added when production alerts exist). Plans in `docs/plans/` are kept after merge as a learning record.
