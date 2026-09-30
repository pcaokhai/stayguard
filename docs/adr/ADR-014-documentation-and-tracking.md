# ADR-014 Documentation and tracking system

- Status: Accepted
- Date: 2026-09-30
- Deciders: Tech lead
- Related: docs/11, docs/07 §7

## Context
AI lanes produce a lot of change; decisions, progress, releases and bugs must live in the repository, not in chat.

## Options considered
1. Issue tracker only
2. Documents in the repository updated in the same PR
3. Wiki

## Decision
Tracking files in the repository: `docs/progress.md`, `docs/release-notes.md`, `docs/adr/`, `docs/bugs/`, `docs/plans/`. Rules and formats are in docs/11. Plans contain no code. Updating the files is part of the Definition of Done.

## Consequences
- Good: Everything is versioned, reviewable and visible to future sessions; no tool lock-in
- Bad or costly: Small overhead per story; discipline needed to keep files short
- Revisit when: The overhead exceeds its value or the team grows and needs an issue tracker on top.
