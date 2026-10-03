-- +goose Up
-- One cash ledger for every cash movement. A movement by the owner or a manager is on the open receptionist shift of the stay's building
-- when there is one (by_owner marks it: the cash leaves the same drawer), else it is owner cash: a ledger line with no shift.
ALTER TABLE app.cash_entries ALTER COLUMN shift_id DROP NOT NULL;
ALTER TABLE app.cash_entries ADD COLUMN by_owner boolean NOT NULL DEFAULT false;
CREATE INDEX cash_entries_by_time ON app.cash_entries (tenant_id, created_at);
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION app.cash_entries_require_open_shift() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF NEW.shift_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM app.shifts s WHERE s.tenant_id = NEW.tenant_id AND s.id = NEW.shift_id AND s.status = 'OPEN') THEN
        RAISE EXCEPTION 'cash entries need an open shift' USING ERRCODE = 'restrict_violation';
    END IF;
    RETURN NEW;
END
$$;
-- +goose StatementEnd

-- Cash the owner or a manager already took (no ledger line existed): deposits, cash payments and deposit refunds, as owner cash.
INSERT INTO app.cash_entries (id, tenant_id, shift_id, kind, amount, stay_id, created_at, by_owner)
SELECT 'ce_bfd_' || s.id, s.tenant_id, NULL, 'DEPOSIT', s.deposit, s.id, s.check_in_at, true
FROM app.stays s
WHERE s.deposit > 0 AND NOT EXISTS (SELECT 1 FROM app.cash_entries e WHERE e.tenant_id = s.tenant_id AND e.stay_id = s.id AND e.kind = 'DEPOSIT');

INSERT INTO app.cash_entries (id, tenant_id, shift_id, kind, amount, stay_id, payment_id, created_at, by_owner)
SELECT 'ce_bfp_' || p.id, p.tenant_id, NULL, 'PAYMENT', p.amount, iv.stay_id, p.id, p.paid_at, true
FROM app.payments p JOIN app.invoices iv ON iv.tenant_id = p.tenant_id AND iv.id = p.invoice_id
WHERE p.method = 'CASH' AND p.status = 'PAID' AND p.paid_at IS NOT NULL AND p.amount > 0
  AND NOT EXISTS (SELECT 1 FROM app.cash_entries e WHERE e.tenant_id = p.tenant_id AND e.payment_id = p.id AND e.kind = 'PAYMENT');

INSERT INTO app.cash_entries (id, tenant_id, shift_id, kind, amount, stay_id, payment_id, created_at, by_owner)
SELECT 'ce_bfr_' || p.id, p.tenant_id, NULL, 'REFUND', (iv.quote->>'refundDue')::bigint, iv.stay_id, p.id, p.paid_at, true
FROM app.payments p JOIN app.invoices iv ON iv.tenant_id = p.tenant_id AND iv.id = p.invoice_id
WHERE p.method = 'CASH' AND p.status = 'PAID' AND p.paid_at IS NOT NULL AND coalesce((iv.quote->>'refundDue')::bigint, 0) > 0
  AND NOT EXISTS (SELECT 1 FROM app.cash_entries e WHERE e.tenant_id = p.tenant_id AND e.payment_id = p.id AND e.kind = 'REFUND');
