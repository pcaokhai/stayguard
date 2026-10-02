-- +goose Up
-- L-A1: PIN sign-in. Users gain a sign-in name, app access and status; PINs live in their own table.

ALTER TABLE app.users DROP CONSTRAINT users_role_check;
ALTER TABLE app.users ADD CONSTRAINT users_role_check CHECK (role IN ('OWNER', 'MANAGER', 'RECEPTIONIST', 'HOUSEKEEPING'));

-- NONE = no sign-in (roster and payroll only). Fails closed: a user inserted without it cannot sign in.
ALTER TABLE app.users
    ADD COLUMN app_access text NOT NULL DEFAULT 'NONE'
        CHECK (app_access IN ('NONE', 'OWNER', 'MANAGER', 'RECEPTIONIST', 'HOUSEKEEPING')),
    ADD COLUMN status text NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'LOCKED', 'REMOVED')),
    ADD COLUMN username text CHECK (username ~ '^[a-z0-9._-]{1,32}$');
UPDATE app.users SET app_access = role;
CREATE UNIQUE INDEX users_username ON app.users (tenant_id, username) WHERE username IS NOT NULL;

ALTER TABLE app.tenants ADD COLUMN guesthouse_code text CHECK (guesthouse_code ~ '^[a-z0-9-]{3,16}$');
CREATE UNIQUE INDEX tenants_guesthouse_code ON app.tenants (guesthouse_code) WHERE guesthouse_code IS NOT NULL;

-- The code being signed in with, transaction-local like app.tenant_id. Unset or empty gives NULL (fail closed).
CREATE FUNCTION app.signin_code() RETURNS text
    LANGUAGE sql STABLE
    AS $$ SELECT nullif(current_setting('app.signin_code', true), '') $$;

-- Sign-in knows only a guesthouse code, so the tenant is unknown until it is read: SELECT only, the one
-- tenant holding that code, and only while no tenant is set (same pattern as sessions_lookup, ADR-015).
CREATE POLICY tenants_signin_lookup ON app.tenants FOR SELECT
    USING (guesthouse_code = app.signin_code() AND app.current_tenant() IS NULL);

CREATE TABLE app.pin_credentials (
    tenant_id         text NOT NULL,
    user_id           text NOT NULL,
    pin_hash          text NOT NULL,
    failed_count      integer NOT NULL DEFAULT 0 CHECK (failed_count >= 0),
    first_failed_at   timestamptz,
    locked_until      timestamptz,
    must_change       boolean NOT NULL DEFAULT false,
    one_time_expires_at timestamptz,
    changed_at        timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, user_id),
    FOREIGN KEY (tenant_id, user_id) REFERENCES app.users (tenant_id, id)
);
ALTER TABLE app.pin_credentials ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.pin_credentials FORCE ROW LEVEL SECURITY;
CREATE POLICY pin_credentials_tenant ON app.pin_credentials
    USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant());
GRANT SELECT, INSERT, UPDATE, DELETE ON app.pin_credentials TO stayguard_app;
GRANT SELECT, DELETE ON app.pin_credentials TO stayguard_maint;
