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
INSERT INTO app.cash_entries (id, tenant_id, shift_id, kind, amount, stay_id, payment_id, description, created_by, created_at)
VALUES (@id, @tenant_id, @shift_id, @kind, @amount, sqlc.narg(stay_id), sqlc.narg(payment_id), sqlc.narg(description),
        sqlc.narg(created_by), @created_at);

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
SELECT un.code AS room_code, st.rental_type, e.created_at, e.amount
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
