-- +goose Up
-- L-B4: damage reports become maintenance tickets; a ticket can lock its room.
-- A per-tenant counter gives the human code (BT-001, BT-002 ...) without two reports racing to the same number.
ALTER TABLE app.tenants ADD COLUMN ticket_seq integer NOT NULL DEFAULT 0;

CREATE TABLE app.maintenance_tickets (
    id             text PRIMARY KEY,
    tenant_id      text NOT NULL,
    code           text NOT NULL,
    unit_id        text NOT NULL,
    category       text NOT NULL CHECK (category IN ('AIR_CONDITIONER', 'HOT_WATER', 'PLUMBING', 'POWER_LIGHTS', 'TV', 'DOOR_LOCK', 'MISSING_ITEMS', 'OTHER')),
    description    text NOT NULL,
    status         text NOT NULL DEFAULT 'NEW' CHECK (status IN ('NEW', 'IN_REPAIR', 'DONE')),
    room_locked    boolean NOT NULL DEFAULT false,
    reported_by    text,
    reported_at    timestamptz NOT NULL,
    expected_done_on date,
    parts_cost     bigint CHECK (parts_cost >= 0),
    labour_cost    bigint CHECK (labour_cost >= 0),
    repairer       text,
    note           text,
    completed_at   timestamptz,
    completed_by   text,
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, code),
    CHECK ((status = 'DONE') = (completed_at IS NOT NULL)),
    CHECK (status <> 'DONE' OR NOT room_locked),
    FOREIGN KEY (tenant_id, unit_id) REFERENCES app.units (tenant_id, id)
);
CREATE INDEX maintenance_tickets_by_status ON app.maintenance_tickets (tenant_id, status, reported_at DESC);
CREATE INDEX maintenance_tickets_by_unit ON app.maintenance_tickets (tenant_id, unit_id) WHERE status <> 'DONE';

-- +goose StatementBegin
DO $$
BEGIN
    ALTER TABLE app.maintenance_tickets ENABLE ROW LEVEL SECURITY;
    ALTER TABLE app.maintenance_tickets FORCE ROW LEVEL SECURITY;
    CREATE POLICY maintenance_tickets_tenant ON app.maintenance_tickets
        USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant());
END
$$;
-- +goose StatementEnd

GRANT SELECT, INSERT, UPDATE, DELETE ON app.maintenance_tickets TO stayguard_app;
GRANT SELECT, DELETE ON app.maintenance_tickets TO stayguard_maint;
