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
SELECT id, name, role, locale FROM app.users WHERE tenant_id = @tenant_id AND id = @id;

-- name: InsertUser :one
INSERT INTO app.users (id, tenant_id, name, role, locale)
VALUES (@id, @tenant_id, @name, @role, @locale)
RETURNING id, name, role, locale;

-- name: UpdateUserLocale :execrows
UPDATE app.users SET locale = @locale WHERE tenant_id = @tenant_id AND id = @id;

-- name: InsertSession :exec
INSERT INTO app.sessions (token_hash, tenant_id, user_id, expires_at)
VALUES (@token_hash, @tenant_id, @user_id, @expires_at);

-- name: ListBuildingIDs :many
SELECT id FROM app.buildings WHERE tenant_id = @tenant_id ORDER BY id;
