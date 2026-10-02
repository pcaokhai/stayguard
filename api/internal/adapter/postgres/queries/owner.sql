-- Every query filters by tenant explicitly; RLS is the second guard (ADR-005).

-- name: OwnerRevenueByBuilding :many
-- Revenue is what invoices paid in [from, to) were for, like the income and cost report: the deposit (taken in cash at
-- check-in) counts as cash, and the rest by the method of the payment that settled the invoice. Buildings without
-- invoices are listed with zeros.
SELECT b.id, b.name,
       COALESCE(SUM(least(coalesce((iv.quote->>'depositPaid')::bigint, 0), iv.total)
                    + CASE WHEN p.method = 'CASH' THEN coalesce((iv.quote->>'balanceDue')::bigint, 0) ELSE 0 END), 0)::bigint AS cash,
       COALESCE(SUM(CASE WHEN p.method = 'TRANSFER' THEN coalesce((iv.quote->>'balanceDue')::bigint, 0) ELSE 0 END), 0)::bigint AS transfer
FROM app.buildings b
LEFT JOIN app.units u ON u.tenant_id = b.tenant_id AND u.building_id = b.id
LEFT JOIN app.stays s ON s.tenant_id = u.tenant_id AND s.unit_id = u.id
LEFT JOIN app.invoices iv ON iv.tenant_id = s.tenant_id AND iv.stay_id = s.id AND iv.status = 'PAID'
                         AND iv.paid_at >= @from_at AND iv.paid_at < @to_at
LEFT JOIN app.payments p ON p.tenant_id = iv.tenant_id AND p.invoice_id = iv.id AND p.status = 'PAID'
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
