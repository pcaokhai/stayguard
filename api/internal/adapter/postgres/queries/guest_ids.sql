-- Guest ID data (F-A2). Only the guest ID repository runs these; stay queries see yes/no indicators only.

-- name: GuestIDStay :one
SELECT s.id, s.status, s.check_out_at, u.building_id, u.code AS room_code
FROM app.stays s JOIN app.units u ON u.tenant_id = s.tenant_id AND u.id = s.unit_id
WHERE s.tenant_id = @tenant_id AND s.id = @stay_id
FOR UPDATE OF s;

-- name: UpsertGuestNumber :exec
INSERT INTO app.guest_ids (tenant_id, stay_id, number_enc, collected_at, collected_by)
VALUES (@tenant_id, @stay_id, @number_enc, @at, sqlc.narg(by))
ON CONFLICT (tenant_id, stay_id) DO UPDATE SET number_enc = EXCLUDED.number_enc;

-- name: EnsureGuestRecord :exec
INSERT INTO app.guest_ids (tenant_id, stay_id, collected_at, collected_by)
VALUES (@tenant_id, @stay_id, @at, sqlc.narg(by))
ON CONFLICT (tenant_id, stay_id) DO NOTHING;

-- name: GetGuestIDRow :one
SELECT number_enc, collected_at FROM app.guest_ids WHERE tenant_id = @tenant_id AND stay_id = @stay_id;

-- name: ClearGuestNumber :execrows
UPDATE app.guest_ids SET number_enc = NULL WHERE tenant_id = @tenant_id AND stay_id = @stay_id AND number_enc IS NOT NULL;

-- name: UpsertGuestPhoto :exec
INSERT INTO app.guest_id_photos (tenant_id, stay_id, side, image_enc, bytes, uploaded_at, uploaded_by)
VALUES (@tenant_id, @stay_id, @side, @image_enc, @bytes, @uploaded_at, sqlc.narg(uploaded_by))
ON CONFLICT (tenant_id, stay_id, side) DO UPDATE
SET image_enc = EXCLUDED.image_enc, bytes = EXCLUDED.bytes, uploaded_at = EXCLUDED.uploaded_at, uploaded_by = EXCLUDED.uploaded_by;

-- name: ListGuestPhotoMeta :many
SELECT p.side, p.bytes, p.uploaded_at, COALESCE(u.name, '') AS uploaded_by
FROM app.guest_id_photos p LEFT JOIN app.users u ON u.tenant_id = p.tenant_id AND u.id = p.uploaded_by
WHERE p.tenant_id = @tenant_id AND p.stay_id = @stay_id ORDER BY p.side;

-- name: GetGuestPhotoEnc :one
SELECT image_enc FROM app.guest_id_photos WHERE tenant_id = @tenant_id AND stay_id = @stay_id AND side = @side;

-- name: DeleteGuestPhoto :execrows
DELETE FROM app.guest_id_photos WHERE tenant_id = @tenant_id AND stay_id = @stay_id AND side = @side;

-- name: IdRetentionDays :one
SELECT id_retention_days FROM app.properties WHERE tenant_id = @tenant_id ORDER BY created_at, id LIMIT 1;

-- name: ExpiredGuestIDStays :many
-- Stays whose guest ID data is past retention: checked out more than N days ago.
SELECT g.stay_id FROM app.guest_ids g
JOIN app.stays s ON s.tenant_id = g.tenant_id AND s.id = g.stay_id
WHERE g.tenant_id = @tenant_id AND s.check_out_at IS NOT NULL AND s.check_out_at < @cutoff
ORDER BY g.stay_id;

-- name: DeleteGuestPhotosOfStay :exec
DELETE FROM app.guest_id_photos WHERE tenant_id = @tenant_id AND stay_id = @stay_id;

-- name: DeleteGuestIDRow :exec
DELETE FROM app.guest_ids WHERE tenant_id = @tenant_id AND stay_id = @stay_id;

-- name: GuestIDIndicatorsFor :many
-- Yes/no flags for a page of stays in one read; stays with no guest ID row are simply absent.
SELECT g.stay_id,
       (g.number_enc IS NOT NULL)::boolean AS has_number,
       EXISTS (SELECT 1 FROM app.guest_id_photos p WHERE p.tenant_id = g.tenant_id AND p.stay_id = g.stay_id AND p.side = 'FRONT') AS has_front,
       EXISTS (SELECT 1 FROM app.guest_id_photos p WHERE p.tenant_id = g.tenant_id AND p.stay_id = g.stay_id AND p.side = 'BACK') AS has_back
FROM app.guest_ids g
WHERE g.tenant_id = @tenant_id AND g.stay_id = ANY(@stay_ids::text[]);
