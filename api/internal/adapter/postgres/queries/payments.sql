-- Every query filters by tenant explicitly; RLS is the second guard (ADR-005).
-- Rule: a TRANSFER payment is created PENDING and becomes PAID only through SettleTransfer, which only the
-- payment-event handler calls (CLAUDE.md §4.4). TestOnlySettleQuerySetsPaid_A2 guards this file.

-- name: LockInvoiceForPayment :one
SELECT i.id, i.status, i.bill_code, i.stay_id, i.quote, u.building_id
FROM app.invoices i
JOIN app.stays s ON s.tenant_id = i.tenant_id AND s.id = i.stay_id
JOIN app.units u ON u.tenant_id = s.tenant_id AND u.id = s.unit_id
WHERE i.tenant_id = @tenant_id AND i.id = @invoice_id
FOR UPDATE OF i;

-- name: GetPaymentByID :one
SELECT p.id, p.invoice_id, p.method, p.status, p.amount, p.received_amount, p.paid_at, p.transaction_id,
       i.bill_code, u.building_id
FROM app.payments p
JOIN app.invoices i ON i.tenant_id = p.tenant_id AND i.id = p.invoice_id
JOIN app.stays s ON s.tenant_id = i.tenant_id AND s.id = i.stay_id
JOIN app.units u ON u.tenant_id = s.tenant_id AND u.id = s.unit_id
WHERE p.tenant_id = @tenant_id AND p.id = @payment_id;

-- name: GetPendingPaymentForInvoice :one
SELECT p.id, p.invoice_id, p.method, p.status, p.amount, p.received_amount, p.paid_at, p.transaction_id,
       i.bill_code, u.building_id
FROM app.payments p
JOIN app.invoices i ON i.tenant_id = p.tenant_id AND i.id = p.invoice_id
JOIN app.stays s ON s.tenant_id = i.tenant_id AND s.id = i.stay_id
JOIN app.units u ON u.tenant_id = s.tenant_id AND u.id = s.unit_id
WHERE p.tenant_id = @tenant_id AND p.invoice_id = @invoice_id AND p.status = 'PENDING';

-- name: ListPendingTransfers :many
SELECT p.id, p.invoice_id, p.amount, i.bill_code, i.stay_id, u.code AS room_code
FROM app.payments p
JOIN app.invoices i ON i.tenant_id = p.tenant_id AND i.id = p.invoice_id
JOIN app.stays s ON s.tenant_id = i.tenant_id AND s.id = i.stay_id
JOIN app.units u ON u.tenant_id = s.tenant_id AND u.id = s.unit_id
WHERE p.tenant_id = @tenant_id AND p.method = 'TRANSFER' AND p.status = 'PENDING';

-- name: InsertPendingTransfer :exec
INSERT INTO app.payments (id, tenant_id, invoice_id, method, status, amount, reference_code, created_at)
VALUES (@id, @tenant_id, @invoice_id, 'TRANSFER', 'PENDING', @amount, @reference_code, @created_at);

-- name: InsertCashPayment :exec
INSERT INTO app.payments (id, tenant_id, invoice_id, method, status, amount, created_at, paid_at, received_amount)
VALUES (@id, @tenant_id, @invoice_id, 'CASH', 'PAID', @amount, @at, @at, @amount);

-- name: ExpirePendingForInvoice :exec
UPDATE app.payments SET status = 'EXPIRED'
WHERE tenant_id = @tenant_id AND invoice_id = @invoice_id AND status = 'PENDING';

-- name: SettleTransfer :execrows
UPDATE app.payments
SET status = 'PAID', paid_at = @paid_at, received_amount = @received_amount, transaction_id = @transaction_id
WHERE tenant_id = @tenant_id AND id = @payment_id AND method = 'TRANSFER' AND status = 'PENDING';

-- name: MarkTransferMismatch :execrows
UPDATE app.payments
SET status = 'MISMATCH', received_amount = @received_amount, transaction_id = @transaction_id
WHERE tenant_id = @tenant_id AND id = @payment_id AND method = 'TRANSFER' AND status = 'PENDING';

-- name: MarkInvoicePaid :execrows
UPDATE app.invoices SET status = 'PAID', paid_at = @paid_at
WHERE tenant_id = @tenant_id AND id = @invoice_id AND status = 'OPEN';

-- name: ReleaseRoomToClean :exec
UPDATE app.units u SET status = 'TO_CLEAN'
FROM app.stays s
WHERE u.tenant_id = @tenant_id AND u.status = 'OCCUPIED'
  AND s.tenant_id = u.tenant_id AND s.id = @stay_id AND s.unit_id = u.id;

-- name: InsertPaymentEvent :execrows
INSERT INTO app.payment_events (id, tenant_id, provider, external_id, amount, reference_code, result, received_at)
VALUES (@id, @tenant_id, @provider, @external_id, @amount, @reference_code, 'UNMATCHED', @received_at)
ON CONFLICT (provider, external_id) DO NOTHING;

-- name: SetPaymentEventResult :exec
UPDATE app.payment_events SET result = @result
WHERE tenant_id = @tenant_id AND provider = @provider AND external_id = @external_id;

-- name: GetDefaultBankAccount :one
SELECT id, account_enc FROM app.bank_accounts
WHERE tenant_id = @tenant_id AND is_default AND sepay_status = 'CONNECTED';
