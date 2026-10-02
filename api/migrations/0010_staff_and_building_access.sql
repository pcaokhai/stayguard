-- +goose Up
-- L-A2: staff profiles and stored building access (replaces permissions.Derived and RoleBased).

-- Real people share names; sign-in uses the username. The demo role picker (users without a username that
-- can sign in) still needs one user per role name: concurrent demo requests rely on this conflict.
ALTER TABLE app.users DROP CONSTRAINT users_tenant_id_name_key;
CREATE UNIQUE INDEX users_demo_name ON app.users (tenant_id, name) WHERE username IS NULL AND app_access <> 'NONE';

CREATE TABLE app.staff_profiles (
    tenant_id           text NOT NULL,
    user_id             text NOT NULL,
    phone               text,
    position            text NOT NULL CHECK (position IN ('FRONT_DESK', 'HOUSEKEEPING', 'SECURITY', 'MANAGER', 'MAINTENANCE', 'OTHER')),
    pay_type            text NOT NULL CHECK (pay_type IN ('MONTHLY', 'PER_SHIFT', 'HOURLY')),
    rate                bigint NOT NULL CHECK (rate >= 0),
    fixed_allowance     bigint NOT NULL CHECK (fixed_allowance >= 0),
    standard_shifts     integer NOT NULL CHECK (standard_shifts >= 0),
    start_date          date NOT NULL,
    annual_leave_days   integer NOT NULL CHECK (annual_leave_days >= 0),
    created_at          timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, user_id),
    FOREIGN KEY (tenant_id, user_id) REFERENCES app.users (tenant_id, id)
);

-- One row per person and building; a missing row is NONE. The owner has implicit EDIT and needs no rows.
CREATE TABLE app.building_permissions (
    tenant_id   text NOT NULL,
    user_id     text NOT NULL,
    building_id text NOT NULL,
    level       text NOT NULL CHECK (level IN ('NONE', 'VIEW', 'EDIT')),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, user_id, building_id),
    FOREIGN KEY (tenant_id, user_id) REFERENCES app.users (tenant_id, id),
    FOREIGN KEY (tenant_id, building_id) REFERENCES app.buildings (tenant_id, id)
);

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['staff_profiles', 'building_permissions'] LOOP
        EXECUTE format('ALTER TABLE app.%I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE app.%I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('CREATE POLICY %I ON app.%I USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant())', t || '_tenant', t);
        EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON app.%I TO stayguard_app', t);
        EXECUTE format('GRANT SELECT, DELETE ON app.%I TO stayguard_maint', t);
    END LOOP;
END
$$;
-- +goose StatementEnd

-- Until now every non-owner acted in every building (FAST MODE). Keep that for existing data, so trials
-- created before this migration keep working; new buildings start with no staff access.
INSERT INTO app.building_permissions (tenant_id, user_id, building_id, level)
SELECT u.tenant_id, u.id, b.id, 'EDIT'
FROM app.users u JOIN app.buildings b ON b.tenant_id = u.tenant_id
WHERE u.role <> 'OWNER';
