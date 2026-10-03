-- name: LockOpenShift :one
SELECT s.id, s.user_id, u.name AS user_name, s.status, s.shift_code, s.opened_at, s.closed_at, s.opening_float
FROM app.shifts s JOIN app.users u ON u.tenant_id = s.tenant_id AND u.id = s.user_id
WHERE s.tenant_id = @tenant_id AND s.user_id = @user_id AND s.status = 'OPEN'
FOR UPDATE OF s;

-- name: InsertShift :execrows
-- A race between two first cash actions of one user ends in DO NOTHING for the loser, who then reads the winner's shift.
INSERT INTO app.shifts (id, tenant_id, user_id, shift_code, opened_at, opening_float)
VALUES (@id, @tenant_id, @user_id, @shift_code, @opened_at, @opening_float)
ON CONFLICT (tenant_id, user_id) WHERE status = 'OPEN' DO NOTHING;

-- name: LastFloatLeft :one
-- The float left in the drawer by the shift closed last, which the next shift opens with.
SELECT float_left FROM app.shifts
WHERE tenant_id = @tenant_id AND status = 'CLOSED'
ORDER BY closed_at DESC, id DESC LIMIT 1;

-- name: InsertCashEntry :exec
INSERT INTO app.cash_entries (id, tenant_id, shift_id, kind, amount, stay_id, payment_id, description, created_by, created_at, by_owner)
VALUES (@id, @tenant_id, sqlc.narg(shift_id), @kind, @amount, sqlc.narg(stay_id), sqlc.narg(payment_id), sqlc.narg(description),
        sqlc.narg(created_by), @created_at, @by_owner);

-- name: ShiftCash :one
SELECT coalesce(sum(amount) FILTER (WHERE kind IN ('DEPOSIT', 'PAYMENT')), 0)::bigint AS cash_in,
       coalesce(sum(amount) FILTER (WHERE kind IN ('REFUND', 'PAYOUT')), 0)::bigint AS cash_out
FROM app.cash_entries WHERE tenant_id = @tenant_id AND shift_id = @shift_id;

-- name: TransfersBetween :one
SELECT coalesce(sum(coalesce(received_amount, amount)), 0)::bigint AS total
FROM app.payments
WHERE tenant_id = @tenant_id AND method = 'TRANSFER' AND status = 'PAID' AND paid_at >= @from_at AND paid_at < @to_at;

-- name: CloseShift :execrows
UPDATE app.shifts SET status = 'CLOSED', closed_at = @closed_at, expected_cash = @expected_cash, counted_cash = @counted_cash,
       difference = @difference, reason = sqlc.narg(reason), reason_recorded_at = sqlc.narg(reason_recorded_at),
       float_left = @float_left, handover_to_user_id = sqlc.narg(handover_to_user_id), counts = @counts
WHERE tenant_id = @tenant_id AND id = @shift_id AND status = 'OPEN';

-- name: GetShiftByID :one
SELECT s.id, s.user_id, u.name AS user_name, s.status, s.shift_code, s.opened_at, s.closed_at, s.opening_float,
       s.expected_cash, s.counted_cash, s.difference, s.reason, s.reason_recorded_at, s.float_left
FROM app.shifts s JOIN app.users u ON u.tenant_id = s.tenant_id AND u.id = s.user_id
WHERE s.tenant_id = @tenant_id AND s.id = @shift_id;

-- name: ListShiftCashIn :many
SELECT un.code AS room_code, st.rental_type, e.created_at, e.amount, e.by_owner
FROM app.cash_entries e
JOIN app.stays st ON st.tenant_id = e.tenant_id AND st.id = e.stay_id
JOIN app.units un ON un.tenant_id = st.tenant_id AND un.id = st.unit_id
WHERE e.tenant_id = @tenant_id AND e.shift_id = @shift_id AND e.kind IN ('DEPOSIT', 'PAYMENT')
ORDER BY e.created_at, e.id;

-- name: StaffMonthStats :one
SELECT count(*)::bigint AS shifts_with_difference,
       coalesce(sum(-difference) FILTER (WHERE difference < 0), 0)::bigint AS total_short
FROM app.shifts
WHERE tenant_id = @tenant_id AND user_id = @user_id AND status = 'CLOSED' AND difference <> 0
  AND closed_at >= @from_at AND closed_at < @to_at;

-- name: ListClosedShifts :many
SELECT s.id, u.name AS user_name, s.shift_code, s.opened_at, s.closed_at, s.difference
FROM app.shifts s JOIN app.users u ON u.tenant_id = s.tenant_id AND u.id = s.user_id
WHERE s.tenant_id = @tenant_id AND s.status = 'CLOSED'
  AND (sqlc.narg(from_at)::timestamptz IS NULL OR (s.closed_at >= sqlc.narg(from_at)::timestamptz AND s.closed_at < sqlc.narg(to_at)::timestamptz))
  AND (sqlc.narg(user_id)::text IS NULL OR s.user_id = sqlc.narg(user_id)::text)
  AND (NOT @only_differences::boolean OR s.difference <> 0)
  AND (sqlc.narg(cursor_at)::timestamptz IS NULL OR (s.closed_at, s.id) < (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::text))
ORDER BY s.closed_at DESC, s.id DESC
LIMIT @row_limit;

-- name: ListShiftUnpaidInvoices :many
-- Invoices not fully paid (including an open deposit refund) that belong to a shift: checked out during it, or with bank money that arrived during it.
-- Same figures as ListUnpaidInvoices (the owner's list): paid is the deposit plus reported bank money.
SELECT iv.id, iv.bill_code, un.code AS room_code, s.guest_name, s.check_out_at, iv.total,
       least(coalesce((iv.quote->>'depositPaid')::bigint, 0), iv.total)::bigint AS deposit,
       coalesce((iv.quote->>'refundDue')::bigint, 0)::bigint AS refund_due,
       (coalesce((SELECT sum(coalesce(p.received_amount, 0)) FROM app.payments p
                  WHERE p.tenant_id = iv.tenant_id AND p.invoice_id = iv.id AND p.status = 'MISMATCH'), 0)
        + coalesce((SELECT sum(pe.amount) FROM app.payment_events pe
                    WHERE pe.tenant_id = iv.tenant_id AND pe.invoice_id = iv.id AND pe.result = 'PARTIAL'), 0))::bigint AS reported
FROM app.invoices iv
JOIN app.stays s ON s.tenant_id = iv.tenant_id AND s.id = iv.stay_id
JOIN app.units un ON un.tenant_id = s.tenant_id AND un.id = s.unit_id
WHERE iv.tenant_id = @tenant_id AND iv.status = 'OPEN' AND s.check_out_at IS NOT NULL
  AND ((iv.created_at >= @from_at AND iv.created_at <= @to_at)
       OR EXISTS (SELECT 1 FROM app.payment_events pe WHERE pe.tenant_id = iv.tenant_id AND pe.invoice_id = iv.id
                  AND pe.result = 'PARTIAL' AND pe.received_at >= @from_at AND pe.received_at <= @to_at))
ORDER BY s.check_out_at DESC, iv.id DESC
LIMIT 200;

-- name: LockOpenShiftForStay :one
-- The open shift whose person can edit the building of the stay's room (the latest opened): where cash moved by the owner or a manager
-- for that stay belongs, because it leaves the same drawer.
SELECT s.id, s.user_id, u.name AS user_name, s.status, s.shift_code, s.opened_at, s.closed_at, s.opening_float
FROM app.shifts s JOIN app.users u ON u.tenant_id = s.tenant_id AND u.id = s.user_id
WHERE s.tenant_id = @tenant_id AND s.status = 'OPEN'
  AND EXISTS (SELECT 1 FROM app.stays st
              JOIN app.units un ON un.tenant_id = st.tenant_id AND un.id = st.unit_id
              JOIN app.building_permissions bp ON bp.tenant_id = s.tenant_id AND bp.user_id = s.user_id AND bp.building_id = un.building_id AND bp.level = 'EDIT'
              WHERE st.tenant_id = s.tenant_id AND st.id = @stay_id::text)
ORDER BY s.opened_at DESC, s.id DESC
LIMIT 1
FOR UPDATE OF s;

-- name: OwnerCashShifts :many
-- Shifts open at any time in [from, to) with their whole ledger; expected cash is worked out in Go by the same function the shift screen uses.
SELECT s.opening_float,
       coalesce(sum(e.amount) FILTER (WHERE e.kind IN ('DEPOSIT', 'PAYMENT')), 0)::bigint AS cash_in,
       coalesce(sum(e.amount) FILTER (WHERE e.kind IN ('REFUND', 'PAYOUT')), 0)::bigint AS cash_out
FROM app.shifts s
LEFT JOIN app.cash_entries e ON e.tenant_id = s.tenant_id AND e.shift_id = s.id
WHERE s.tenant_id = @tenant_id AND s.opened_at < @to_at AND (s.closed_at IS NULL OR s.closed_at >= @from_at)
GROUP BY s.id, s.opening_float;

-- name: OwnerCashNoShift :one
-- Owner cash of the day: ledger lines with no shift.
SELECT coalesce(sum(amount) FILTER (WHERE kind IN ('DEPOSIT', 'PAYMENT')), 0)::bigint AS cash_in,
       coalesce(sum(amount) FILTER (WHERE kind IN ('REFUND', 'PAYOUT')), 0)::bigint AS cash_out
FROM app.cash_entries
WHERE tenant_id = @tenant_id AND shift_id IS NULL AND created_at >= @from_at AND created_at < @to_at;
