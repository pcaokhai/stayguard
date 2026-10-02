-- Every query filters by tenant explicitly; RLS is the second guard (ADR-005).

-- name: GetCheckInRoom :one
SELECT u.id, u.code, u.building_id, u.status, ut.rate_plan, ut.rate_plan_version
FROM app.units u
JOIN app.unit_types ut ON ut.tenant_id = u.tenant_id AND ut.id = u.unit_type_id
WHERE u.tenant_id = @tenant_id AND u.id = @unit_id AND NOT u.retired;

-- name: LockCheckInRoom :one
-- FOR UPDATE OF u serializes concurrent check-ins of one room; the unit type row stays unlocked.
SELECT u.id, u.code, u.building_id, u.status, ut.rate_plan, ut.rate_plan_version
FROM app.units u
JOIN app.unit_types ut ON ut.tenant_id = u.tenant_id AND ut.id = u.unit_type_id
WHERE u.tenant_id = @tenant_id AND u.id = @unit_id AND NOT u.retired
FOR UPDATE OF u;

-- name: InsertStay :exec
-- check_in_at comes from the argument (the server clock of the use case), never from now().
INSERT INTO app.stays (id, tenant_id, unit_id, rental_type, status, deposit, check_in_at,
                       rate_plan_snapshot, rate_plan_schema, guest_name, guest_phone, id_number_enc, created_by)
VALUES (@id, @tenant_id, @unit_id, @rental_type, 'ACTIVE', @deposit, @check_in_at,
        @rate_plan_snapshot, @rate_plan_schema, @guest_name, @guest_phone, @id_number_enc, sqlc.narg(created_by));

-- name: OccupyRoom :execrows
UPDATE app.units SET status = 'OCCUPIED'
WHERE tenant_id = @tenant_id AND id = @unit_id AND status = 'VACANT';

-- name: GetStayByID :one
SELECT s.id, s.unit_id, u.code AS room_code, u.building_id, s.rental_type, s.status, s.guest_name, s.guest_phone,
       s.id_number_enc, s.deposit, s.check_in_at, s.check_out_at, s.rate_plan_snapshot
FROM app.stays s
JOIN app.units u ON u.tenant_id = s.tenant_id AND u.id = s.unit_id
WHERE s.tenant_id = @tenant_id AND s.id = @stay_id;

-- name: ListStayExtras :many
SELECT sv.code AS service_code, sv.name AS service_name, e.quantity, e.unit_amount
FROM app.stay_extras e
JOIN app.services sv ON sv.tenant_id = e.tenant_id AND sv.id = e.service_id
WHERE e.tenant_id = @tenant_id AND e.stay_id = @stay_id
ORDER BY e.created_at, e.id;
