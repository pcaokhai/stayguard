-- Every query filters by tenant explicitly; RLS is the second guard (ADR-005).

-- name: InsertTrialTenant :exec
INSERT INTO app.tenants (id, name, is_trial, expires_at)
VALUES (@id, @name, true, @expires_at);

-- name: TrialTenantActive :one
SELECT EXISTS (
    SELECT 1 FROM app.tenants
    WHERE id = @tenant_id AND is_trial AND expires_at > @now
);

-- name: GetTenantInfo :one
SELECT id, name, time_zone, currency FROM app.tenants WHERE id = @tenant_id;

-- name: GetUserByRole :one
SELECT id, name, role, locale FROM app.users
WHERE tenant_id = @tenant_id AND role = @role
ORDER BY created_at, id
LIMIT 1;

-- name: GetUserByID :one
SELECT u.id, u.name, u.role, u.locale,
       (u.status <> 'ACTIVE' OR u.app_access = 'NONE')::boolean AS blocked,
       COALESCE(c.must_change, false)::boolean AS must_change_pin
FROM app.users u
LEFT JOIN app.pin_credentials c ON c.tenant_id = u.tenant_id AND c.user_id = u.id
WHERE u.tenant_id = @tenant_id AND u.id = @id;

-- name: InsertUser :one
INSERT INTO app.users (id, tenant_id, name, role, locale, app_access)
VALUES (@id, @tenant_id, @name, @role, @locale, @role)
RETURNING id, name, role, locale;

-- name: UpdateUserLocale :execrows
UPDATE app.users SET locale = @locale WHERE tenant_id = @tenant_id AND id = @id;

-- name: InsertSession :exec
INSERT INTO app.sessions (token_hash, tenant_id, user_id, expires_at)
VALUES (@token_hash, @tenant_id, @user_id, @expires_at);

-- name: ListBuildingIDs :many
SELECT id FROM app.buildings WHERE tenant_id = @tenant_id ORDER BY id;

-- name: GetSignInUser :one
SELECT u.id, u.name, u.role, u.locale, u.app_access, u.status,
       c.pin_hash, c.failed_count, c.first_failed_at, c.locked_until, c.must_change, c.one_time_expires_at
FROM app.users u
JOIN app.pin_credentials c ON c.tenant_id = u.tenant_id AND c.user_id = u.id
WHERE u.tenant_id = @tenant_id AND u.username = @username;

-- name: GetPinState :one
SELECT pin_hash, failed_count, first_failed_at, locked_until, must_change, one_time_expires_at
FROM app.pin_credentials WHERE tenant_id = @tenant_id AND user_id = @user_id
FOR UPDATE;

-- name: UpdatePinFailures :exec
UPDATE app.pin_credentials
SET failed_count = @failed_count, first_failed_at = sqlc.narg(first_failed_at), locked_until = sqlc.narg(locked_until)
WHERE tenant_id = @tenant_id AND user_id = @user_id;

-- name: UpsertPin :exec
INSERT INTO app.pin_credentials (tenant_id, user_id, pin_hash, must_change, one_time_expires_at, changed_at)
VALUES (@tenant_id, @user_id, @pin_hash, @must_change, sqlc.narg(one_time_expires_at), @now)
ON CONFLICT (tenant_id, user_id) DO UPDATE
SET pin_hash = EXCLUDED.pin_hash, must_change = EXCLUDED.must_change,
    one_time_expires_at = EXCLUDED.one_time_expires_at, changed_at = EXCLUDED.changed_at,
    failed_count = 0, first_failed_at = NULL, locked_until = NULL;

-- name: DeleteSession :exec
DELETE FROM app.sessions WHERE tenant_id = @tenant_id AND token_hash = @token_hash;

-- name: DeleteOtherUserSessions :exec
DELETE FROM app.sessions WHERE tenant_id = @tenant_id AND user_id = @user_id AND token_hash <> @keep_hash;

-- name: UpdatePinHash :exec
UPDATE app.pin_credentials SET pin_hash = @pin_hash WHERE tenant_id = @tenant_id AND user_id = @user_id;
