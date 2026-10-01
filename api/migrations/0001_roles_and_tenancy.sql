-- +goose Up
CREATE SCHEMA IF NOT EXISTS app;

-- Roles carry no password and cannot log in until deploy tooling sets one (CLAUDE.md §9).
-- PostgreSQL has no CREATE ROLE IF NOT EXISTS, so each role is guarded; an existing role is left as is.
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'stayguard_app') THEN
        CREATE ROLE stayguard_app NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'stayguard_maint') THEN
        -- Trial cleanup only (SG-102 or later): reads and deletes across tenants.
        CREATE ROLE stayguard_maint NOLOGIN NOSUPERUSER BYPASSRLS NOCREATEDB NOCREATEROLE;
    END IF;
END
$$;
-- +goose StatementEnd

-- The tenant of the current transaction. Unset or empty gives NULL, which equals nothing, so every
-- policy built on it returns zero rows (fail closed, ADR-005).
CREATE FUNCTION app.current_tenant() RETURNS text
    LANGUAGE sql STABLE
    AS $$ SELECT nullif(current_setting('app.tenant_id', true), '') $$;

CREATE TABLE app.tenants (
    id             text PRIMARY KEY,
    name           text NOT NULL,
    default_locale text NOT NULL DEFAULT 'vi' CHECK (default_locale IN ('vi', 'en')),
    currency       text NOT NULL DEFAULT 'VND' CHECK (currency = 'VND'),
    time_zone      text NOT NULL DEFAULT 'Asia/Ho_Chi_Minh',
    is_trial       boolean NOT NULL DEFAULT false,
    expires_at     timestamptz,
    bank_account_enc bytea,
    created_at     timestamptz NOT NULL DEFAULT now(),
    CHECK (NOT is_trial OR expires_at IS NOT NULL)
);
CREATE INDEX tenants_trial_expiry ON app.tenants (expires_at) WHERE is_trial;
ALTER TABLE app.tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.tenants FORCE ROW LEVEL SECURITY;
CREATE POLICY tenants_own ON app.tenants
    USING (id = app.current_tenant()) WITH CHECK (id = app.current_tenant());

CREATE TABLE app.users (
    id         text PRIMARY KEY,
    tenant_id  text NOT NULL REFERENCES app.tenants (id),
    name       text NOT NULL,
    role       text NOT NULL CHECK (role IN ('OWNER', 'RECEPTIONIST', 'HOUSEKEEPING')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, name)
);

CREATE TABLE app.sessions (
    token_hash text PRIMARY KEY,
    tenant_id  text NOT NULL,
    user_id    text NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, user_id) REFERENCES app.users (tenant_id, id)
);
CREATE INDEX sessions_expires_at ON app.sessions (expires_at);

CREATE TABLE app.properties (
    id         text PRIMARY KEY,
    tenant_id  text NOT NULL REFERENCES app.tenants (id),
    name       text NOT NULL,
    vertical   text NOT NULL DEFAULT 'GUESTHOUSE' CHECK (vertical IN ('GUESTHOUSE')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id)
);

CREATE TABLE app.buildings (
    id          text PRIMARY KEY,
    tenant_id   text NOT NULL,
    property_id text NOT NULL,
    code        text NOT NULL,
    name        text NOT NULL,
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, code),
    FOREIGN KEY (tenant_id, property_id) REFERENCES app.properties (tenant_id, id)
);

CREATE TABLE app.floors (
    id          text PRIMARY KEY,
    tenant_id   text NOT NULL,
    building_id text NOT NULL,
    level       integer NOT NULL,
    UNIQUE (tenant_id, id),
    UNIQUE (building_id, level),
    FOREIGN KEY (tenant_id, building_id) REFERENCES app.buildings (tenant_id, id)
);

CREATE TABLE app.unit_types (
    id                text PRIMARY KEY,
    tenant_id         text NOT NULL REFERENCES app.tenants (id),
    name              jsonb NOT NULL,
    rate_plan         jsonb NOT NULL,
    rate_plan_version integer NOT NULL CHECK (rate_plan_version > 0),
    UNIQUE (tenant_id, id)
);

CREATE TABLE app.units (
    id           text PRIMARY KEY,
    tenant_id    text NOT NULL,
    building_id  text NOT NULL,
    floor_id     text NOT NULL,
    unit_type_id text NOT NULL,
    code         text NOT NULL,
    status       text NOT NULL DEFAULT 'VACANT' CHECK (status IN ('VACANT', 'OCCUPIED', 'TO_CLEAN', 'MAINTENANCE')),
    attributes   jsonb NOT NULL DEFAULT '{}',
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, code),
    FOREIGN KEY (tenant_id, building_id) REFERENCES app.buildings (tenant_id, id),
    FOREIGN KEY (tenant_id, floor_id) REFERENCES app.floors (tenant_id, id),
    FOREIGN KEY (tenant_id, unit_type_id) REFERENCES app.unit_types (tenant_id, id)
);
CREATE INDEX units_room_map ON app.units (tenant_id, building_id, code);

-- Every table below the tenants table is isolated by the same policy.
-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['users', 'sessions', 'properties', 'buildings', 'floors', 'unit_types', 'units'] LOOP
        EXECUTE format('ALTER TABLE app.%I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE app.%I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('CREATE POLICY %I ON app.%I USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant())', t || '_tenant', t);
    END LOOP;
END
$$;
-- +goose StatementEnd
