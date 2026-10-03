-- Every query filters by tenant explicitly; RLS is the second guard (ADR-005).

-- name: LockStayByID :one
-- Same projection as GetStayByID. FOR UPDATE OF s serializes extras and check-out on one stay.
SELECT s.id, s.unit_id, u.code AS room_code, u.building_id, s.rental_type, s.status, s.guest_name, s.guest_phone,
       EXISTS (SELECT 1 FROM app.guest_ids g WHERE g.tenant_id = s.tenant_id AND g.stay_id = s.id AND g.number_enc IS NOT NULL) AS has_id_number,
       EXISTS (SELECT 1 FROM app.guest_id_photos gp WHERE gp.tenant_id = s.tenant_id AND gp.stay_id = s.id AND gp.side = 'FRONT') AS has_front_photo,
       EXISTS (SELECT 1 FROM app.guest_id_photos gb WHERE gb.tenant_id = s.tenant_id AND gb.stay_id = s.id AND gb.side = 'BACK') AS has_back_photo,
       s.deposit, s.check_in_at, s.check_out_at, s.rate_plan_snapshot
FROM app.stays s
JOIN app.units u ON u.tenant_id = s.tenant_id AND u.id = s.unit_id
WHERE s.tenant_id = @tenant_id AND s.id = @stay_id
FOR UPDATE OF s;

-- name: InsertStayExtra :exec
-- created_at comes from the argument (the server clock of the use case), never from now().
INSERT INTO app.stay_extras (id, tenant_id, stay_id, service_id, quantity, unit_amount, amount, created_at, created_by)
VALUES (@id, @tenant_id, @stay_id, @service_id, @quantity, @unit_amount, @amount, @created_at, sqlc.narg(created_by));

-- name: MarkStayCheckedOut :execrows
UPDATE app.stays SET status = 'CHECKED_OUT', check_out_at = @at
WHERE tenant_id = @tenant_id AND id = @stay_id AND status = 'ACTIVE';

-- name: GetInvoiceByStay :one
SELECT id, stay_id, bill_code, status, quote, total, created_at
FROM app.invoices
WHERE tenant_id = @tenant_id AND stay_id = @stay_id;

-- name: InsertInvoice :exec
-- status is OPEN and created_at comes from the argument, never from now().
INSERT INTO app.invoices (id, tenant_id, stay_id, bill_code, quote, total, status, created_at)
VALUES (@id, @tenant_id, @stay_id, @bill_code, @quote, @total, 'OPEN', @created_at);

-- name: BillCodeTaken :one
SELECT EXISTS (SELECT 1 FROM app.invoices WHERE tenant_id = @tenant_id AND bill_code = @bill_code);

-- name: GetStayPendingPayment :one
-- The open invoice of a stay with the money so far: paid is the deposit plus bank money that did not settle it yet.
SELECT iv.id AS invoice_id, iv.total, iv.created_at,
       coalesce((iv.quote->>'depositPaid')::bigint, 0)::bigint AS deposit,
       coalesce((iv.quote->>'refundDue')::bigint, 0)::bigint AS refund_due,
       coalesce((SELECT sum(pe.amount) FROM app.payment_events pe WHERE pe.tenant_id = iv.tenant_id AND pe.invoice_id = iv.id AND pe.result = 'PARTIAL'), 0)::bigint AS received,
       coalesce((SELECT p.id FROM app.payments p WHERE p.tenant_id = iv.tenant_id AND p.invoice_id = iv.id AND p.method = 'TRANSFER' AND p.status = 'PENDING'
                 ORDER BY p.created_at DESC LIMIT 1), '')::text AS payment_id
FROM app.invoices iv
WHERE iv.tenant_id = @tenant_id AND iv.stay_id = @stay_id AND iv.status = 'OPEN';
