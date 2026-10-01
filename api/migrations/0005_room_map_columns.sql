-- +goose Up
-- SG-201: two contract fields the room map needs. Table-level grants of 0003 already cover new columns.
ALTER TABLE app.unit_types ADD COLUMN code text;
UPDATE app.unit_types
SET code = coalesce(
    nullif(btrim(upper(regexp_replace(name->>'en', '[^A-Za-z0-9]+', '_', 'g')), '_'), ''),
    id);
ALTER TABLE app.unit_types ALTER COLUMN code SET NOT NULL;
ALTER TABLE app.unit_types ADD CONSTRAINT unit_types_tenant_code_key UNIQUE (tenant_id, code);

-- The default only fills rows that exist today; SG-203 always provides the guest name.
ALTER TABLE app.stays ADD COLUMN guest_name text NOT NULL DEFAULT '';
ALTER TABLE app.stays ALTER COLUMN guest_name DROP DEFAULT;
