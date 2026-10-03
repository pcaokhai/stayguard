-- +goose Up
-- A paid invoice leaves nothing for needs-action. Clean what is already there: partial and mismatch bank events of invoices that are paid,
-- and the old MISMATCH payment rows of those invoices.
UPDATE app.payment_events pe SET result = 'SETTLED'
FROM app.invoices iv
WHERE pe.result = 'PARTIAL' AND iv.tenant_id = pe.tenant_id AND iv.id = pe.invoice_id AND iv.status = 'PAID';

UPDATE app.payment_events pe SET result = 'SETTLED', invoice_id = p.invoice_id
FROM app.payments p
JOIN app.invoices iv ON iv.tenant_id = p.tenant_id AND iv.id = p.invoice_id AND iv.status = 'PAID'
WHERE pe.result = 'MISMATCH' AND p.tenant_id = pe.tenant_id AND p.transaction_id = pe.external_id AND p.status = 'MISMATCH';

UPDATE app.payments p SET status = 'EXPIRED'
FROM app.invoices iv
WHERE p.status = 'MISMATCH' AND iv.tenant_id = p.tenant_id AND iv.id = p.invoice_id AND iv.status = 'PAID';
