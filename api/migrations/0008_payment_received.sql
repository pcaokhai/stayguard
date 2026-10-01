-- +goose Up
-- A2: what the bank reported (amount and provider transaction id). Table-level grants of 0003 already cover new columns.
ALTER TABLE app.payments ADD COLUMN received_amount bigint CHECK (received_amount >= 0);
ALTER TABLE app.payments ADD COLUMN transaction_id text;
