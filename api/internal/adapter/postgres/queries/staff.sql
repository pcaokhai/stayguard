-- Staff are users with a staff_profile; the owner has none and is never listed.

-- name: ListStaffRows :many
SELECT u.id, u.name, u.role, u.app_access, u.status, u.username,
       p.phone, p.position, p.pay_type, p.rate, p.fixed_allowance, p.standard_shifts, p.start_date, p.annual_leave_days,
       c.locked_until,
       (SELECT max(s.created_at) FROM app.sessions s WHERE s.tenant_id = u.tenant_id AND s.user_id = u.id)::timestamptz AS last_activity_at
FROM app.users u
JOIN app.staff_profiles p ON p.tenant_id = u.tenant_id AND p.user_id = u.id
LEFT JOIN app.pin_credentials c ON c.tenant_id = u.tenant_id AND c.user_id = u.id
WHERE u.tenant_id = @tenant_id
  AND (sqlc.narg(position)::text IS NULL OR p.position = sqlc.narg(position))
  AND (sqlc.narg(user_id)::text IS NULL OR u.id = sqlc.narg(user_id))
ORDER BY (u.status = 'REMOVED'), u.name, u.id;

-- name: InsertStaffUser :exec
INSERT INTO app.users (id, tenant_id, name, role, locale, app_access, username)
VALUES (@id, @tenant_id, @name, @role, 'vi', @app_access, sqlc.narg(username));

-- name: InsertStaffProfile :exec
INSERT INTO app.staff_profiles (tenant_id, user_id, phone, position, pay_type, rate, fixed_allowance, standard_shifts, start_date, annual_leave_days)
VALUES (@tenant_id, @user_id, sqlc.narg(phone), @position, @pay_type, @rate, @fixed_allowance, @standard_shifts, @start_date, @annual_leave_days);

-- name: UpdateStaffUser :execrows
UPDATE app.users
SET name = COALESCE(sqlc.narg(name), name), role = COALESCE(sqlc.narg(role), role), app_access = COALESCE(sqlc.narg(app_access), app_access)
WHERE users.tenant_id = @tenant_id AND users.id = @id AND users.status <> 'REMOVED'
  AND EXISTS (SELECT 1 FROM app.staff_profiles p WHERE p.tenant_id = users.tenant_id AND p.user_id = users.id);

-- name: UpdateStaffProfile :exec
UPDATE app.staff_profiles
SET phone = COALESCE(sqlc.narg(phone), phone), position = COALESCE(sqlc.narg(position), position),
    pay_type = COALESCE(sqlc.narg(pay_type), pay_type), rate = COALESCE(sqlc.narg(rate), rate),
    fixed_allowance = COALESCE(sqlc.narg(fixed_allowance), fixed_allowance),
    standard_shifts = COALESCE(sqlc.narg(standard_shifts), standard_shifts),
    start_date = COALESCE(sqlc.narg(start_date), start_date),
    annual_leave_days = COALESCE(sqlc.narg(annual_leave_days), annual_leave_days)
WHERE tenant_id = @tenant_id AND user_id = @user_id;

-- name: SetStaffStatus :execrows
UPDATE app.users SET status = @status
WHERE users.tenant_id = @tenant_id AND users.id = @id AND users.status <> 'REMOVED'
  AND EXISTS (SELECT 1 FROM app.staff_profiles p WHERE p.tenant_id = users.tenant_id AND p.user_id = users.id);

-- name: DeleteUserSessions :exec
DELETE FROM app.sessions WHERE tenant_id = @tenant_id AND user_id = @user_id;

-- name: ListUserBuildingLevels :many
SELECT building_id, level FROM app.building_permissions WHERE tenant_id = @tenant_id AND user_id = @user_id;

-- name: ListStaffBuildingLevels :many
SELECT user_id, building_id, level FROM app.building_permissions WHERE tenant_id = @tenant_id ORDER BY user_id, building_id;

-- name: GetBuildingPermission :one
SELECT level FROM app.building_permissions WHERE tenant_id = @tenant_id AND user_id = @user_id AND building_id = @building_id;

-- name: UpsertBuildingPermission :exec
INSERT INTO app.building_permissions (tenant_id, user_id, building_id, level, updated_at)
VALUES (@tenant_id, @user_id, @building_id, @level, @now)
ON CONFLICT (tenant_id, user_id, building_id) DO UPDATE SET level = EXCLUDED.level, updated_at = EXCLUDED.updated_at;

-- name: GrantAllBuildings :exec
INSERT INTO app.building_permissions (tenant_id, user_id, building_id, level)
SELECT b.tenant_id, @user_id, b.id, @level FROM app.buildings b WHERE b.tenant_id = @tenant_id
ON CONFLICT (tenant_id, user_id, building_id) DO NOTHING;

-- name: BuildingExists :one
SELECT EXISTS (SELECT 1 FROM app.buildings WHERE tenant_id = @tenant_id AND id = @id);

-- name: GetUserRoleStatus :one
SELECT role, status, app_access, name FROM app.users WHERE tenant_id = @tenant_id AND id = @id;

-- name: ListPermissionUsers :many
SELECT id, name, role FROM app.users
WHERE tenant_id = @tenant_id AND status <> 'REMOVED' AND app_access <> 'NONE' AND role <> 'OWNER'
ORDER BY name, id;
