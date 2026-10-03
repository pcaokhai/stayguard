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

-- name: ReceivedForInvoice :one
-- Bank money already matched to the invoice that did not close it.
SELECT coalesce(sum(amount), 0)::bigint FROM app.payment_events
WHERE tenant_id = @tenant_id AND invoice_id = @invoice_id AND result = 'PARTIAL';

-- name: SetPaymentEventMatched :exec
UPDATE app.payment_events SET result = @result, invoice_id = @invoice_id
WHERE tenant_id = @tenant_id AND provider = @provider AND external_id = @external_id;

-- name: SettleInvoiceEvents :exec
UPDATE app.payment_events SET result = 'SETTLED'
WHERE tenant_id = @tenant_id AND invoice_id = @invoice_id AND result = 'PARTIAL';

-- name: SetTransferReceived :execrows
-- A partial transfer: the pending payment records how much the bank has sent so far and stays PENDING.
UPDATE app.payments SET received_amount = @received_amount
WHERE tenant_id = @tenant_id AND id = @payment_id AND method = 'TRANSFER' AND status = 'PENDING';

-- name: ListStalePartials :many
-- Open invoices that received bank money (PARTIAL events) whose first such event is older than @before and that have no
-- PAYMENT_PARTIAL alert yet (the bill code is unique per tenant).
SELECT iv.id AS invoice_id, iv.bill_code, iv.stay_id, un.code AS room_code,
       coalesce((iv.quote->>'balanceDue')::bigint, 0)::bigint AS balance_due,
       sum(pe.amount)::bigint AS received, min(pe.received_at)::timestamptz AS first_at
FROM app.invoices iv
JOIN app.payment_events pe ON pe.tenant_id = iv.tenant_id AND pe.invoice_id = iv.id AND pe.result = 'PARTIAL'
JOIN app.stays s ON s.tenant_id = iv.tenant_id AND s.id = iv.stay_id
JOIN app.units un ON un.tenant_id = s.tenant_id AND un.id = s.unit_id
WHERE iv.tenant_id = @tenant_id AND iv.status = 'OPEN'
  AND NOT EXISTS (SELECT 1 FROM app.alerts a WHERE a.tenant_id = iv.tenant_id AND a.kind = 'PAYMENT_PARTIAL' AND a.details->>'billCode' = iv.bill_code)
GROUP BY iv.id, iv.bill_code, iv.stay_id, un.code, iv.quote
HAVING min(pe.received_at) <= @before::timestamptz
ORDER BY min(pe.received_at), iv.id;

-- name: ListStaleUnpaid :many
-- Open invoices (nothing paid, nothing refunded) checked out at or before @before, with no bank money on them and no
-- PAYMENT_UNPAID alert yet for the stay. A refund-only invoice has balance_due 0 and refund_due above it.
SELECT iv.id AS invoice_id, iv.bill_code, iv.stay_id, un.code AS room_code,
       coalesce((iv.quote->>'balanceDue')::bigint, 0)::bigint AS balance_due,
       coalesce((iv.quote->>'refundDue')::bigint, 0)::bigint AS refund_due
FROM app.invoices iv
JOIN app.stays s ON s.tenant_id = iv.tenant_id AND s.id = iv.stay_id
JOIN app.units un ON un.tenant_id = s.tenant_id AND un.id = s.unit_id
WHERE iv.tenant_id = @tenant_id AND iv.status = 'OPEN' AND iv.created_at <= @before::timestamptz
  AND NOT EXISTS (SELECT 1 FROM app.payment_events pe WHERE pe.tenant_id = iv.tenant_id AND pe.invoice_id = iv.id AND pe.result = 'PARTIAL')
  AND NOT EXISTS (SELECT 1 FROM app.alerts a WHERE a.tenant_id = iv.tenant_id AND a.kind = 'PAYMENT_UNPAID' AND a.stay_id = iv.stay_id)
ORDER BY iv.created_at, iv.id;
