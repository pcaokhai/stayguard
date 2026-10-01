-- Every query filters by tenant explicitly; RLS is the second guard (ADR-005).

-- name: SetTenantBankAccount :exec
UPDATE app.tenants SET bank_account_enc = @bank_account_enc WHERE id = @tenant_id;

-- name: InsertProperty :exec
INSERT INTO app.properties (id, tenant_id, name) VALUES (@id, @tenant_id, @name);

-- name: InsertBuilding :exec
INSERT INTO app.buildings (id, tenant_id, property_id, code, name)
VALUES (@id, @tenant_id, @property_id, @code, @name);

-- name: InsertFloor :exec
INSERT INTO app.floors (id, tenant_id, building_id, level) VALUES (@id, @tenant_id, @building_id, @level);

-- name: InsertUnitType :exec
INSERT INTO app.unit_types (id, tenant_id, code, name, rate_plan, rate_plan_version)
VALUES (@id, @tenant_id, @code, @name, @rate_plan, @rate_plan_version);

-- name: InsertUnit :exec
INSERT INTO app.units (id, tenant_id, building_id, floor_id, unit_type_id, code, status)
VALUES (@id, @tenant_id, @building_id, @floor_id, @unit_type_id, @code, @status);

-- name: InsertService :exec
INSERT INTO app.services (id, tenant_id, code, name, price, stock)
VALUES (@id, @tenant_id, @code, @name, @price, @stock);
