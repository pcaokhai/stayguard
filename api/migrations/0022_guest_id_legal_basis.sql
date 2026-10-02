-- +goose Up
-- The guest ID is collected under the duty to declare stays, not by consent: the record says so instead of holding a consent.
ALTER TABLE app.guest_ids RENAME COLUMN consent_at TO collected_at;
ALTER TABLE app.guest_ids RENAME COLUMN consent_by TO collected_by;
ALTER TABLE app.guest_ids ADD COLUMN legal_basis text NOT NULL DEFAULT 'STAY_DECLARATION' CHECK (legal_basis = 'STAY_DECLARATION');
