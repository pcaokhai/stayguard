-- +goose Up
-- Bank events that must never be matched or linked, kept for audit and dedupe:
--   IGNORED   outgoing money and money to an account that is not the tenant's (set by the webhook, no alert, not in any list).
--   DISMISSED an unmatched inbound transfer the owner closed with a note (reserved for the dismiss operation; nothing sets it yet).
-- Rows stored before this migration cannot be classified (no direction was stored), so none is backfilled: an old outgoing or
-- other-account event stays UNMATCHED.
ALTER TABLE app.payment_events DROP CONSTRAINT payment_events_result_check;
ALTER TABLE app.payment_events ADD CONSTRAINT payment_events_result_check
    CHECK (result IN ('SETTLED', 'PARTIAL', 'MISMATCH', 'UNMATCHED', 'IGNORED', 'DISMISSED', 'DUPLICATE_IGNORED'));
