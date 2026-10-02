-- +goose Up
-- Partial transfers: bank events matched to an invoice accumulate. An event that matched an invoice but did not close it is
-- PARTIAL; when the invoice is paid all its events become SETTLED. Overpayments raise an OVERPAID alert.
ALTER TABLE app.payment_events ADD COLUMN invoice_id text;
CREATE INDEX payment_events_by_invoice ON app.payment_events (tenant_id, invoice_id) WHERE invoice_id IS NOT NULL;
ALTER TABLE app.payment_events DROP CONSTRAINT payment_events_result_check;
ALTER TABLE app.payment_events ADD CONSTRAINT payment_events_result_check
    CHECK (result IN ('SETTLED', 'PARTIAL', 'MISMATCH', 'UNMATCHED', 'DUPLICATE_IGNORED'));

ALTER TABLE app.alerts DROP CONSTRAINT alerts_kind_check;
ALTER TABLE app.alerts ADD CONSTRAINT alerts_kind_check CHECK (kind IN ('ACCOUNT_LOCKED', 'CASH_OVER', 'CASH_SHORT', 'DAMAGE_REPORTED',
    'LEAVE_REQUESTED', 'OVERPAID', 'PAYMENT_MISMATCH', 'SEPAY_UPDATED', 'STAY_TIME_EDITED', 'STOCKTAKE_DIFFERENCE',
    'UNMATCHED_TRANSFER', 'UNUSED_ROOM_REPORT'));

-- The transactions list shows partial events (money that arrived but did not settle the invoice) as MISMATCH rows, in place of
-- the payments that used to be marked MISMATCH (those rows stay for history).
CREATE OR REPLACE VIEW app.transactions WITH (security_invoker = true) AS
    SELECT p.tenant_id, p.id AS id, p.paid_at AS happened_at, coalesce(p.received_amount, p.amount) AS amount, p.method AS method,
           un.code AS room_code, iv.bill_code AS bill_code,
           CASE WHEN p.method = 'CASH' THEN 'CASH' ELSE 'MATCHED' END AS reconciliation,
           ''::text AS transfer_note, coalesce(pe.id, '') AS payment_event_id, coalesce(ce.shift_id, '') AS shift_id
    FROM app.payments p
    JOIN app.invoices iv ON iv.tenant_id = p.tenant_id AND iv.id = p.invoice_id
    JOIN app.stays st ON st.tenant_id = iv.tenant_id AND st.id = iv.stay_id
    JOIN app.units un ON un.tenant_id = st.tenant_id AND un.id = st.unit_id
    LEFT JOIN app.payment_events pe ON pe.tenant_id = p.tenant_id AND pe.external_id = p.transaction_id
    LEFT JOIN app.cash_entries ce ON ce.tenant_id = p.tenant_id AND ce.payment_id = p.id AND ce.kind = 'PAYMENT'
    WHERE p.status = 'PAID' AND p.paid_at IS NOT NULL
  UNION ALL
    SELECT p.tenant_id, p.id, coalesce(pe.received_at, p.created_at), coalesce(p.received_amount, p.amount), p.method,
           un.code, iv.bill_code, 'MISMATCH', coalesce(pe.reference_code, ''), coalesce(pe.id, ''), ''
    FROM app.payments p
    JOIN app.invoices iv ON iv.tenant_id = p.tenant_id AND iv.id = p.invoice_id
    JOIN app.stays st ON st.tenant_id = iv.tenant_id AND st.id = iv.stay_id
    JOIN app.units un ON un.tenant_id = st.tenant_id AND un.id = st.unit_id
    LEFT JOIN app.payment_events pe ON pe.tenant_id = p.tenant_id AND pe.external_id = p.transaction_id AND pe.result = 'MISMATCH'
    WHERE p.status = 'MISMATCH'
  UNION ALL
    SELECT pe.tenant_id, pe.id, pe.received_at, pe.amount, 'TRANSFER', un.code, iv.bill_code, 'MISMATCH',
           coalesce(pe.reference_code, ''), pe.id, ''
    FROM app.payment_events pe
    JOIN app.invoices iv ON iv.tenant_id = pe.tenant_id AND iv.id = pe.invoice_id
    JOIN app.stays st ON st.tenant_id = iv.tenant_id AND st.id = iv.stay_id
    JOIN app.units un ON un.tenant_id = st.tenant_id AND un.id = st.unit_id
    WHERE pe.result = 'PARTIAL'
  UNION ALL
    SELECT pe.tenant_id, pe.id, pe.received_at, pe.amount, 'TRANSFER', '', '', 'UNMATCHED', coalesce(pe.reference_code, ''), pe.id, ''
    FROM app.payment_events pe
    WHERE pe.result = 'UNMATCHED' AND pe.tenant_id IS NOT NULL;
