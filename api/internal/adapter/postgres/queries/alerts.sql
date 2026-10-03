-- name: InsertAlert :exec
INSERT INTO app.alerts (id, tenant_id, kind, room_code, shift_id, stay_id, actor_id, amount, details)
VALUES (@id, @tenant_id, @kind, sqlc.narg(room_code), sqlc.narg(shift_id), sqlc.narg(stay_id), sqlc.narg(actor_id),
        sqlc.narg(amount), @details);

-- name: ResolveAlerts :execrows
-- Closes the open alerts of the given kinds for a stay, and the alert of a bank event (by event id; an alert raised before event ids
-- were kept is found by its amount and transfer note). History stays: only resolved_at and resolution are set.
UPDATE app.alerts SET resolved_at = @resolved_at, resolution = @resolution
WHERE tenant_id = @tenant_id AND resolved_at IS NULL
  AND ((@stay_id::text <> '' AND stay_id = @stay_id::text AND kind = ANY(@kinds::text[]))
       OR (@event_id::text <> '' AND kind = 'UNMATCHED_TRANSFER' AND (details->>'eventId' = @event_id::text
            OR (details->>'eventId' IS NULL AND amount = @event_amount::bigint AND details->>'transferNote' = @event_note::text))));
