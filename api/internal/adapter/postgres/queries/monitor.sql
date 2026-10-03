-- name: ListAlerts :many
SELECT a.id, a.kind, coalesce(a.room_code, '') AS room_code, coalesce(a.shift_id, '') AS shift_id, coalesce(a.stay_id, '') AS stay_id,
       coalesce(u.name, '') AS actor_name, a.amount, a.details, a.created_at, a.resolved_at, a.resolution,
       a.read_at, coalesce(ru.name, '') AS read_by_name
FROM app.alerts a
LEFT JOIN app.users u ON u.tenant_id = a.tenant_id AND u.id = a.actor_id
LEFT JOIN app.users ru ON ru.tenant_id = a.tenant_id AND ru.id = a.read_by
WHERE a.tenant_id = @tenant_id
  AND (NOT @unread_only::boolean OR a.read_at IS NULL)
  AND (NOT @unresolved_only::boolean OR a.resolved_at IS NULL)
  AND (sqlc.narg(kind)::text IS NULL OR a.kind = sqlc.narg(kind)::text)
  AND (sqlc.narg(cursor_at)::timestamptz IS NULL OR (a.created_at, a.id) < (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::text))
ORDER BY a.created_at DESC, a.id DESC
LIMIT @row_limit;

-- name: MarkAlertRead :execrows
-- An alert already read stays as it was: the first reader and time are kept, and the call still succeeds.
UPDATE app.alerts SET read_at = coalesce(read_at, @read_at), read_by = coalesce(read_by, @read_by)
WHERE tenant_id = @tenant_id AND id = @alert_id;

-- name: ListTransactions :many
-- Reads the app.transactions view (migration 0015): paid cash and transfers, transfers with a wrong amount (MISMATCH)
-- and bank events nobody matched (UNMATCHED). The view runs with the caller's rights, so row-level security applies.
SELECT t.id, t.happened_at, t.amount, t.method, t.room_code, t.bill_code, t.reconciliation, t.transfer_note,
       t.payment_event_id, t.shift_id, t.kind, t.received_at, t.settled_at
FROM app.transactions t
WHERE t.tenant_id = @tenant_id AND t.happened_at >= @from_at AND t.happened_at < @to_at
  AND (@kind::text = 'ALL' OR (@kind::text = 'NEEDS_ACTION' AND t.reconciliation IN ('MISMATCH', 'UNMATCHED')) OR t.method = @kind::text)
  AND (sqlc.narg(pattern)::text IS NULL OR t.room_code ILIKE sqlc.narg(pattern)::text ESCAPE '\'
       OR t.bill_code ILIKE sqlc.narg(pattern)::text ESCAPE '\' OR t.transfer_note ILIKE sqlc.narg(pattern)::text ESCAPE '\')
  AND (sqlc.narg(cursor_at)::timestamptz IS NULL OR (t.happened_at, t.id) < (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::text))
ORDER BY t.happened_at DESC, t.id DESC
LIMIT @row_limit;

-- name: LockPaymentEvent :one
SELECT id, provider, external_id, amount, coalesce(reference_code, '') AS content, result, received_at
FROM app.payment_events
WHERE tenant_id = @tenant_id AND id = @event_id
FOR UPDATE;

-- name: ListAuditLogs :many
-- room_code and bill_code are resolved here from entity_type and entity_id (stay, invoice, payment, room), so rows
-- written before the documents carried them are filled too.
SELECT a.id, a.created_at, coalesce(u.name, '') AS actor_name, coalesce(u.role, '') AS actor_role, a.action, a.after,
       coalesce(ru.code, '')::text AS room_code, coalesce(inv.bill_code, '')::text AS bill_code
FROM app.audit_logs a
LEFT JOIN app.users u ON u.tenant_id = a.tenant_id AND u.id = a.actor_id
LEFT JOIN app.payments pm ON pm.tenant_id = a.tenant_id AND a.entity_type = 'payment' AND pm.id = a.entity_id
LEFT JOIN app.invoices inv ON inv.tenant_id = a.tenant_id
     AND ((a.entity_type = 'invoice' AND inv.id = a.entity_id) OR (a.entity_type = 'payment' AND inv.id = pm.invoice_id)
          OR (a.entity_type = 'stay' AND inv.stay_id = a.entity_id))
LEFT JOIN app.stays st ON st.tenant_id = a.tenant_id AND st.id = CASE WHEN a.entity_type = 'stay' THEN a.entity_id ELSE inv.stay_id END
LEFT JOIN app.units ru ON ru.tenant_id = a.tenant_id AND ru.id = CASE WHEN a.entity_type = 'room' THEN a.entity_id ELSE st.unit_id END
WHERE a.tenant_id = @tenant_id AND a.created_at >= @from_at AND a.created_at < @to_at
  AND (sqlc.narg(actor_id)::text IS NULL OR a.actor_id = sqlc.narg(actor_id)::text)
  AND (cardinality(@prefixes::text[]) = 0 OR a.action LIKE ANY(@prefixes::text[]))
  AND (sqlc.narg(pattern)::text IS NULL OR a.action ILIKE sqlc.narg(pattern)::text ESCAPE '\'
       OR a.entity_id ILIKE sqlc.narg(pattern)::text ESCAPE '\' OR u.name ILIKE sqlc.narg(pattern)::text ESCAPE '\')
  AND (sqlc.narg(cursor_at)::timestamptz IS NULL OR (a.created_at, a.id) < (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::text))
ORDER BY a.created_at DESC, a.id DESC
LIMIT @row_limit;

-- name: ListLongToClean :many
-- Rooms waiting to be cleaned since before the cutoff (last check-out time).
SELECT u.code AS room_code, max(s.check_out_at)::timestamptz AS since
FROM app.units u
JOIN app.stays s ON s.tenant_id = u.tenant_id AND s.unit_id = u.id AND s.check_out_at IS NOT NULL
WHERE u.tenant_id = @tenant_id AND u.status = 'TO_CLEAN' AND NOT u.retired
GROUP BY u.code
HAVING max(s.check_out_at) < @before
ORDER BY since, u.code;

-- name: ListUnpaidInvoices :many
-- Checked-out invoices still open with something to pay. paid is the deposit plus money the bank reported that did not
-- settle the invoice (a transfer of the wrong amount); the caller works out the balance and the order.
SELECT iv.id, iv.bill_code, un.code AS room_code, s.guest_name, s.check_out_at, iv.total,
       least(coalesce((iv.quote->>'depositPaid')::bigint, 0), iv.total)::bigint AS deposit,
       (coalesce((SELECT sum(coalesce(p.received_amount, 0)) FROM app.payments p
                  WHERE p.tenant_id = iv.tenant_id AND p.invoice_id = iv.id AND p.status = 'MISMATCH'), 0)
        + coalesce((SELECT sum(pe.amount) FROM app.payment_events pe
                    WHERE pe.tenant_id = iv.tenant_id AND pe.invoice_id = iv.id AND pe.result = 'PARTIAL'), 0))::bigint AS reported
FROM app.invoices iv
JOIN app.stays s ON s.tenant_id = iv.tenant_id AND s.id = iv.stay_id
JOIN app.units un ON un.tenant_id = s.tenant_id AND un.id = s.unit_id
WHERE iv.tenant_id = @tenant_id AND iv.status = 'OPEN' AND s.check_out_at IS NOT NULL
  AND coalesce((iv.quote->>'balanceDue')::bigint, 0) > 0
ORDER BY s.check_out_at DESC, iv.id DESC
LIMIT 500;

-- name: CountUnreadAlerts :one
-- Alerts nobody read and that still need the owner (not resolved): the overview's badge. Not limited like the overview list.
SELECT count(*)::bigint FROM app.alerts WHERE tenant_id = @tenant_id AND read_at IS NULL AND resolved_at IS NULL;
