-- +goose Up
-- L-B1: stay corrections (time edit, move), who did what on a stay, and owner alerts.

-- Who created the row; NULL for rows that predate this migration. Table-level grants of 0003 cover new columns.
ALTER TABLE app.stays ADD COLUMN created_by text;
ALTER TABLE app.stay_extras ADD COLUMN created_by text;

CREATE TABLE app.stay_edits (
    id              text PRIMARY KEY,
    tenant_id       text NOT NULL,
    stay_id         text NOT NULL,
    kind            text NOT NULL CHECK (kind IN ('CHECK_IN', 'MOVE')),
    actor_id        text,
    old_check_in_at timestamptz,
    new_check_in_at timestamptz,
    reason_code     text,
    note            text,
    from_room_code  text,
    to_room_code    text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, stay_id) REFERENCES app.stays (tenant_id, id)
);
CREATE INDEX stay_edits_by_stay ON app.stay_edits (tenant_id, stay_id, created_at);

-- shift_id and stay_id carry no foreign key: an alert must outlive what it points at being tidied away,
-- and the shifts table (L-B2) comes after this one.
CREATE TABLE app.alerts (
    id         text PRIMARY KEY,
    tenant_id  text NOT NULL REFERENCES app.tenants (id),
    kind       text NOT NULL CHECK (kind IN ('ACCOUNT_LOCKED', 'CASH_OVER', 'CASH_SHORT', 'DAMAGE_REPORTED',
        'LEAVE_REQUESTED', 'PAYMENT_MISMATCH', 'SEPAY_UPDATED', 'STAY_TIME_EDITED', 'STOCKTAKE_DIFFERENCE',
        'UNMATCHED_TRANSFER', 'UNUSED_ROOM_REPORT')),
    room_code  text,
    shift_id   text,
    stay_id    text,
    actor_id   text,
    amount     bigint CHECK (amount >= 0),
    details    jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    read_at    timestamptz,
    read_by    text,
    UNIQUE (tenant_id, id)
);
CREATE INDEX alerts_by_tenant ON app.alerts (tenant_id, created_at DESC, id DESC);
CREATE INDEX alerts_unread ON app.alerts (tenant_id, created_at DESC) WHERE read_at IS NULL;

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['stay_edits', 'alerts'] LOOP
        EXECUTE format('ALTER TABLE app.%I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE app.%I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('CREATE POLICY %I ON app.%I USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant())', t || '_tenant', t);
    END LOOP;
END
$$;
-- +goose StatementEnd

GRANT SELECT, INSERT, UPDATE, DELETE ON app.stay_edits, app.alerts TO stayguard_app;
GRANT SELECT, DELETE ON app.stay_edits, app.alerts TO stayguard_maint;
