-- +goose Up
-- PAYMENT_PARTIAL: bank money reached an invoice but 15 minutes later it is still not fully paid.
ALTER TABLE app.alerts DROP CONSTRAINT alerts_kind_check;
ALTER TABLE app.alerts ADD CONSTRAINT alerts_kind_check CHECK (kind IN ('ACCOUNT_LOCKED', 'CASH_OVER', 'CASH_SHORT', 'DAMAGE_REPORTED',
    'LEAVE_REQUESTED', 'OVERPAID', 'PAYMENT_MISMATCH', 'PAYMENT_PARTIAL', 'SEPAY_UPDATED', 'STAY_TIME_EDITED', 'STOCKTAKE_DIFFERENCE',
    'UNMATCHED_TRANSFER', 'UNUSED_ROOM_REPORT'));

