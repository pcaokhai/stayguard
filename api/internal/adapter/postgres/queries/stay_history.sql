-- name: GetStayBuilding :one
SELECT u.building_id
FROM app.stays s JOIN app.units u ON u.tenant_id = s.tenant_id AND u.id = s.unit_id
WHERE s.tenant_id = @tenant_id AND s.id = @stay_id;

-- name: ListStayHistory :many
-- state: MISMATCH and UNPAID (money problems) win over TIME_EDITED, which wins over IN_STAY and PAID.
-- The search pattern is escaped by the caller; check_in_at DESC, id DESC is the cursor order.
WITH base AS (
    SELECT s.id, u.code AS room_code, s.guest_name, s.rental_type, s.check_in_at, s.check_out_at, s.status,
           u.building_id, coalesce(fu.name, '') AS front_desk_name, s.guest_phone,
           iv.total AS total, coalesce(pm.method, '')::text AS payment_method,
           CASE
               WHEN iv.status = 'OPEN' AND EXISTS (SELECT 1 FROM app.payments p
                    WHERE p.tenant_id = s.tenant_id AND p.invoice_id = iv.id AND p.status = 'MISMATCH') THEN 'MISMATCH'
               WHEN iv.status = 'OPEN' THEN 'UNPAID'
               WHEN EXISTS (SELECT 1 FROM app.stay_edits e
                    WHERE e.tenant_id = s.tenant_id AND e.stay_id = s.id AND e.kind = 'CHECK_IN') THEN 'TIME_EDITED'
               WHEN s.status = 'ACTIVE' THEN 'IN_STAY'
               ELSE 'PAID'
           END AS state
    FROM app.stays s
    JOIN app.units u ON u.tenant_id = s.tenant_id AND u.id = s.unit_id
    LEFT JOIN app.users fu ON fu.tenant_id = s.tenant_id AND fu.id = s.created_by
    LEFT JOIN app.invoices iv ON iv.tenant_id = s.tenant_id AND iv.stay_id = s.id
    LEFT JOIN LATERAL (
        SELECT p.method FROM app.payments p
        WHERE p.tenant_id = s.tenant_id AND p.invoice_id = iv.id AND p.status = 'PAID'
        ORDER BY p.paid_at DESC LIMIT 1) pm ON true
    WHERE s.tenant_id = @tenant_id
      AND s.check_in_at >= @from_at AND s.check_in_at < @to_at
      AND u.building_id = ANY(@building_ids::text[])
)
SELECT id, room_code, guest_name, rental_type, check_in_at, check_out_at, status, front_desk_name, total, payment_method, state
FROM base
WHERE (sqlc.narg(building_id)::text IS NULL OR building_id = sqlc.narg(building_id)::text)
  AND (sqlc.narg(state)::text IS NULL OR state = sqlc.narg(state)::text)
  AND (sqlc.narg(pattern)::text IS NULL OR room_code ILIKE sqlc.narg(pattern)::text ESCAPE '\'
       OR guest_name ILIKE sqlc.narg(pattern)::text ESCAPE '\' OR guest_phone ILIKE sqlc.narg(pattern)::text ESCAPE '\')
  AND (sqlc.narg(cursor_at)::timestamptz IS NULL OR (check_in_at, id) < (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::text))
ORDER BY check_in_at DESC, id DESC
LIMIT @row_limit;

-- name: ListStayTimeline :many
-- One row per event; actors are resolved from the recorded user id (empty for the bank and the system).
SELECT at::timestamptz AS at, kind::text AS kind, actor_name::text AS actor_name, details::jsonb AS details FROM (
    SELECT coalesce((SELECT e.old_check_in_at FROM app.stay_edits e
                     WHERE e.tenant_id = s.tenant_id AND e.stay_id = s.id AND e.kind = 'CHECK_IN'
                     ORDER BY e.created_at, e.id LIMIT 1), s.check_in_at) AS at,
           'CHECKED_IN' AS kind, coalesce(u.name, '') AS actor_name,
           jsonb_build_object('room', un.code, 'rentalType', s.rental_type) AS details
    FROM app.stays s
    JOIN app.units un ON un.tenant_id = s.tenant_id AND un.id = s.unit_id
    LEFT JOIN app.users u ON u.tenant_id = s.tenant_id AND u.id = s.created_by
    WHERE s.tenant_id = @tenant_id AND s.id = @stay_id
  UNION ALL
    SELECT e.created_at, 'CHECK_IN_EDITED', coalesce(u.name, ''),
           jsonb_build_object('oldTime', to_char(e.old_check_in_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
                              'newTime', to_char(e.new_check_in_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
                              'reasonCode', e.reason_code, 'note', coalesce(e.note, ''))
    FROM app.stay_edits e LEFT JOIN app.users u ON u.tenant_id = e.tenant_id AND u.id = e.actor_id
    WHERE e.tenant_id = @tenant_id AND e.stay_id = @stay_id AND e.kind = 'CHECK_IN'
  UNION ALL
    SELECT e.created_at, 'MOVED', coalesce(u.name, ''),
           jsonb_build_object('from', coalesce(e.from_room_code, ''), 'to', coalesce(e.to_room_code, ''))
    FROM app.stay_edits e LEFT JOIN app.users u ON u.tenant_id = e.tenant_id AND u.id = e.actor_id
    WHERE e.tenant_id = @tenant_id AND e.stay_id = @stay_id AND e.kind = 'MOVE'
  UNION ALL
    SELECT x.created_at, 'EXTRAS_ADDED', coalesce(u.name, ''),
           jsonb_build_object('service', sv.code, 'quantity', x.quantity::text)
    FROM app.stay_extras x
    JOIN app.services sv ON sv.tenant_id = x.tenant_id AND sv.id = x.service_id
    LEFT JOIN app.users u ON u.tenant_id = x.tenant_id AND u.id = x.created_by
    WHERE x.tenant_id = @tenant_id AND x.stay_id = @stay_id
  UNION ALL
    SELECT iv.created_at, 'CHECKED_OUT', coalesce(u.name, ''), jsonb_build_object('billCode', iv.bill_code)
    FROM app.invoices iv
    LEFT JOIN app.audit_logs a ON a.tenant_id = iv.tenant_id AND a.action = 'stay.checked_out' AND a.entity_id = iv.stay_id
    LEFT JOIN app.users u ON u.tenant_id = a.tenant_id AND u.id = a.actor_id
    WHERE iv.tenant_id = @tenant_id AND iv.stay_id = @stay_id
  UNION ALL
    SELECT p.paid_at, 'PAYMENT_RECEIVED', coalesce(u.name, ''),
           jsonb_build_object('method', p.method, 'amount', p.amount::text)
    FROM app.payments p
    JOIN app.invoices iv ON iv.tenant_id = p.tenant_id AND iv.id = p.invoice_id
    LEFT JOIN app.audit_logs a ON a.tenant_id = p.tenant_id AND a.action = 'payment.settled' AND a.entity_id = p.id
    LEFT JOIN app.users u ON u.tenant_id = a.tenant_id AND u.id = a.actor_id
    WHERE p.tenant_id = @tenant_id AND iv.stay_id = @stay_id AND p.status = 'PAID'
  UNION ALL
    SELECT pe.received_at, 'PAYMENT_MISMATCH', '',
           jsonb_build_object('expected', p.amount::text, 'received', coalesce(p.received_amount, 0)::text)
    FROM app.payments p
    JOIN app.invoices iv ON iv.tenant_id = p.tenant_id AND iv.id = p.invoice_id
    JOIN app.payment_events pe ON pe.tenant_id = p.tenant_id AND pe.external_id = p.transaction_id AND pe.result = 'MISMATCH'
    WHERE p.tenant_id = @tenant_id AND iv.stay_id = @stay_id AND p.status = 'MISMATCH'
  UNION ALL
    SELECT a.created_at, 'LINKED_BY_OWNER', coalesce(u.name, ''), '{}'::jsonb
    FROM app.invoices iv
    JOIN app.audit_logs a ON a.tenant_id = iv.tenant_id AND a.action = 'payment.linked' AND a.entity_id = iv.id
    LEFT JOIN app.users u ON u.tenant_id = a.tenant_id AND u.id = a.actor_id
    WHERE iv.tenant_id = @tenant_id AND iv.stay_id = @stay_id
  UNION ALL
    SELECT c.created_at, 'CLEANED', coalesce(u.name, ''), '{}'::jsonb
    FROM app.stays s
    JOIN app.invoices iv ON iv.tenant_id = s.tenant_id AND iv.stay_id = s.id AND iv.paid_at IS NOT NULL
    JOIN LATERAL (SELECT a.created_at, a.actor_id, a.tenant_id FROM app.audit_logs a
                  WHERE a.tenant_id = s.tenant_id AND a.action = 'room.cleaned' AND a.entity_id = s.unit_id
                    AND a.created_at >= iv.paid_at
                  ORDER BY a.created_at LIMIT 1) c ON true
    LEFT JOIN app.users u ON u.tenant_id = c.tenant_id AND u.id = c.actor_id
    WHERE s.tenant_id = @tenant_id AND s.id = @stay_id
) ev ORDER BY at, kind;

-- name: GetReceiptInvoice :one
SELECT iv.id, iv.stay_id, iv.bill_code, iv.quote, u.building_id, u.code AS room_code, s.check_in_at, s.check_out_at,
       coalesce((SELECT p.name FROM app.properties p WHERE p.tenant_id = iv.tenant_id ORDER BY p.created_at, p.id LIMIT 1), '')::text AS property_name
FROM app.invoices iv
JOIN app.stays s ON s.tenant_id = iv.tenant_id AND s.id = iv.stay_id
JOIN app.units u ON u.tenant_id = s.tenant_id AND u.id = s.unit_id
WHERE iv.tenant_id = @tenant_id AND iv.id = @invoice_id AND s.check_out_at IS NOT NULL;

-- name: ListReceiptPayments :many
SELECT p.id, p.method, coalesce(p.received_amount, p.amount) AS amount, p.paid_at
FROM app.payments p
WHERE p.tenant_id = @tenant_id AND p.invoice_id = @invoice_id AND p.status = 'PAID' AND p.paid_at IS NOT NULL
ORDER BY p.paid_at, p.id;

-- name: GetFrontDeskHistoryDays :one
SELECT coalesce(min(front_desk_history_days), 7)::int AS days FROM app.properties WHERE tenant_id = @tenant_id;
