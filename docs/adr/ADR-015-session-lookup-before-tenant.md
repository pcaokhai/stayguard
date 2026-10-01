# ADR-015 Session lookup by token hash before the tenant is known

- Status: Accepted
- Date: 2026-10-01
- Deciders: Tech lead
- Related: ADR-005, ADR-010, SG-003 Ruling 11, SG-102

## Context
A request carries only a bearer token. The tenant is stored in `sessions`, which is under forced RLS, so the application role sees zero rows until a tenant is set (fail closed). The lookup needs a narrow, auditable exception that stays closed for every other path. `FORCE ROW LEVEL SECURITY` also binds the table owner, so a plain `SECURITY DEFINER` function owned by the migration owner would still see nothing.

## Options considered
1. `SECURITY DEFINER` function (fixed `search_path`, hash-only argument), owned by a new NOLOGIN role that holds a SELECT-only policy on `sessions` (BYPASSRLS on the owner would be broader and is rejected). Good: the app role gets no table access beyond the function. Costs: a new role (managed hosts may restrict role creation, SG-003 Ruling 3), function owner outside the startup privilege check, extra grants.
2. Dedicated lookup role and second pool. Costs: second credential and connection, and the whole table is readable through that connection.
3. A SELECT-only policy on `sessions` that matches the row whose `token_hash` equals a transaction-local setting `app.session_hash`, and only while no tenant is set. No new role, no function, no second pool.

## Decision
Option 3 (recommended). Policy `sessions_lookup` is `FOR SELECT` and its predicate is `token_hash = app.session_lookup_hash() AND app.current_tenant() IS NULL`; both helpers read settings with `missing_ok`, so an unset setting yields NULL and no row. The adapter resolves a token in its own short transaction (set hash, select tenant, user, expiry), then every later query runs in the normal tenant transaction. INSERT, UPDATE and DELETE are untouched: they still require the tenant policy. A row is reachable only by someone who already holds its hash, which is the same capability option 1 would give. Option 1 is the fallback if the tech lead wants the app role to have no direct read path at all.

## Consequences
- Good: one migration, no new role or credential; fail-closed when the setting is absent; TS-08 stays green and gains a hash-lookup case; works on managed hosts.
- Bad or costly: a code path that sets the setting can read one session row; the guard is the 256-bit hash and the small adapter that is the only caller. The RLS catalog check needs no exception because the predicate names `app.current_tenant()`.
- Revisit when: sessions move to a cache or a signed token format, or a second table needs a pre-tenant lookup.
