# Documentation Index

Everything needed to start Sprint 0 is here. Nothing lives only in chat.

## Documents

| # | Document | Purpose | Primary audience | Status |
| --- | --- | --- | --- | --- |
| 01 | [Product requirements](01-prd.md) | Why, for whom, objectives, releases, assumptions | Everyone | v1.0 |
| 02 | [Software architecture](02-software-architecture.md) | Drivers, C4, runtime views, cross-cutting rules, stack | Tech lead, lanes | v1.0 |
| 04 | [API contract](04-api-contract.md) | Conventions, errors, authorization matrix, catalogue | API and WEB lanes | v1.0 |
| 05 | [Data model](05-data-model.md) | Tables, modelling and migration rules, indexes | API lane | v1.0 |
| 06 | [User stories](06-user-stories.md) | 27 stories with numbered acceptance criteria | Everyone | v1.0 |
| 07 | [Delivery plan](07-delivery-plan.md) | Lanes, slices, sprints, capacity, playbook | Tech lead, lanes | v1.0 |
| 08 | [Test strategy](08-test-strategy.md) | Pyramid, gates, journeys, scenarios | All lanes, QA | v1.0 |
| 09 | [Risk register](09-risk-register.md) | Pre-mortem and actions | Tech lead | v1.0 |
| 10 | [Engineering standards](10-engineering-standards.md) | Non-negotiables, patterns, conventions, review checklist | All lanes | v1.0 |
| 11 | [AI workflow and tracking](11-ai-workflow-and-tracking.md) | Superpowers, token discipline, plans, bug log, public repository rules | All lanes | v1.0 |
| | [ADRs](adr/README.md) | Architecture decisions | Everyone | 14 accepted |
| | [Progress](progress.md), [Release notes](release-notes.md), [Bug log](bugs/README.md), [Plans](plans/README.md) | Live tracking | Everyone | Living |

Document 03 (interface specification) is intentionally absent: there is no internal protocol beyond REST and SSE. Numbering stays stable.

## Reading order

- Day 1, everyone: 01 §1–4, 02 §1–4, 07 §1–4, then `CLAUDE.md`.
- API lane: 02 §5–7, 04, 05, 10 §4.2, `api/CLAUDE.md`, contracts.
- WEB lane: 02 §7.9, 04, 10 §4.3, `web/CLAUDE.md`, the design references below.
- PLAT: 07, 08 §3, 09, 11.
- Reviewer and QA: 06, 08, 10 §12.

## Design references

Screen designs (Vietnamese and English, phone and desktop) live on the design canvas: https://claude.ai/artifact/9iqS9mqkCH4MyETQsADLAv. The earlier bilingual planning document is at https://claude.ai/code/artifact/6d2dba08-646e-475b-8f81-0e130ca964f4 and is superseded where docs/02 §12 lists changes. Both links are private; before the public push replace them with exported images or PDFs in `docs/assets/` (docs/11 §6).

## Conventions

- Language: documents are in English; the UI is Vietnamese and English.
- Formats: money in whole VND written `40,000₫` in English text and `40.000đ` in Vietnamese; dates ISO 8601 in documents; tenant time zone Asia/Ho_Chi_Minh for the demo.
- IDs: stories `SG-<epic><nn>`, criteria `<story>-AC<n>`, requirements `FR-nn` and `NFR-nn`, scenarios `TS-nn`, risks `R-nn`, assumptions `A-nn`, ADRs `ADR-nnn`, bugs `BUG-nnn`, pricing cases `PRC-xxx`.
- The words MUST, MUST NOT and SHOULD carry their RFC 2119 meaning.
- Change control: documents change in the same PR as the behaviour they describe. Accepted ADRs are superseded, not edited. Versions are bumped on structural change only.
