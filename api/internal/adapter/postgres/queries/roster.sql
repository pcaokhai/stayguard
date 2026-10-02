-- name: ListRosterAssignments :many
SELECT r.user_id, u.name AS user_name, r.work_date, r.shift FROM app.roster_assignments r
JOIN app.users u ON u.tenant_id = r.tenant_id AND u.id = r.user_id
WHERE r.tenant_id = @tenant_id AND r.work_date >= @from_date AND r.work_date <= @to_date
ORDER BY r.work_date, r.shift, r.user_id;

-- name: ListActiveUserIDs :many
-- Who a roster may use: everyone not removed, including staff with no sign-in.
SELECT id FROM app.users WHERE tenant_id = @tenant_id AND id = ANY(@user_ids::text[]) AND status <> 'REMOVED';

-- name: InsertRosterAssignment :execrows
INSERT INTO app.roster_assignments (tenant_id, user_id, work_date, shift, created_by)
VALUES (@tenant_id, @user_id, @work_date, @shift, sqlc.narg(created_by))
ON CONFLICT DO NOTHING;

-- name: DeleteRosterAssignment :execrows
DELETE FROM app.roster_assignments WHERE tenant_id = @tenant_id AND user_id = @user_id AND work_date = @work_date AND shift = @shift;

-- name: CountRosterBetween :one
SELECT count(*) FROM app.roster_assignments WHERE tenant_id = @tenant_id AND work_date >= @from_date AND work_date <= @to_date;

-- name: CopyRosterWeek :execrows
-- Seven days from @src_from, moved forward a week; people removed since are not copied.
INSERT INTO app.roster_assignments (tenant_id, user_id, work_date, shift, created_by)
SELECT r.tenant_id, r.user_id, r.work_date + 7, r.shift, sqlc.narg(created_by)
FROM app.roster_assignments r
JOIN app.users u ON u.tenant_id = r.tenant_id AND u.id = r.user_id AND u.status <> 'REMOVED'
WHERE r.tenant_id = @tenant_id AND r.work_date >= sqlc.arg(src_from)::date AND r.work_date < sqlc.arg(src_from)::date + 7
ON CONFLICT DO NOTHING;

-- name: ListScheduledShifts :many
-- The shifts a person is rostered for on a day; a NIGHT shift started the evening before still counts after midnight.
SELECT shift FROM app.roster_assignments
WHERE tenant_id = @tenant_id AND user_id = @user_id
  AND ((work_date = sqlc.arg(day)::date AND shift <> 'NIGHT') OR (work_date = sqlc.arg(day)::date - 1 AND shift = 'NIGHT') OR (work_date = sqlc.arg(day)::date AND shift = 'NIGHT'))
ORDER BY CASE shift WHEN 'MORNING' THEN 1 WHEN 'AFTERNOON' THEN 2 ELSE 3 END;

-- name: InsertLeave :exec
INSERT INTO app.leave_requests (id, tenant_id, user_id, from_date, to_date, shift, kind, reason, cover_user_id, created_at)
VALUES (@id, @tenant_id, @user_id, @from_date, @to_date, sqlc.narg(shift), @kind, sqlc.narg(reason), sqlc.narg(cover_user_id), @created_at);

-- name: GetLeave :one
SELECT l.id, l.user_id, u.name AS user_name, l.from_date, l.to_date, coalesce(l.shift, '') AS shift, l.kind, coalesce(l.reason, '') AS reason,
       coalesce(l.cover_user_id, '') AS cover_user_id, l.status, coalesce(l.decline_reason, '') AS decline_reason, l.created_at, l.decided_at
FROM app.leave_requests l JOIN app.users u ON u.tenant_id = l.tenant_id AND u.id = l.user_id
WHERE l.tenant_id = @tenant_id AND l.id = @leave_id;

-- name: LockLeave :one
SELECT id FROM app.leave_requests WHERE tenant_id = @tenant_id AND id = @leave_id FOR UPDATE;

-- name: ListLeave :many
-- Filters are all optional: a status, a person, and the days a request must touch.
SELECT l.id, l.user_id, u.name AS user_name, l.from_date, l.to_date, coalesce(l.shift, '') AS shift, l.kind, coalesce(l.reason, '') AS reason,
       coalesce(l.cover_user_id, '') AS cover_user_id, l.status, coalesce(l.decline_reason, '') AS decline_reason, l.created_at, l.decided_at
FROM app.leave_requests l JOIN app.users u ON u.tenant_id = l.tenant_id AND u.id = l.user_id
WHERE l.tenant_id = @tenant_id
  AND (sqlc.narg(status)::text IS NULL OR l.status = sqlc.narg(status)::text)
  AND (sqlc.narg(user_id)::text IS NULL OR l.user_id = sqlc.narg(user_id)::text)
  AND (sqlc.narg(from_date)::date IS NULL OR (l.to_date >= sqlc.narg(from_date)::date AND l.from_date <= sqlc.narg(to_date)::date))
  AND (NOT @standing_only::boolean OR l.status IN ('PENDING', 'APPROVED', 'CANCEL_REQUESTED'))
ORDER BY l.from_date DESC, l.created_at DESC, l.id DESC;

-- name: SetLeaveStatus :exec
UPDATE app.leave_requests SET status = @status, decline_reason = sqlc.narg(decline_reason), decided_at = @decided_at, decided_by = @decided_by
WHERE tenant_id = @tenant_id AND id = @leave_id;

-- name: CountOverlappingLeave :one
SELECT count(*) FROM app.leave_requests
WHERE tenant_id = @tenant_id AND user_id = @user_id AND status IN ('PENDING', 'APPROVED', 'CANCEL_REQUESTED')
  AND to_date >= @from_date AND from_date <= @to_date;

-- name: SumPaidLeaveDays :one
-- Days of paid leave that stand (approved, or approved with a cancel pending) inside the year.
SELECT coalesce(sum(least(to_date, @year_end::date) - greatest(from_date, @year_start::date) + 1), 0)::bigint AS days
FROM app.leave_requests
WHERE tenant_id = @tenant_id AND user_id = @user_id AND kind = 'PAID' AND status IN ('APPROVED', 'CANCEL_REQUESTED')
  AND to_date >= @year_start::date AND from_date <= @year_end::date;

-- name: GetAnnualLeaveDays :one
SELECT annual_leave_days FROM app.staff_profiles WHERE tenant_id = @tenant_id AND user_id = @user_id;

-- name: ListPendingLeave :many
SELECT l.id, u.name AS user_name FROM app.leave_requests l JOIN app.users u ON u.tenant_id = l.tenant_id AND u.id = l.user_id
WHERE l.tenant_id = @tenant_id AND l.status = 'PENDING' ORDER BY l.created_at, l.id;
