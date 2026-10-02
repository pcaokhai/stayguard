-- Every query filters by tenant explicitly; RLS is the second guard (ADR-005).

-- name: ListServices :many
-- The front desk sees items on sale; the owner and manager see everything. soldLast7Days counts extras added in the window.
SELECT s.id, s.code, s.name, s.price, s.stock, s.unit, s.low_stock_at, s.on_sale, s.latest_unit_cost,
       COALESCE((SELECT sum(x.quantity) FROM app.stay_extras x
                 WHERE x.tenant_id = s.tenant_id AND x.service_id = s.id AND x.created_at >= @since), 0)::bigint AS sold_recent
FROM app.services s
WHERE s.tenant_id = @tenant_id AND (s.on_sale OR @include_off_sale::boolean)
ORDER BY s.code;

-- name: ListServicesByCodes :many
SELECT id, code, name, price, stock
FROM app.services
WHERE tenant_id = @tenant_id AND on_sale AND code = ANY(@codes::text[])
ORDER BY code;

-- name: DecrementServiceStock :one
-- The guard is in the WHERE: zero rows means not enough stock, never a read followed by a write.
UPDATE app.services SET stock = stock - @qty
WHERE tenant_id = @tenant_id AND id = @service_id AND stock >= @qty
RETURNING price;
