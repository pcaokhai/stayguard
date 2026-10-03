-- +goose Up
-- Alerts keep their history but can be resolved: an invoice paid, a transfer linked, a refund recorded. REFUND_PENDING is an open deposit
-- refund (nothing to collect), told apart from PAYMENT_UNPAID.
ALTER TABLE app.alerts ADD COLUMN resolved_at timestamptz, ADD COLUMN resolution text;
ALTER TABLE app.alerts ADD CONSTRAINT alerts_resolution_check CHECK ((resolved_at IS NULL) = (resolution IS NULL));
CREATE INDEX alerts_open ON app.alerts (tenant_id, created_at DESC) WHERE resolved_at IS NULL;
ALTER TABLE app.alerts DROP CONSTRAINT alerts_kind_check;
ALTER TABLE app.alerts ADD CONSTRAINT alerts_kind_check CHECK (kind IN ('ACCOUNT_LOCKED', 'CASH_OVER', 'CASH_SHORT', 'DAMAGE_REPORTED',
    'LEAVE_REQUESTED', 'OVERPAID', 'PAYMENT_MISMATCH', 'PAYMENT_PARTIAL', 'PAYMENT_UNPAID', 'REFUND_PENDING', 'SEPAY_UPDATED', 'STAY_TIME_EDITED',
    'STOCKTAKE_DIFFERENCE', 'UNMATCHED_TRANSFER', 'UNUSED_ROOM_REPORT'));

-- Alerts already in the database: a PAYMENT_UNPAID that was really an open refund becomes REFUND_PENDING, then everything whose invoice
-- is paid, or whose transfer was linked, is resolved. No manual repair.
UPDATE app.alerts a SET kind = 'REFUND_PENDING', details = a.details || jsonb_build_object('refundDue', coalesce(a.amount, 0)::text)
FROM app.invoices i
WHERE a.kind = 'PAYMENT_UNPAID' AND i.tenant_id = a.tenant_id AND i.stay_id = a.stay_id
  AND coalesce((i.quote->>'balanceDue')::bigint, 0) = 0 AND coalesce((i.quote->>'refundDue')::bigint, 0) > 0;

UPDATE app.alerts a SET resolved_at = coalesce(i.paid_at, now()),
       resolution = CASE WHEN a.kind = 'REFUND_PENDING' THEN 'REFUNDED' ELSE 'PAID' END
FROM app.invoices i
WHERE a.kind IN ('PAYMENT_MISMATCH', 'PAYMENT_PARTIAL', 'PAYMENT_UNPAID', 'REFUND_PENDING') AND a.resolved_at IS NULL
  AND i.tenant_id = a.tenant_id AND i.stay_id = a.stay_id AND i.status = 'PAID';

UPDATE app.alerts a SET resolved_at = now(), resolution = 'LINKED'
WHERE a.kind = 'UNMATCHED_TRANSFER' AND a.resolved_at IS NULL
  AND EXISTS (SELECT 1 FROM app.payment_events pe WHERE pe.tenant_id = a.tenant_id AND pe.amount = a.amount
              AND pe.reference_code = a.details->>'transferNote' AND pe.result = 'SETTLED');
