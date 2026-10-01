-- Every query filters by tenant explicitly; RLS is the second guard (ADR-005).
-- A task is a room in TO_CLEAN; its id is the room id. createdAt is the last check-out, or the tenant's creation.

-- name: ListToCleanRooms :many
SELECT u.id, u.code, u.building_id,
       COALESCE((SELECT max(s.check_out_at) FROM app.stays s WHERE s.tenant_id = u.tenant_id AND s.unit_id = u.id),
                t.created_at)::timestamptz AS cleaning_since
FROM app.units u
JOIN app.tenants t ON t.id = u.tenant_id
WHERE u.tenant_id = @tenant_id AND u.status = 'TO_CLEAN'
ORDER BY cleaning_since, u.code;

-- name: LockRoomForClean :one
SELECT u.id, u.code, u.building_id, u.status,
       COALESCE((SELECT max(s.check_out_at) FROM app.stays s WHERE s.tenant_id = u.tenant_id AND s.unit_id = u.id),
                t.created_at)::timestamptz AS cleaning_since
FROM app.units u
JOIN app.tenants t ON t.id = u.tenant_id
WHERE u.tenant_id = @tenant_id AND u.id = @unit_id
FOR UPDATE OF u;

-- name: MarkRoomClean :execrows
UPDATE app.units SET status = 'VACANT'
WHERE tenant_id = @tenant_id AND id = @unit_id AND status = 'TO_CLEAN';
