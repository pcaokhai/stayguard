-- name: FirstRecordedCheckIn :one
SELECT old_check_in_at FROM app.stay_edits
WHERE tenant_id = @tenant_id AND stay_id = @stay_id AND kind = 'CHECK_IN'
ORDER BY created_at, id LIMIT 1;

-- name: UpdateStayCheckIn :execrows
UPDATE app.stays SET check_in_at = @check_in_at
WHERE tenant_id = @tenant_id AND id = @stay_id AND status = 'ACTIVE';

-- name: MoveStay :execrows
UPDATE app.stays SET unit_id = @unit_id, rental_type = @rental_type, rate_plan_snapshot = @rate_plan_snapshot,
       rate_plan_schema = @rate_plan_schema
WHERE tenant_id = @tenant_id AND id = @stay_id AND status = 'ACTIVE';

-- name: ReleaseRoom :execrows
UPDATE app.units SET status = 'TO_CLEAN'
WHERE tenant_id = @tenant_id AND id = @unit_id AND status = 'OCCUPIED';

-- name: InsertStayEdit :exec
INSERT INTO app.stay_edits (id, tenant_id, stay_id, kind, actor_id, old_check_in_at, new_check_in_at, reason_code, note,
                            from_room_code, to_room_code, created_at)
VALUES (@id, @tenant_id, @stay_id, @kind, sqlc.narg(actor_id), sqlc.narg(old_check_in_at), sqlc.narg(new_check_in_at),
        sqlc.narg(reason_code), sqlc.narg(note), sqlc.narg(from_room_code), sqlc.narg(to_room_code), @created_at);
