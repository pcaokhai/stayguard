-- name: InsertAlert :exec
INSERT INTO app.alerts (id, tenant_id, kind, room_code, shift_id, stay_id, actor_id, amount, details)
VALUES (@id, @tenant_id, @kind, sqlc.narg(room_code), sqlc.narg(shift_id), sqlc.narg(stay_id), sqlc.narg(actor_id),
        sqlc.narg(amount), @details);
