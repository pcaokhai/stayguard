# Progress

Live status. Protocol: docs/11 §7. Update one row at a time; ship edits in the story's PR (`docs(progress): SG-xxx → <status>`).

## Current sprint

Sprint: 0 (not started) · Dates: <fill at sprint start> · Committed: 22 pts of 25 · Release target: v0.0.1

## Sprints

| Sprint | Goal | Planned pts | Done pts | Release | Status |
| --- | --- | --- | --- | --- | --- |
| 0 | Foundations and the pricing core | 22 | 0 | v0.0.1 | Not started |
| 1 | Front-desk loop up to payment creation | 19 | 0 | v0.0.2 | Not started |
| 2 | Payment confirmation, housekeeping, owner, permissions, packaging | 19 | 0 | v0.0.3 | Not started |
| 3 | Shifts, trials, end-to-end, public release | 13 | 0 | v0.1.0 | Not started |

## Stories

Status values: Backlog, Ready, In progress, In review, Merged (flag off), Released.

| ID | Title | Lane | Pts | Sprint | Status | Branch or PR | Flag | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| SG-001 | Repository scaffold, tooling and CI | PLAT | 3 | 0 | Merged (flag off) | PR #1, 69cf067 | | AC4 CI proof: run 36802418823 |
| SG-002 | Contract pipeline | PLAT | 3 | 0 | Merged (flag off) | PR #7, 09be60f | | R-14: 3.1 kept, orval for mocks; AC4 CI proof: run 36813246897; PR size above 400 accepted by tech lead; follow-up contracts PR: x-story on POST /v1/webhooks/bank; MSW worker not yet checked in a real browser |
| SG-003 | Database foundation | API | 3 | 0 | Merged (flag off) | PR #10, 1cf214b | none | CI green incl. -race; PR size above 400 accepted by tech lead; owned follow-ups in plan Ruling 11 |
| SG-004 | Web foundation | WEB | 2 | 0 | Backlog | | | Static-export i18n spike (A-06) |
| SG-101 | Pricing engine | API | 5 | 0 | Merged (flag off) | PR #18 (3e3a60c), PR #19 (026dcd0) | none | Two PRs, both over 400 lines; 25/25 golden + 2 errors + 2 bills; manual mutation review, no survivors; A-01 still open (rates are plan data); follow-ups in plan Rulings 5 and 7 |
| SG-102 | Sessions, identity and tenant context | API | 3 | 0 | Merged (flag off) | PR #12, 828feca | none | CI green; PR size above 400 (one PR chosen by tech lead); DEMO_MODE off in public deploys until SG-601; follow-ups in plan Rulings 7 and 8 |
| SG-202 | Room map screen | WEB | 3 | 0 | Backlog | | FF_S1_ROOM_MAP | |
| SG-201 | Rooms and buildings read API | API | 2 | 1 | Merged (flag off) | PR #15 (b4086ac), PR #16 (8143aff) | FF_S1_ROOM_MAP | Two PRs chosen by tech lead; both over 400 lines; pricing bound to the quoter (SG-101 engine); flag can be turned on after the S1 integration checkpoint (docs/07); AC5 p95 about 6.4 ms; follow-ups in plan Rulings 7 and 8 |
| SG-203 | Check-in API | API | 3 | 1 | Backlog | | FF_S2_CHECKIN | |
| SG-204 | Check-in screen | WEB | 2 | 1 | Backlog | | FF_S2_CHECKIN | |
| SG-205 | Extras and check-out API | API | 3 | 1 | Backlog | | FF_S3_CHECKOUT | |
| SG-206 | Stay details, extras sheet and check-out screens | WEB | 3 | 1 | Backlog | | FF_S3_CHECKOUT | |
| SG-301 | Payment creation and QR payload | API | 3 | 1 | Backlog | | FF_S4_PAYMENTS | QR scan check in two bank apps (A-04) |
| SG-303 | QR and paid screens | WEB | 3 | 1 | Backlog | | FF_S4_PAYMENTS | |
| SG-302 | Payment event handler, simulator and SSE | API | 3 | 2 | Backlog | | FF_S4_PAYMENTS | |
| SG-401 | Housekeeping API | API | 2 | 2 | Backlog | | FF_S5_HOUSEKEEPING | |
| SG-402 | Housekeeping screen | WEB | 2 | 2 | Backlog | | FF_S5_HOUSEKEEPING | |
| SG-403 | Owner overview API | API | 2 | 2 | Backlog | | FF_S6_OWNER | |
| SG-404 | Owner overview screen | WEB | 2 | 2 | Backlog | | FF_S6_OWNER | |
| SG-501 | Building permissions API | API | 3 | 2 | Backlog | | FF_S7_PERMISSIONS | |
| SG-502 | Staff permissions screen | WEB | 2 | 2 | Backlog | | FF_S7_PERMISSIONS | |
| SG-603 | Deployment packaging | PLAT | 3 | 2 | Backlog | | | Measure cold start (A-08) |
| SG-503 | Shift close and cash reconciliation API | API | 3 | 3 | Backlog | | FF_S8_SHIFTS | |
| SG-504 | Close-shift and shift-review screens | WEB | 3 | 3 | Backlog | | FF_S8_SHIFTS | |
| SG-601 | Trial tenants | API | 3 | 3 | Backlog | | FF_S9_TRIALS | |
| SG-602 | Role picker and language switch | WEB | 2 | 3 | Backlog | | FF_S9_TRIALS | |
| SG-604 | End-to-end journeys and isolation suite | PLAT | 2 | 3 | Backlog | | | Public checklist run (docs/11 §6) |

## Blockers

None yet.

## Open assumptions to confirm

A-01 rate plan and grace, A-02 cash deposits, A-03 owner acceptance of server-confirmed transfers, A-04 QR scan, A-05 two buildings one desk, A-07 licence, A-08 cold start. Owners and dates are in docs/01 §7.4.

## Retro notes

Add one short block per finished sprint: what went well, what did not, one change to try. Then collapse the sprint's details into a single line.

## Metrics (docs/07 §11)

Cross-lane conflicts: 0 · WEB days blocked by API: 0 · Review queue age: n/a · Bugs logged / fixed: 0 / 0
