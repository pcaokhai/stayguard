-- +goose Up
-- L-A4: setup of rooms, rates and items; stock moves only through movements.

ALTER TABLE app.floors ADD COLUMN name text;

-- A retired room keeps its history and code but leaves the map and cannot be checked in.
ALTER TABLE app.units ADD COLUMN retired boolean NOT NULL DEFAULT false;

ALTER TABLE app.unit_types ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();

ALTER TABLE app.services
    ADD COLUMN unit             text NOT NULL DEFAULT 'pcs',
    ADD COLUMN low_stock_at     integer NOT NULL DEFAULT 0 CHECK (low_stock_at >= 0),
    ADD COLUMN on_sale          boolean NOT NULL DEFAULT true,
    ADD COLUMN latest_unit_cost bigint CHECK (latest_unit_cost >= 0);

-- Every change of services.stock has one row here. quantity is signed: IN and OPENING add, SALE subtracts,
-- COUNT and ADJUST carry the difference.
CREATE TABLE app.stock_movements (
    id          text PRIMARY KEY,
    tenant_id   text NOT NULL,
    service_id  text NOT NULL,
    kind        text NOT NULL CHECK (kind IN ('OPENING', 'IN', 'SALE', 'COUNT', 'ADJUST')),
    quantity    integer NOT NULL CHECK (quantity <> 0),
    unit_cost   bigint CHECK (unit_cost >= 0),
    ref         text,
    actor_id    text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, service_id) REFERENCES app.services (tenant_id, id)
);
CREATE INDEX stock_movements_by_service ON app.stock_movements (tenant_id, service_id, created_at DESC, id DESC);
ALTER TABLE app.stock_movements ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.stock_movements FORCE ROW LEVEL SECURITY;
CREATE POLICY stock_movements_tenant ON app.stock_movements
    USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant());
GRANT SELECT, INSERT ON app.stock_movements TO stayguard_app;
GRANT SELECT, DELETE ON app.stock_movements TO stayguard_maint;

-- Items that exist today were stocked without a record: give each an OPENING movement so the history adds up.
INSERT INTO app.stock_movements (id, tenant_id, service_id, kind, quantity)
SELECT 'sm_open_' || id, tenant_id, id, 'OPENING', stock::integer FROM app.services WHERE stock > 0;
