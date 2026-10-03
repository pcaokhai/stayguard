-- +goose Up
-- Who closed an unmatched bank transfer, when, and why (result DISMISSED, allowed since migration 0031). The event row stays for audit.
ALTER TABLE app.payment_events ADD COLUMN dismissed_note text, ADD COLUMN dismissed_by text, ADD COLUMN dismissed_at timestamptz;
