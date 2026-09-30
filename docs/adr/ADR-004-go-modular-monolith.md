# ADR-004 Go modular monolith with hexagonal layering and one deployable

- Status: Accepted
- Date: 2026-09-30
- Deciders: Tech lead
- Related: docs/02 §5, docs/10 §2

## Context
Small team, low hosting budget, strong need for testable business rules, possible later extraction of parts.

## Options considered
1. Go modular monolith
2. Microservices
3. Node.js (NestJS) monolith

## Decision
One Go binary with packages `domain`, `app`, `adapter`, `platform`, dependencies pointing inward, enforced by an import-boundary test. The web export is embedded in the same binary. Internal domain events are in-process.

## Consequences
- Good: Simple deployment and debugging; pure domain code is fast to test; clear seams (ports) for providers
- Bad or costly: No independent scaling of parts; discipline needed to keep boundaries
- Revisit when: A component needs separate scaling or a separate team; extract along the existing port boundaries.
