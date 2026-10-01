-- +goose Up
-- SG-102: per-user UI language, and the pre-tenant session lookup of ADR-015.
ALTER TABLE app.users ADD COLUMN locale text NOT NULL DEFAULT 'vi' CHECK (locale IN ('vi', 'en'));

-- The token hash being resolved, transaction-local like app.tenant_id. Unset or empty gives NULL,
-- which equals nothing, so the policy below returns zero rows (fail closed).
CREATE FUNCTION app.session_lookup_hash() RETURNS text
    LANGUAGE sql STABLE
    AS $$ SELECT nullif(current_setting('app.session_hash', true), '') $$;

-- A request carries only a bearer token, so the tenant is unknown until the session is read.
-- SELECT only, one row (the holder of the 256-bit hash), and only while no tenant is set; INSERT,
-- UPDATE and DELETE still need the tenant policy (ADR-015).
CREATE POLICY sessions_lookup ON app.sessions FOR SELECT
    USING (token_hash = app.session_lookup_hash() AND app.current_tenant() IS NULL);
