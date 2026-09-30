# ADR-001 Repository layout and lane ownership

- Status: Accepted
- Date: 2026-09-30
- Deciders: Tech lead
- Related: ADR-003, docs/07

## Context
One human reviews the work of several Claude Code lanes. Lanes need directories they own so they can work at the same time without conflicts.

## Options considered
1. Monorepo with `api/`, `web/`, `contracts/`, `docs/`, `deploy/`
2. Separate repositories per service plus a contracts repository
3. Monorepo managed with a workspace tool

## Decision
Single monorepo. `api/` (lane API), `web/` (lane WEB), and `contracts/`, `.github/`, `deploy/`, root files (lane PLAT). Ownership is encoded in `.github/CODEOWNERS` and in each story's Lane field. No workspace tool in the demo.

## Consequences
- Good: Contract changes are atomic with their consumers; one CI; simple onboarding
- Bad or costly: Large repository history mixes concerns; lane discipline is enforced by convention and review
- Revisit when: A second team or a separate release cadence for the web app.
