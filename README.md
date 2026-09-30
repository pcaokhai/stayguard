# StayGuard

Anti-loss management for small guesthouses: automatic pricing by hour, night and day, QR payments that always reach the owner's account, per-building staff permissions, and shift cash reconciliation.

Status: **pre-implementation**. The specification pack in this repository is complete; code is built story by story (see `docs/progress.md`).

> The demo is for trying the product. Do not enter real guest data.

## Start here

- People: `docs/README.md` (reading order per role)
- Claude Code sessions: `CLAUDE.md`, then `api/CLAUDE.md` or `web/CLAUDE.md`
- Live status: `docs/progress.md`; release history: `docs/release-notes.md`

## Layout

`api/` Go backend · `web/` Next.js app · `contracts/` OpenAPI, event schemas, pricing vectors, fixtures · `docs/` specification, plans, ADRs, bug log · `.github/` CI and templates. Deployment files arrive with story SG-603.

## Licence

Apache-2.0 (see ADR-002). Added by story SG-001.
