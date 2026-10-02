-- name: NextTicketSeq :one
-- The counter row is locked by the update, so two reports at once get two numbers.
UPDATE app.tenants SET ticket_seq = ticket_seq + 1 WHERE id = @tenant_id RETURNING ticket_seq;

-- name: InsertTicket :exec
INSERT INTO app.maintenance_tickets (id, tenant_id, code, unit_id, category, description, room_locked, reported_by, reported_at)
VALUES (@id, @tenant_id, @code, @unit_id, @category, @description, @room_locked, sqlc.narg(reported_by), @reported_at);

-- name: GetTicket :one
SELECT t.id, t.code, t.unit_id, u.code AS room_code, u.building_id, u.status AS room_status, t.category, t.description, t.status, t.room_locked,
       coalesce(rb.name, '') AS reported_by, t.reported_at, t.expected_done_on, t.parts_cost, t.labour_cost, t.repairer, t.note, t.completed_at
FROM app.maintenance_tickets t
JOIN app.units u ON u.tenant_id = t.tenant_id AND u.id = t.unit_id
LEFT JOIN app.users rb ON rb.tenant_id = t.tenant_id AND rb.id = t.reported_by
WHERE t.tenant_id = @tenant_id AND t.id = @ticket_id;

-- name: LockTicket :one
SELECT t.id FROM app.maintenance_tickets t WHERE t.tenant_id = @tenant_id AND t.id = @ticket_id FOR UPDATE;

-- name: ListTickets :many
SELECT t.id, t.code, t.unit_id, u.code AS room_code, u.building_id, u.status AS room_status, t.category, t.description, t.status, t.room_locked,
       coalesce(rb.name, '') AS reported_by, t.reported_at, t.expected_done_on, t.parts_cost, t.labour_cost, t.repairer, t.note, t.completed_at
FROM app.maintenance_tickets t
JOIN app.units u ON u.tenant_id = t.tenant_id AND u.id = t.unit_id
LEFT JOIN app.users rb ON rb.tenant_id = t.tenant_id AND rb.id = t.reported_by
WHERE t.tenant_id = @tenant_id AND (sqlc.narg(status)::text IS NULL OR t.status = sqlc.narg(status)::text)
ORDER BY t.reported_at DESC, t.id DESC;

-- name: UpdateTicket :exec
UPDATE app.maintenance_tickets SET status = @status, room_locked = @room_locked, expected_done_on = sqlc.narg(expected_done_on),
       parts_cost = sqlc.narg(parts_cost), labour_cost = sqlc.narg(labour_cost), repairer = sqlc.narg(repairer), note = sqlc.narg(note),
       completed_at = sqlc.narg(completed_at), completed_by = sqlc.narg(completed_by)
WHERE tenant_id = @tenant_id AND id = @ticket_id;

-- name: LockRoomForMaintenance :one
SELECT u.id, u.code, u.building_id, u.status FROM app.units u WHERE u.tenant_id = @tenant_id AND u.id = @unit_id FOR UPDATE OF u;

-- name: SetRoomMaintenance :execrows
UPDATE app.units SET status = 'MAINTENANCE' WHERE tenant_id = @tenant_id AND id = @unit_id AND status IN ('VACANT', 'TO_CLEAN', 'MAINTENANCE');

-- name: ReopenRoom :execrows
UPDATE app.units SET status = 'VACANT' WHERE tenant_id = @tenant_id AND id = @unit_id AND status = 'MAINTENANCE';

-- name: CountOtherLockingTickets :one
SELECT count(*) FROM app.maintenance_tickets
WHERE tenant_id = @tenant_id AND unit_id = @unit_id AND id <> @ticket_id AND room_locked AND status <> 'DONE';

-- name: ListOpenTickets :many
SELECT t.id, u.code AS room_code FROM app.maintenance_tickets t
JOIN app.units u ON u.tenant_id = t.tenant_id AND u.id = t.unit_id
WHERE t.tenant_id = @tenant_id AND t.status <> 'DONE'
ORDER BY t.reported_at, t.id;
