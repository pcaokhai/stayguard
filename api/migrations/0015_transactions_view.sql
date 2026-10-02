-- +goose Up
-- L-B3: one list of money events for the owner. Linked events show through the payment they settled, so an event appears once.
-- security_invoker makes the view run with the caller's rights: row-level security of the tables underneath still applies.
CREATE VIEW app.transactions WITH (security_invoker = true) AS
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
    SELECT pe.tenant_id, pe.id, pe.received_at, pe.amount, 'TRANSFER', '', '', 'UNMATCHED', coalesce(pe.reference_code, ''), pe.id, ''
    FROM app.payment_events pe
    WHERE pe.result = 'UNMATCHED' AND pe.tenant_id IS NOT NULL;

GRANT SELECT ON app.transactions TO stayguard_app;
