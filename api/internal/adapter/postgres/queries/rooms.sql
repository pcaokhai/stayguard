-- Every query filters by tenant explicitly; RLS is the second guard (ADR-005).

-- name: ListBuildings :many
SELECT id, code, name FROM app.buildings WHERE tenant_id = @tenant_id ORDER BY code, id;

-- name: ListRooms :many
-- One statement for the whole map; an empty filter argument means no filter.
SELECT u.id, u.code, u.building_id, f.id AS floor_id, f.name AS floor_name, f.level AS floor_level,
       ut.code AS unit_type_code, ut.name AS unit_type_name, u.status,
       coalesce(jsonb_typeof(u.attributes->'note') = 'string', false)::boolean AS has_note, coalesce(u.attributes->>'note', '')::text AS note,
       s.id AS stay_id, s.rental_type AS stay_rental_type, s.guest_name AS stay_guest_name,
       s.check_in_at AS stay_check_in_at, s.rate_plan_snapshot AS stay_rate_plan_snapshot
FROM app.units u
JOIN app.floors f ON f.tenant_id = u.tenant_id AND f.id = u.floor_id
JOIN app.unit_types ut ON ut.tenant_id = u.tenant_id AND ut.id = u.unit_type_id
LEFT JOIN app.stays s ON s.tenant_id = u.tenant_id AND s.unit_id = u.id AND s.status = 'ACTIVE'
WHERE u.tenant_id = @tenant_id AND NOT u.retired
  AND (@building_id::text = '' OR u.building_id = @building_id::text)
  AND (@unit_id::text = '' OR u.id = @unit_id::text)
ORDER BY u.building_id, u.code;

-- name: TenantTimezone :one
SELECT time_zone FROM app.tenants WHERE id = @tenant_id;

-- name: ListFloors :many
SELECT id, building_id, level, name FROM app.floors WHERE tenant_id = @tenant_id ORDER BY building_id, level, id;
