# Risk Register (Pre-mortem)

Version 1.0 · 2026-09-30 · Owner: Tech lead

It is the end of Sprint 3 and the demo failed. What happened?

## Tigers

| ID | Risk | Urgency | Mitigation | Owner | Check by |
| --- | --- | --- | --- | --- | --- |
| R-01 | Review bottleneck: three lanes produce more than one person can review; quality drops silently | Launch-blocking | Commit ≤ 25 pts per sprint; PRs < 400 lines; review queue first each day; no story starts without ready acceptance criteria | Tech lead | Every sprint end |
| R-02 | Contract drift between mocked frontend and real API | Launch-blocking | One OpenAPI source; provider tests validate real responses; integration checkpoint per slice with mocks off | PLAT | Each slice |
| R-03 | Pricing rules differ from real guesthouse habits, so prospects lose trust | Launch-blocking | Rules are configuration; ask the lead customer for their price list and grace habits (A-01); golden vectors change with the generator only | Tech lead | Before SG-101 merges, again at v0.1.0 |
| R-04 | Tenant isolation leak (RLS misconfigured, query without tenant) | Launch-blocking | Deny-by-default RLS with a test that every table has a policy; isolation suite over every operation; application role without BYPASSRLS | API | SG-003, SG-604 |
| R-05 | Secret, personal data or customer name reaches the public repository | Launch-blocking | gitleaks on full history in CI and pre-commit; fixtures reviewed; the repository stays private until the v0.1.0 checklist (docs/11 §6) passes | PLAT | SG-001, v0.1.0 |
| R-06 | VietQR payload does not scan correctly in real banking apps | Launch-blocking | Structure and CRC unit tests; manual scan in two bank apps with a real account of the tech lead before merge (A-04) | API | SG-301 |
| R-07 | Static export limits (dynamic routes, locale routing) force a rework of the web app | Fast-follow | Spike in SG-004; entity ids in query strings; ADR-011 records the trigger to move to a server runtime | WEB | SG-004 |
| R-08 | Cold starts on free hosting make the first impression slow | Fast-follow | Self-test on free tiers only; prospects use the VPS; measure in SG-603 (A-08) | PLAT | SG-603 |
| R-09 | AI-written code the owner cannot explain or maintain | Launch-blocking | Plans in words, tests first, one story per PR, ADRs for decisions, owner reads every diff; bug log records root causes | Tech lead | Continuous |
| R-10 | Trial abuse: spam creates many tenants or floods payments | Fast-follow | Per-IP limit, global active-trial cap, lazy cleanup; simulator gated by `DEMO_MODE` | API | SG-601 |
| R-11 | Scope creep from customer conversations pulls production features into the demo | Fast-follow | Out-of-scope list in PRD §8; new requests go to the backlog section of docs/06 with a note in docs/progress.md | Tech lead | Every sprint start |
| R-12 | Flaky end-to-end tests erode trust in CI | Track | Deterministic clock, fixtures, retries only at network level, quarantine list reviewed weekly | PLAT | SG-604 |
| R-13 | Personal-data and stay-declaration obligations differ from what the demo assumes | Track | No real guest data in the demo; check current Vietnamese rules before any production customer (docs/02 §7.1) | Tech lead | Before production |
| R-14 | Code generators do not fully support OpenAPI 3.1 (Go generator support is recent and described as initial) | Fast-follow | Spike in SG-002; fallback is a 3.0.3 contract using `nullable`; ADR-003 records the outcome | PLAT | SG-002 |

## Paper tigers

| Concern | Why it is not real |
| --- | --- |
| Row-level security will be slow | At tens of thousands of rows per tenant the indexes in docs/05 §6 dominate; measured in SG-201 |
| Go plus Next.js is too much for one person | One container, generated clients and one contract keep the surface small |
| Two languages slow the team | Message keys from day one and a CI check make it a constant small cost |
| Free hosting may disappear | Only self-test uses it; the image runs anywhere with Docker |

## Elephants

| Concern | Investigation |
| --- | --- |
| Will owners choose a custom product over cheap monthly software? | After v0.1.0 show the demo to three owners; ask what would make them switch; record answers in docs/progress.md |
| Can one person maintain public and private repositories over years? | Review the sync burden after two production features (ADR-002 trigger) |
| Is the "owner can see it all" promise credible when the operator hosts the data? | Contract wording and an access log for operator access; see docs/02 §7.1 |

## Action plans for launch-blocking tigers

| Risk | Action | Owner | Due |
| --- | --- | --- | --- |
| R-01 | Enforce capacity and PR size in `docs/progress.md` sprint plan | Tech lead | Sprint 0 |
| R-02 | Provider contract tests in CI; integration checkpoint prompt in docs/07 §10 | PLAT | Sprint 1 |
| R-03 | Collect the lead customer's real price list and grace rules | Tech lead | Sprint 0 |
| R-04 | RLS policy test and isolation suite | API | Sprint 0 and 3 |
| R-05 | gitleaks full-history scan and public-repository checklist | PLAT | Sprint 0 and 3 |
| R-06 | Manual QR scan record in the SG-301 PR | API | Sprint 1 |
| R-09 | Enforce docs/11 rules (plans in words, bug log, ADR) from the first story | Tech lead | Sprint 0 |
