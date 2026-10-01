-- Every query filters by tenant explicitly; RLS is the second guard (ADR-005).

-- name: OwnerRevenueByBuilding :many
-- PAID payments with paid_at in [from, to), per building; buildings without payments are listed with zeros.
SELECT b.id, b.name,
       COALESCE(SUM(p.amount) FILTER (WHERE p.method = 'CASH'), 0)::bigint AS cash,
       COALESCE(SUM(p.amount) FILTER (WHERE p.method = 'TRANSFER'), 0)::bigint AS transfer
FROM app.buildings b
LEFT JOIN app.units u ON u.tenant_id = b.tenant_id AND u.building_id = b.id
LEFT JOIN app.stays s ON s.tenant_id = u.tenant_id AND s.unit_id = u.id
LEFT JOIN app.invoices i ON i.tenant_id = s.tenant_id AND i.stay_id = s.id
LEFT JOIN app.payments p ON p.tenant_id = i.tenant_id AND p.invoice_id = i.id AND p.status = 'PAID'
                        AND p.paid_at >= @from_at AND p.paid_at < @to_at
WHERE b.tenant_id = @tenant_id
GROUP BY b.id, b.name, b.code
ORDER BY b.code;

-- name: OwnerLatestPayments :many
SELECT p.id, u.code AS room_code, p.method, p.amount, p.paid_at
FROM app.payments p
JOIN app.invoices i ON i.tenant_id = p.tenant_id AND i.id = p.invoice_id
JOIN app.stays s ON s.tenant_id = i.tenant_id AND s.id = i.stay_id
JOIN app.units u ON u.tenant_id = s.tenant_id AND u.id = s.unit_id
WHERE p.tenant_id = @tenant_id AND p.status = 'PAID'
ORDER BY p.paid_at DESC, p.id DESC
LIMIT @max_rows;
