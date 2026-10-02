-- name: InsertExpense :exec
INSERT INTO app.expenses (id, tenant_id, month, category, amount, paid_on, note, recurring, source, ref_id, root_id, attachment_asset_id, created_by)
VALUES (@id, @tenant_id, @month, @category, @amount, sqlc.narg(paid_on), sqlc.narg(note), @recurring, @source, sqlc.narg(ref_id),
        sqlc.narg(root_id), sqlc.narg(attachment_asset_id), sqlc.narg(created_by));

-- name: InsertAutoExpense :execrows
-- One line per source and reference: posting the same payroll line, ticket or stock movement twice changes nothing.
INSERT INTO app.expenses (id, tenant_id, month, category, amount, paid_on, note, recurring, source, ref_id, created_by)
VALUES (@id, @tenant_id, @month, @category, @amount, sqlc.narg(paid_on), sqlc.narg(note), false, @source, @ref_id, sqlc.narg(created_by))
ON CONFLICT (tenant_id, source, ref_id) WHERE ref_id IS NOT NULL DO NOTHING;

-- name: GetExpense :one
SELECT id, month, category, amount, paid_on, note, recurring, source, attachment_asset_id
FROM app.expenses WHERE tenant_id = @tenant_id AND id = @expense_id;

-- name: ListExpensesOfMonth :many
SELECT id, month, category, amount, paid_on, note, recurring, source, attachment_asset_id
FROM app.expenses WHERE tenant_id = @tenant_id AND month = @month
ORDER BY category, source, created_at, id;

-- name: UpdateExpense :execrows
-- Only lines a person wrote can be edited; the system's lines (PAYROLL, MAINTENANCE, STOCK) are not touched.
UPDATE app.expenses SET month = @month, category = @category, amount = @amount, paid_on = sqlc.narg(paid_on), note = sqlc.narg(note),
       recurring = @recurring, attachment_asset_id = sqlc.narg(attachment_asset_id)
WHERE tenant_id = @tenant_id AND id = @expense_id AND source IN ('MANUAL', 'RECURRING');

-- name: DeleteExpense :execrows
DELETE FROM app.expenses WHERE tenant_id = @tenant_id AND id = @expense_id AND source IN ('MANUAL', 'RECURRING');

-- name: ExpenseTotalsByMonth :many
SELECT month, sum(amount)::bigint AS total FROM app.expenses
WHERE tenant_id = @tenant_id AND month >= @from_month AND month <= @to_month GROUP BY month ORDER BY month;

-- name: ExpenseTotalsByCategory :many
SELECT category, sum(amount)::bigint AS total FROM app.expenses
WHERE tenant_id = @tenant_id AND month >= @from_month AND month <= @to_month GROUP BY category ORDER BY category;

-- name: ExpenseSummaryOfMonth :many
SELECT category, source, sum(amount)::bigint AS total FROM app.expenses
WHERE tenant_id = @tenant_id AND month = @month GROUP BY category, source ORDER BY category, source;

-- name: EarliestRecurringMonth :one
SELECT coalesce(min(month), '')::text AS month FROM app.expenses WHERE tenant_id = @tenant_id AND recurring;

-- name: RecurringRan :one
SELECT count(*) FROM app.recurring_runs WHERE tenant_id = @tenant_id AND month = @month;

-- name: MarkRecurringRan :exec
INSERT INTO app.recurring_runs (tenant_id, month) VALUES (@tenant_id, @month) ON CONFLICT DO NOTHING;

-- name: CopyRecurringExpenses :execrows
-- The recurring lines of the month before, copied into @to_month; a chain keeps its first line as root.
INSERT INTO app.expenses (id, tenant_id, month, category, amount, note, recurring, source, ref_id, root_id, attachment_asset_id, created_by)
SELECT 'ex_' || replace(gen_random_uuid()::text, '-', ''), e.tenant_id, @to_month::text, e.category, e.amount, e.note, true, 'RECURRING',
       coalesce(e.root_id, e.id) || '@' || @to_month::text, coalesce(e.root_id, e.id), e.attachment_asset_id, e.created_by
FROM app.expenses e
WHERE e.tenant_id = @tenant_id AND e.month = @from_month AND e.recurring AND e.source IN ('MANUAL', 'RECURRING')
ON CONFLICT (tenant_id, source, ref_id) WHERE ref_id IS NOT NULL DO NOTHING;

-- name: PaidInvoiceTotals :one
-- Revenue is what paid invoices were for (the frozen quote total), by the day they were paid.
SELECT coalesce(sum(total), 0)::bigint AS revenue FROM app.invoices
WHERE tenant_id = @tenant_id AND status = 'PAID' AND paid_at >= @from_at AND paid_at < @to_at;

-- name: RevenueByMonth :many
SELECT to_char(paid_at AT TIME ZONE sqlc.arg(zone)::text, 'YYYY-MM') AS month, sum(total)::bigint AS revenue FROM app.invoices
WHERE tenant_id = @tenant_id AND status = 'PAID' AND paid_at >= @from_at AND paid_at < @to_at GROUP BY 1 ORDER BY 1;

-- name: RevenueByRentalType :many
SELECT s.rental_type AS key, sum(iv.total)::bigint AS revenue
FROM app.invoices iv JOIN app.stays s ON s.tenant_id = iv.tenant_id AND s.id = iv.stay_id
WHERE iv.tenant_id = @tenant_id AND iv.status = 'PAID' AND iv.paid_at >= @from_at AND iv.paid_at < @to_at GROUP BY 1 ORDER BY 1;

-- name: RevenueByBuilding :many
SELECT u.building_id AS key, sum(iv.total)::bigint AS revenue
FROM app.invoices iv JOIN app.stays s ON s.tenant_id = iv.tenant_id AND s.id = iv.stay_id
JOIN app.units u ON u.tenant_id = s.tenant_id AND u.id = s.unit_id
WHERE iv.tenant_id = @tenant_id AND iv.status = 'PAID' AND iv.paid_at >= @from_at AND iv.paid_at < @to_at GROUP BY 1 ORDER BY 1;

-- name: RevenueDepositPart :one
-- The part of paid invoices that the deposit covered (taken in cash at check-in).
SELECT coalesce(sum(least(coalesce((quote->>'depositPaid')::bigint, 0), total)), 0)::bigint AS cash
FROM app.invoices WHERE tenant_id = @tenant_id AND status = 'PAID' AND paid_at >= @from_at AND paid_at < @to_at;

-- name: RevenueBalanceByMethod :many
-- The part paid after the deposit, by the method of the payment that settled the invoice.
SELECT p.method AS key, sum(coalesce((iv.quote->>'balanceDue')::bigint, 0))::bigint AS revenue
FROM app.invoices iv JOIN app.payments p ON p.tenant_id = iv.tenant_id AND p.invoice_id = iv.id AND p.status = 'PAID'
WHERE iv.tenant_id = @tenant_id AND iv.status = 'PAID' AND iv.paid_at >= @from_at AND iv.paid_at < @to_at GROUP BY 1 ORDER BY 1;

-- name: OccupiedRoomDays :one
-- Distinct (room, local day) pairs with a stay in them between two local days, both included.
SELECT count(*)::bigint AS days FROM (
    SELECT DISTINCT s.unit_id, d::date AS day
    FROM app.stays s
    CROSS JOIN LATERAL generate_series(
        greatest((s.check_in_at AT TIME ZONE sqlc.arg(zone)::text)::date, sqlc.arg(from_day)::date),
        least((coalesce(s.check_out_at, now()) AT TIME ZONE sqlc.arg(zone)::text)::date, sqlc.arg(to_day)::date),
        interval '1 day') d
    WHERE s.tenant_id = @tenant_id
) x;

-- name: CountRooms :one
SELECT count(*)::bigint FROM app.units WHERE tenant_id = @tenant_id;

-- name: ListPayrollStaff :many
SELECT u.id, u.name, u.status, p.position, p.pay_type, p.rate, p.fixed_allowance, p.standard_shifts
FROM app.staff_profiles p JOIN app.users u ON u.tenant_id = p.tenant_id AND u.id = p.user_id
WHERE p.tenant_id = @tenant_id AND p.start_date <= sqlc.arg(month_last)::date
  AND (u.status <> 'REMOVED'
       OR EXISTS (SELECT 1 FROM app.roster_assignments r WHERE r.tenant_id = u.tenant_id AND r.user_id = u.id
                  AND r.work_date >= sqlc.arg(month_first)::date AND r.work_date <= sqlc.arg(month_last)::date)
       OR EXISTS (SELECT 1 FROM app.payroll_lines l WHERE l.tenant_id = u.tenant_id AND l.user_id = u.id AND l.month = @month))
ORDER BY u.name, u.id;

-- name: ListPayrollLines :many
SELECT user_id, bonus, deduction, coalesce(note, '') AS note, status, frozen, paid_at FROM app.payroll_lines
WHERE tenant_id = @tenant_id AND month = @month;

-- name: UpsertPayrollLine :exec
INSERT INTO app.payroll_lines (tenant_id, user_id, month, bonus, deduction, note)
VALUES (@tenant_id, @user_id, @month, @bonus, @deduction, sqlc.narg(note))
ON CONFLICT (tenant_id, user_id, month) DO UPDATE SET bonus = excluded.bonus, deduction = excluded.deduction, note = excluded.note
WHERE app.payroll_lines.status = 'UNPAID';

-- name: LockPayrollLine :one
SELECT status FROM app.payroll_lines WHERE tenant_id = @tenant_id AND user_id = @user_id AND month = @month FOR UPDATE;

-- name: FreezePayrollLine :execrows
INSERT INTO app.payroll_lines (tenant_id, user_id, month, status, frozen, paid_at, paid_by)
VALUES (@tenant_id, @user_id, @month, 'PAID', @frozen, @paid_at, sqlc.narg(paid_by))
ON CONFLICT (tenant_id, user_id, month) DO UPDATE SET status = 'PAID', frozen = excluded.frozen, paid_at = excluded.paid_at, paid_by = excluded.paid_by
WHERE app.payroll_lines.status = 'UNPAID';
