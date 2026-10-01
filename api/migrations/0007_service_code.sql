-- +goose Up
-- SG-205: the contract addresses extras by service code. Table-level grants of 0003 already cover new columns.
-- The backfill reduces the English name like unit_types.code, falls back to the id, caps at 64 characters
-- (what the extras request accepts) and suffixes later services of one tenant that reduce to the same code
-- with _2, _3 ... (kept inside the cap) so the unique constraint below cannot fail on existing data.
ALTER TABLE app.services ADD COLUMN code text;
WITH base AS (
    SELECT id, tenant_id,
           left(coalesce(nullif(btrim(upper(regexp_replace(name->>'en', '[^A-Za-z0-9]+', '_', 'g')), '_'), ''), id), 64) AS b
    FROM app.services
), ranked AS (
    SELECT id, b, row_number() OVER (PARTITION BY tenant_id, b ORDER BY id) AS n FROM base
)
UPDATE app.services s
SET code = CASE WHEN r.n = 1 THEN r.b ELSE left(r.b, 64 - length('_' || r.n)) || '_' || r.n END
FROM ranked r
WHERE r.id = s.id;
ALTER TABLE app.services ALTER COLUMN code SET NOT NULL;
ALTER TABLE app.services ADD CONSTRAINT services_tenant_code_key UNIQUE (tenant_id, code);
