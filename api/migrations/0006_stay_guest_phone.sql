-- +goose Up
-- SG-203: guest phone is required by the check-in contract; table-level grants of 0003 already cover new columns.
-- The defaults only fill rows that exist today; check-in always provides both values.
ALTER TABLE app.stays ADD COLUMN guest_phone text NOT NULL DEFAULT '';
ALTER TABLE app.stays ALTER COLUMN guest_phone DROP DEFAULT;

-- The stored snapshot's schema version (resolves SG-101 Ruling 7): readers pick the parser by it.
ALTER TABLE app.stays ADD COLUMN rate_plan_schema smallint NOT NULL DEFAULT 1;
ALTER TABLE app.stays ALTER COLUMN rate_plan_schema DROP DEFAULT;
