-- +goose Up
-- No foreign key to tenants: the log must outlive a cleaned-up trial and stay append-only.
CREATE TABLE app.audit_logs (
    id          text PRIMARY KEY,
    tenant_id   text NOT NULL,
    actor_id    text,
    action      text NOT NULL,
    entity_type text NOT NULL,
    entity_id   text NOT NULL,
    before      jsonb,
    after       jsonb,
    trace_id    text,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_by_tenant ON app.audit_logs (tenant_id, created_at DESC);

-- +goose StatementBegin
CREATE FUNCTION app.audit_logs_reject_change() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs is append-only' USING ERRCODE = 'restrict_violation';
END
$$;
-- +goose StatementEnd
CREATE TRIGGER audit_logs_append_only BEFORE UPDATE OR DELETE ON app.audit_logs
    FOR EACH ROW EXECUTE FUNCTION app.audit_logs_reject_change();
CREATE TRIGGER audit_logs_no_truncate BEFORE TRUNCATE ON app.audit_logs
    FOR EACH STATEMENT EXECUTE FUNCTION app.audit_logs_reject_change();

CREATE TABLE app.idempotency_keys (
    tenant_id     text NOT NULL REFERENCES app.tenants (id),
    route         text NOT NULL,
    key           text NOT NULL,
    request_hash  text NOT NULL,
    status_code   integer,
    response_body bytea,
    created_at    timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, route, key)
);
CREATE INDEX idempotency_keys_expires_at ON app.idempotency_keys (expires_at);

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['audit_logs', 'idempotency_keys'] LOOP
        EXECUTE format('ALTER TABLE app.%I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE app.%I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('CREATE POLICY %I ON app.%I USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant())', t || '_tenant', t);
    END LOOP;
END
$$;
-- +goose StatementEnd

-- Grants: the app role has data access only, no DDL (it owns nothing and has no CREATE on any schema).
-- Later migrations grant on the tables they add.
GRANT USAGE ON SCHEMA app TO stayguard_app, stayguard_maint;
GRANT SELECT, INSERT, UPDATE, DELETE ON
    app.tenants, app.users, app.sessions, app.properties, app.buildings, app.floors, app.unit_types,
    app.units, app.stays, app.services, app.stay_extras, app.invoices, app.payments,
    app.payment_events, app.idempotency_keys
    TO stayguard_app;
GRANT SELECT, INSERT ON app.audit_logs TO stayguard_app;
-- Maintenance reads and deletes only (trial cleanup); audit_logs is never deleted.
GRANT SELECT, DELETE ON
    app.tenants, app.users, app.sessions, app.properties, app.buildings, app.floors, app.unit_types,
    app.units, app.stays, app.services, app.stay_extras, app.invoices, app.payments,
    app.payment_events, app.idempotency_keys
    TO stayguard_maint;
GRANT SELECT ON app.audit_logs TO stayguard_maint;
