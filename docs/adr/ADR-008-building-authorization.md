# ADR-008 Building-level authorization checked in the application layer on every request

- Status: Accepted
- Date: 2026-09-30
- Deciders: Tech lead
- Related: docs/04 §2.3, SG-501

## Context
The owner grants none, view or edit per staff member per building and expects changes to apply at once.

## Options considered
1. Permissions embedded in the token
2. Look up permissions on every request
3. Database policies only

## Decision
Permissions live in `building_permissions` and are read on every request by an `Authorizer` that maps the target resource to its building. Owner has implicit edit everywhere. A table test covers every operation, role and level.

## Consequences
- Good: Revocation is immediate; one place to test the matrix
- Bad or costly: One extra indexed lookup per request, cacheable per request
- Revisit when: Lookup cost becomes visible in latency budgets (docs/08 §3).
