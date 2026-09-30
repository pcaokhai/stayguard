# Product Requirements Document — StayGuard

Version 1.0 · 2026-09-30 · Owner: Tech lead

StayGuard is the working title. Rename freely; story IDs use the key `SG`.

## 1. Summary

StayGuard is a web application that lets small guesthouse and motel owners run the front desk from a phone or a computer while making it hard for staff to hide revenue. It prices every stay automatically by hour, night or day, forces bank-transfer payments into the owner's own account, and records every edit. This repository delivers a **demo** that a prospect can try in about three minutes, and it is built to production standards so it becomes the core of the full product.

## 2. Contacts

| Name / role | Responsibility | Comment |
| --- | --- | --- |
| Tech lead (human) | Product decisions, contract PRs, every code review, releases | Bottleneck by design; see docs/07 §2 |
| Lane API (Claude Code session) | Go backend, migrations, `api/` | Works in its own worktree per story |
| Lane WEB (Claude Code session) | Next.js app, `web/` | Builds on generated mocks until the API merges |
| Lane PLAT (Claude Code session, tech lead reviews) | `contracts/`, CI, packaging, `deploy/`, `.github/` | Owns shared files |

## 3. Background

Small guesthouses in Vietnam price by hour, overnight and day, take much of their revenue in cash or by bank transfer, and track it in paper notebooks. A prospective customer described the core problem in the lead conversation that started this project: staff record a stay but ask guests to transfer money to the staff member's own account, shorten stay times, skip the notebook entirely, and under-report drinks. Off-the-shelf hotel software exists and is cheap per month, but it is built for many kinds of properties and rarely locks down these specific behaviours. Two things make a focused product feasible now: bank transfers can be confirmed automatically through reconciliation providers, and AI-assisted development makes a small, well-specified product cheap to build and maintain.

## 4. Objective

| Objective | Key results (all measurable) |
| --- | --- |
| Correct money | 25 of 25 golden pricing cases pass in Go and in the reference generator; property tests find no counter-example in 10,000 random stays; zero floating-point money anywhere |
| Believable control | A trial user sees all four loss controls work: QR payment into the owner's account, server-recorded times, cash reconciliation, per-building permissions; 8 of 8 walkthrough steps complete without help |
| Engineering quality | Tenant-isolation suite covers every endpoint with two tenants and finds zero cross-tenant reads; coverage gates in docs/08 §3 hold; no critical vulnerability at release |
| Usability | Check-in to paid takes ≤ 8 taps on a phone; every screen usable in Vietnamese and English; axe reports zero serious violations |
| Business and personal | Public demo link and 2-minute video ready at v0.1.0; codebase reusable for a second customer type (boarding house) by adding one billing policy and its screens, without touching the payment core |

## 5. Market segments

Defined by the job and the constraint, not by demographics.

| Segment | Job to be done | Constraint |
| --- | --- | --- |
| Owner-operator of a 10 to 60 room guesthouse or motel that staff run day to day | Know that every stay, every dong and every drink is recorded | Not present at the front desk; staff turnover; low tolerance for software complexity |
| Front-desk staff on shared phones and a counter PC | Check guests in and out fast without arithmetic mistakes | Poor connectivity at times; limited training time |
| Housekeeping staff | See which rooms to clean and report a room that looks used | One-handed phone use |
| Secondary, later: boarding-house (nhà trọ) owners | Bill monthly rent, meters and services | Different billing rhythm, same trust problem |

## 6. Value propositions

| Job | Gain | Pain avoided | Why better than alternatives |
| --- | --- | --- | --- |
| Collect transfers safely | QR always pays the owner's account; a bill is paid only when the money arrives | Staff routing transfers to personal accounts | Most alternatives let staff mark a bill paid by hand |
| Price stays correctly | Hour, night and day rules with grace and caps applied by the server | Arguments and mental arithmetic | Rules are configuration, so each guesthouse's habits fit |
| Control cash | Shift close counts cash by note value and demands a reason for any gap | Silent shortages | Owner sees who is repeatedly short |
| Limit what staff can touch | Owner grants view or edit per building per person | One person changing the other building's rooms | Enforced by the server on every request |
| Own the data | Data belongs to the customer and can be exported at any time | Lock-in and per-room price growth | Pricing is not tied to room count |

## 7. Solution

### 7.1 Experience

Screen designs are on the design canvas (link in `docs/README.md`), in Vietnamese and English, for phone and desktop. This document does not re-describe screens; `docs/06` cites screens by number.

### 7.2 Key features

| Feature | Description | Epic |
| --- | --- | --- |
| Trial sessions and role picker | Pick Front desk, Owner or Housekeeping; each trial gets its own sample data for 24 hours | E1, E6 |
| Room map | Buildings, floors, colour-coded status, live elapsed time, view-only mode | E2 |
| Check-in and stay details | Three rental types, deposit, server-recorded time, running total | E2 |
| Extras and check-out | Drinks and items with stock, itemised bill, deposit handling | E2 |
| Payments | Cash, or QR for the exact amount into the owner's account; confirmed by server events | E3 |
| Housekeeping | Rooms to clean, mark clean, report a room that looks used | E4 |
| Owner overview | Revenue, transfers received, cash expected, alerts, latest payments | E4 |
| Building permissions | Owner grants none, view or edit per staff member per building | E5 |
| Shift close | Count cash by note value, reason required on a gap, owner review | E5 |
| Vietnamese and English | Locale-prefixed routes, coded API errors | E0, E6 |

### 7.3 Technology

Go modular monolith, PostgreSQL with row-level security, Next.js static export embedded in one container, contract-first OpenAPI. See `docs/02` and the ADRs.

### 7.4 Assumptions

| ID | Assumption | How validated | By |
| --- | --- | --- | --- |
| A-01 | The demo rate plan and 15-minute grace match a typical guesthouse | Ask the lead customer for their real price list and habits; adjust the rate-plan fixture | Before SG-101 merges if available, else at v0.1.0 review |
| A-02 | Deposits are taken in cash at check-in | Confirm with the lead customer; if transfers are common, add a transfer deposit later | SG-503 review |
| A-03 | Owners will accept "server confirms the transfer" once a bank-reconciliation provider is connected | Interview two owners with the demo | After v0.1.0 |
| A-04 | A VietQR payload built in the API scans correctly in common banking apps | Manual scan with two bank apps against a real account of the tech lead | SG-301 gate |
| A-05 | Two buildings share one front desk; a receptionist assigned to building A does not take payment for building B | Confirm with the lead customer | SG-501 review |
| A-06 | Next.js static export plus locale-prefixed routes is enough for a logged-in app with no SEO need | Spike inside SG-004; ADR-011 records the outcome | SG-004 |
| A-07 | Sharing the code publicly does not hurt the future commercial product | Licence and repository split decided in ADR-002; revisit before the public push | v0.1.0 |
| A-08 | Neon cold start plus Cloud Run cold start is acceptable for self-testing, not for prospects | Measure in SG-603; prospects use the VPS | SG-603 |

## 8. Release

| Release | Content | When |
| --- | --- | --- |
| v0.0.1 (internal) | Repo, CI, contracts pipeline, migrations, RLS, pricing engine, sessions, web shell | End of Sprint 0 |
| v0.0.2 (internal) | Room map, check-in, extras, check-out, payment creation with QR | End of Sprint 1 |
| v0.0.3 (internal preview) | Payment confirmation and SSE, housekeeping, owner overview, permissions, deployment packaging | End of Sprint 2 |
| v0.1.0 (public demo) | Trial tenants, role picker with language switch, shift close and reconciliation, E2E journeys, video, public repository | End of Sprint 3 |

Out of scope for the demo, tracked for the full product: real bank webhooks and provider integration, editing a stay's times with a reason (alert kind exists), stock in/out and stocktake, monthly reports, advance bookings and OTA sync, hardware door locks, e-invoices, automatic stay declaration to authorities, ID-card scanning, boarding-house billing (rent, meters), multi-property owner dashboards, real user accounts and password or PIN sign-in, audit-log viewer screen.
