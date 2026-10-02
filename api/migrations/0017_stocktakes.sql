-- +goose Up
-- F-A1: stocktakes, and items that can be deleted when they were never sold.

CREATE TABLE app.stocktakes (
    id               text PRIMARY KEY,
    tenant_id        text NOT NULL,
    actor_id         text,
    note             text,
    value_difference bigint NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id)
);
ALTER TABLE app.stocktakes ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.stocktakes FORCE ROW LEVEL SECURITY;
CREATE POLICY stocktakes_tenant ON app.stocktakes
    USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant());
GRANT SELECT, INSERT ON app.stocktakes TO stayguard_app;
GRANT SELECT, DELETE ON app.stocktakes TO stayguard_maint;

-- Deleting an item that was never sold takes its movements with it. The app role has no DELETE grant on
-- stock_movements; the cascade runs as the table owner, and items with sales are stopped, never deleted.
ALTER TABLE app.stock_movements DROP CONSTRAINT stock_movements_tenant_id_service_id_fkey;
ALTER TABLE app.stock_movements ADD CONSTRAINT stock_movements_service_fkey
    FOREIGN KEY (tenant_id, service_id) REFERENCES app.services (tenant_id, id) ON DELETE CASCADE;
