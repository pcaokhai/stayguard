# ADR-005 Multi-tenancy with a shared schema, tenant_id and PostgreSQL row-level security

- Status: Accepted
- Date: 2026-09-30
- Deciders: Tech lead
- Related: docs/05, SG-003, SG-604

## Context
Every prospect gets a private trial tenant; production will host several guesthouses. A leak between tenants would end the product.

## Options considered
1. Database per tenant
2. Schema per tenant
3. Shared schema with tenant_id and application filters only
4. Shared schema with tenant_id plus row-level security

## Decision
Shared schema; `tenant_id` on every table; composite foreign keys; RLS policies on every table; the application role has no BYPASSRLS; the tenant is set per transaction; a maintenance role handles trial cleanup. Application queries also filter by tenant explicitly.

## Consequences
- Good: Two independent guards; cheap trial creation; one migration path
- Bad or costly: Every query path needs the tenant set; RLS mistakes fail closed (empty results) and need tests; operational tooling must use the maintenance role
- Revisit when: A customer requires physical isolation or the data volume of one tenant dominates.
