-- Every query filters by tenant explicitly; RLS is the second guard (ADR-005).

-- name: ListServices :many
SELECT id, code, name, price, stock
FROM app.services
WHERE tenant_id = @tenant_id
ORDER BY code;

-- name: ListServicesByCodes :many
SELECT id, code, name, price, stock
FROM app.services
WHERE tenant_id = @tenant_id AND code = ANY(@codes::text[])
ORDER BY code;

-- name: DecrementServiceStock :one
-- The guard is in the WHERE: zero rows means not enough stock, never a read followed by a write.
UPDATE app.services SET stock = stock - @qty
WHERE tenant_id = @tenant_id AND id = @service_id AND stock >= @qty
RETURNING price;
