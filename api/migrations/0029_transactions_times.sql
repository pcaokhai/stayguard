-- +goose Up
-- Transactions: a transfer shows when the bank money arrived (received_at) and when it was settled or linked (settled_at), sorted by
-- the first. A deposit refund is its own negative line (kind CASH_REFUND) instead of a payment of 0.
CREATE OR REPLACE VIEW app.transactions WITH (security_invoker = true) AS
    SELECT p.tenant_id, p.id AS id, coalesce(pe.received_at, p.paid_at) AS happened_at, coalesce(p.received_amount, p.amount) AS amount, p.method AS method,
           un.code AS room_code, iv.bill_code AS bill_code,
           CASE WHEN p.method = 'CASH' THEN 'CASH' ELSE 'MATCHED' END AS reconciliation,
           ''::text AS transfer_note, coalesce(pe.id, '') AS payment_event_id, coalesce(ce.shift_id, '') AS shift_id,
           'PAYMENT'::text AS kind, pe.received_at AS received_at, p.paid_at AS settled_at
    FROM app.payments p
    JOIN app.invoices iv ON iv.tenant_id = p.tenant_id AND iv.id = p.invoice_id
    JOIN app.stays st ON st.tenant_id = iv.tenant_id AND st.id = iv.stay_id
    JOIN app.units un ON un.tenant_id = st.tenant_id AND un.id = st.unit_id
    LEFT JOIN app.payment_events pe ON pe.tenant_id = p.tenant_id AND pe.external_id = p.transaction_id
    LEFT JOIN app.cash_entries ce ON ce.tenant_id = p.tenant_id AND ce.payment_id = p.id AND ce.kind = 'PAYMENT'
    WHERE p.status = 'PAID' AND p.paid_at IS NOT NULL AND coalesce(p.received_amount, p.amount) > 0
  UNION ALL
    SELECT p.tenant_id, p.id, coalesce(pe.received_at, p.created_at), coalesce(p.received_amount, p.amount), p.method,
           un.code, iv.bill_code, 'MISMATCH', coalesce(pe.reference_code, ''), coalesce(pe.id, ''), '',
           'PAYMENT', pe.received_at, NULL::timestamptz
    FROM app.payments p
    JOIN app.invoices iv ON iv.tenant_id = p.tenant_id AND iv.id = p.invoice_id
    JOIN app.stays st ON st.tenant_id = iv.tenant_id AND st.id = iv.stay_id
    JOIN app.units un ON un.tenant_id = st.tenant_id AND un.id = st.unit_id
    LEFT JOIN app.payment_events pe ON pe.tenant_id = p.tenant_id AND pe.external_id = p.transaction_id AND pe.result = 'MISMATCH'
    WHERE p.status = 'MISMATCH'
  UNION ALL
    SELECT pe.tenant_id, pe.id, pe.received_at, pe.amount, 'TRANSFER', un.code, iv.bill_code, 'MISMATCH',
           coalesce(pe.reference_code, ''), pe.id, '', 'PAYMENT', pe.received_at, NULL::timestamptz
    FROM app.payment_events pe
    JOIN app.invoices iv ON iv.tenant_id = pe.tenant_id AND iv.id = pe.invoice_id
    JOIN app.stays st ON st.tenant_id = iv.tenant_id AND st.id = iv.stay_id
    JOIN app.units un ON un.tenant_id = st.tenant_id AND un.id = st.unit_id
    WHERE pe.result = 'PARTIAL'
  UNION ALL
    SELECT pe.tenant_id, pe.id, pe.received_at, pe.amount, 'TRANSFER', '', '', 'UNMATCHED', coalesce(pe.reference_code, ''), pe.id, '',
           'PAYMENT', pe.received_at, NULL::timestamptz
    FROM app.payment_events pe
    WHERE pe.result = 'UNMATCHED' AND pe.tenant_id IS NOT NULL
  UNION ALL
    SELECT p.tenant_id, p.id, p.paid_at, -coalesce((iv.quote->>'refundDue')::bigint, 0), 'CASH',
           un.code, iv.bill_code, 'CASH', '', '', coalesce(ce.shift_id, ''), 'CASH_REFUND', NULL::timestamptz, p.paid_at
    FROM app.payments p
    JOIN app.invoices iv ON iv.tenant_id = p.tenant_id AND iv.id = p.invoice_id
    JOIN app.stays st ON st.tenant_id = iv.tenant_id AND st.id = iv.stay_id
    JOIN app.units un ON un.tenant_id = st.tenant_id AND un.id = st.unit_id
    LEFT JOIN app.cash_entries ce ON ce.tenant_id = p.tenant_id AND ce.payment_id = p.id AND ce.kind = 'REFUND'
    WHERE p.method = 'CASH' AND p.status = 'PAID' AND p.paid_at IS NOT NULL AND coalesce((iv.quote->>'refundDue')::bigint, 0) > 0;
