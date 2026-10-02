-- Setup of buildings, rooms, rates and items (L-A4). Every query filters by tenant; RLS is the second guard.

-- name: FirstPropertyID :one
SELECT id FROM app.properties WHERE tenant_id = @tenant_id ORDER BY created_at, id LIMIT 1;

-- name: GetBuilding :one
SELECT id, code, name FROM app.buildings WHERE tenant_id = @tenant_id AND id = @id FOR UPDATE;

-- name: UpdateBuildingName :exec
UPDATE app.buildings SET name = @name WHERE tenant_id = @tenant_id AND id = @id;

-- name: BuildingStatusCounts :many
SELECT status, count(*)::int AS n FROM app.units
WHERE tenant_id = @tenant_id AND building_id = @building_id AND NOT retired GROUP BY status;

-- name: GetFloor :one
SELECT id, building_id, level FROM app.floors WHERE tenant_id = @tenant_id AND id = @id;

-- name: MaxFloorLevel :one
SELECT COALESCE(max(level), 0)::int FROM app.floors WHERE tenant_id = @tenant_id AND building_id = @building_id;

-- name: ExistingRoomCodes :many
SELECT code FROM app.units WHERE tenant_id = @tenant_id AND code = ANY(@codes::text[]);

-- name: InsertRoom :exec
INSERT INTO app.units (id, tenant_id, building_id, floor_id, unit_type_id, code, status, attributes)
VALUES (@id, @tenant_id, @building_id, @floor_id, @unit_type_id, @code, @status, @attributes);

-- name: GetUnitTypeByCode :one
SELECT id, code, name, rate_plan, rate_plan_version, updated_at FROM app.unit_types
WHERE tenant_id = @tenant_id AND code = @code;

-- name: ListUnitTypes :many
SELECT id, code, name, rate_plan, rate_plan_version, updated_at FROM app.unit_types
WHERE tenant_id = @tenant_id ORDER BY code;

-- name: UpdateUnitTypeRatePlan :exec
UPDATE app.unit_types SET rate_plan = @rate_plan, rate_plan_version = @rate_plan_version, updated_at = @now
WHERE tenant_id = @tenant_id AND id = @id;

-- name: LockRoomForSetup :one
SELECT u.id, u.code, u.building_id, u.floor_id, u.unit_type_id, u.status, u.retired, u.attributes, f.level AS floor_level,
       ut.code AS unit_type_code, ut.name AS unit_type_name,
       EXISTS (SELECT 1 FROM app.stays s WHERE s.tenant_id = u.tenant_id AND s.unit_id = u.id AND s.status = 'ACTIVE') AS has_guest
FROM app.units u
JOIN app.floors f ON f.tenant_id = u.tenant_id AND f.id = u.floor_id
JOIN app.unit_types ut ON ut.tenant_id = u.tenant_id AND ut.id = u.unit_type_id
WHERE u.tenant_id = @tenant_id AND u.id = @id
FOR UPDATE OF u;

-- name: UpdateRoomRow :exec
UPDATE app.units SET code = @code, unit_type_id = @unit_type_id, status = @status, retired = @retired, attributes = @attributes
WHERE tenant_id = @tenant_id AND id = @id;

-- name: GetServiceForUpdate :one
SELECT id, code, name, price, stock, unit, low_stock_at, on_sale, latest_unit_cost FROM app.services
WHERE tenant_id = @tenant_id AND code = @code FOR UPDATE;

-- name: ServiceCodeTaken :one
SELECT EXISTS (SELECT 1 FROM app.services WHERE tenant_id = @tenant_id AND code = @code);

-- name: InsertServiceItem :exec
INSERT INTO app.services (id, tenant_id, code, name, price, stock, unit, low_stock_at, on_sale, latest_unit_cost)
VALUES (@id, @tenant_id, @code, @name, @price, @stock, @unit, @low_stock_at, @on_sale, sqlc.narg(latest_unit_cost));

-- name: UpdateServiceItem :exec
UPDATE app.services
SET name = COALESCE(sqlc.narg(name)::jsonb, name), price = COALESCE(sqlc.narg(price), price), unit = COALESCE(sqlc.narg(unit), unit),
    low_stock_at = COALESCE(sqlc.narg(low_stock_at), low_stock_at), on_sale = COALESCE(sqlc.narg(on_sale), on_sale)
WHERE tenant_id = @tenant_id AND id = @id;

-- name: InsertStockMovement :exec
INSERT INTO app.stock_movements (id, tenant_id, service_id, kind, quantity, unit_cost, ref, actor_id, created_at)
VALUES (@id, @tenant_id, @service_id, @kind, @quantity, sqlc.narg(unit_cost), sqlc.narg(ref), sqlc.narg(actor_id), @created_at);

-- name: AddServiceStock :one
-- Stock changes only here and in DecrementServiceStock, each with a movement row beside it.
UPDATE app.services SET stock = stock + @qty, latest_unit_cost = COALESCE(sqlc.narg(unit_cost), latest_unit_cost)
WHERE tenant_id = @tenant_id AND id = @id RETURNING stock;
