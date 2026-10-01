-- +goose Up
-- SG-205: the contract addresses extras by service code. Table-level grants of 0003 already cover new columns.
ALTER TABLE app.services ADD COLUMN code text;
UPDATE app.services
SET code = coalesce(
    nullif(btrim(upper(regexp_replace(name->>'en', '[^A-Za-z0-9]+', '_', 'g')), '_'), ''),
    id);
ALTER TABLE app.services ALTER COLUMN code SET NOT NULL;
ALTER TABLE app.services ADD CONSTRAINT services_tenant_code_key UNIQUE (tenant_id, code);
