-- name: GetProperty :one
SELECT t.guesthouse_code, p.id, p.name, p.address, p.phone, p.qr_expiry_minutes, p.id_retention_days, p.front_desk_history_days
FROM app.properties p JOIN app.tenants t ON t.id = p.tenant_id
WHERE p.tenant_id = @tenant_id
ORDER BY p.created_at, p.id LIMIT 1;

-- name: UpdateProperty :exec
UPDATE app.properties
SET name = COALESCE(sqlc.narg(name), name), address = COALESCE(sqlc.narg(address), address), phone = COALESCE(sqlc.narg(phone), phone),
    qr_expiry_minutes = COALESCE(sqlc.narg(qr_expiry_minutes), qr_expiry_minutes),
    id_retention_days = COALESCE(sqlc.narg(id_retention_days), id_retention_days),
    front_desk_history_days = COALESCE(sqlc.narg(front_desk_history_days), front_desk_history_days)
WHERE tenant_id = @tenant_id AND id = @id;

-- name: ListBankAccounts :many
SELECT id, bank_bin, bank_name, account_no_masked, account_name, is_default, sepay_status, make_default_when_connected, last_webhook_at
FROM app.bank_accounts WHERE tenant_id = @tenant_id ORDER BY is_default DESC, created_at, id;

-- name: GetBankAccount :one
SELECT id, bank_bin, bank_name, account_no_masked, account_name, is_default, sepay_status, make_default_when_connected, last_webhook_at
FROM app.bank_accounts WHERE tenant_id = @tenant_id AND id = @id FOR UPDATE;

-- name: InsertBankAccount :exec
INSERT INTO app.bank_accounts (id, tenant_id, bank_bin, bank_name, account_enc, account_no_masked, account_name, account_fp,
                               is_default, sepay_status, make_default_when_connected)
VALUES (@id, @tenant_id, @bank_bin, @bank_name, @account_enc, @account_no_masked, @account_name, @account_fp,
        @is_default, @sepay_status, @make_default_when_connected);

-- name: ClearDefaultBankAccount :exec
UPDATE app.bank_accounts SET is_default = false WHERE tenant_id = @tenant_id AND is_default;

-- name: SetDefaultBankAccount :execrows
UPDATE app.bank_accounts SET is_default = true, make_default_when_connected = false
WHERE tenant_id = @tenant_id AND id = @id AND sepay_status = 'CONNECTED';

-- name: ConnectBankAccount :exec
UPDATE app.bank_accounts SET sepay_status = 'CONNECTED' WHERE tenant_id = @tenant_id AND id = @id;

-- name: DeleteBankAccount :execrows
DELETE FROM app.bank_accounts WHERE tenant_id = @tenant_id AND id = @id AND NOT is_default;

-- name: GetSepayState :one
SELECT t.sepay_signature_ok, (t.sepay_secret_enc IS NOT NULL)::boolean AS has_secret, t.hook_id,
       EXISTS (SELECT 1 FROM app.bank_accounts b WHERE b.tenant_id = t.id AND b.is_default AND b.sepay_status = 'CONNECTED') AS default_connected,
       (SELECT max(b.last_webhook_at) FROM app.bank_accounts b WHERE b.tenant_id = t.id)::timestamptz AS last_webhook_at
FROM app.tenants t WHERE t.id = @tenant_id;

-- name: SetTenantHook :exec
UPDATE app.tenants SET hook_id = @hook_id WHERE id = @tenant_id AND hook_id IS NULL;

-- name: SetSepaySecret :exec
UPDATE app.tenants SET sepay_secret_enc = @sepay_secret_enc, sepay_signature_ok = NULL WHERE id = @tenant_id;

-- name: InsertTenant :exec
INSERT INTO app.tenants (id, name, guesthouse_code, hook_id, default_locale, time_zone)
VALUES (@id, @name, @guesthouse_code, @hook_id, @default_locale, @time_zone);

-- name: GetSepaySecretEnc :one
SELECT sepay_secret_enc FROM app.tenants WHERE id = @tenant_id;

-- name: ListBankAccountBlobs :many
SELECT id, account_enc FROM app.bank_accounts WHERE tenant_id = @tenant_id ORDER BY id;

-- name: TouchBankAccountWebhook :exec
UPDATE app.bank_accounts SET last_webhook_at = @at WHERE tenant_id = @tenant_id AND id = @id;

-- name: SetSepaySignatureOK :exec
UPDATE app.tenants SET sepay_signature_ok = @ok WHERE id = @tenant_id AND sepay_signature_ok IS DISTINCT FROM @ok;
