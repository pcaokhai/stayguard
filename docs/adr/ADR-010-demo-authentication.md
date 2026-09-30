# ADR-010 Demo authentication through role-picker trial sessions, production credentials later

- Status: Accepted
- Date: 2026-09-30
- Deciders: Tech lead
- Related: docs/04, SG-102, SG-601

## Context
A prospect must try the product in seconds without an account.

## Options considered
1. Real accounts for the demo
2. Shared demo login
3. Anonymous role-picker sessions on per-trial tenants

## Decision
`POST /v1/demo/sessions` (DEMO_MODE only) creates an opaque session for a chosen role. Tokens are random, stored hashed, short-lived. Production adds real users with PIN or password sign-in behind the same session and `Authorizer` code; the demo endpoint is not exposed there.

## Consequences
- Good: Zero friction; identical authorization path in demo and production
- Bad or costly: Session issuance differs per environment and must be tested in both; demo endpoint needs abuse limits
- Revisit when: Production sign-in design (PIN, password, single sign-on) is decided.
