-- Every query filters by tenant explicitly; RLS is the second guard (ADR-005).

-- name: LockStayByID :one
-- Same projection as GetStayByID. FOR UPDATE OF s serializes extras and check-out on one stay.
SELECT s.id, s.unit_id, u.code AS room_code, u.building_id, s.rental_type, s.status, s.guest_name, s.guest_phone,
       s.id_number_enc, s.deposit, s.check_in_at, s.check_out_at, s.rate_plan_snapshot
FROM app.stays s
JOIN app.units u ON u.tenant_id = s.tenant_id AND u.id = s.unit_id
WHERE s.tenant_id = @tenant_id AND s.id = @stay_id
FOR UPDATE OF s;

-- name: InsertStayExtra :exec
-- created_at comes from the argument (the server clock of the use case), never from now().
INSERT INTO app.stay_extras (id, tenant_id, stay_id, service_id, quantity, unit_amount, amount, created_at)
VALUES (@id, @tenant_id, @stay_id, @service_id, @quantity, @unit_amount, @amount, @created_at);

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
