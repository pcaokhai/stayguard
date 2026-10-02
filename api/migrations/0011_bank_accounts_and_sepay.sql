-- +goose Up
-- L-A3: receiving bank accounts, the per-tenant SePay hook, and the property settings the owner edits.

CREATE TABLE app.bank_accounts (
    id                         text PRIMARY KEY,
    tenant_id                  text NOT NULL REFERENCES app.tenants (id),
    bank_bin                   text NOT NULL CHECK (bank_bin ~ '^[0-9]{6}$'),
    bank_name                  text NOT NULL,
    -- AES-GCM of the JSON {bankBin, accountNo, accountName}; the number is never stored in clear.
    account_enc                bytea NOT NULL,
    account_no_masked          text NOT NULL,
    account_name               text NOT NULL,
    -- Keyed fingerprint of the number, so the same account cannot be added twice without keeping the number in clear.
    account_fp                 text NOT NULL,
    is_default                 boolean NOT NULL DEFAULT false,
    sepay_status               text NOT NULL DEFAULT 'PENDING' CHECK (sepay_status IN ('CONNECTED', 'PENDING')),
    make_default_when_connected boolean NOT NULL DEFAULT false,
    last_webhook_at            timestamptz,
    created_at                 timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, account_fp),
    -- Only a connected account may be the QR default.
    CHECK (NOT is_default OR sepay_status = 'CONNECTED')
);
-- One default per tenant: the QR always pays one account.
CREATE UNIQUE INDEX bank_accounts_one_default ON app.bank_accounts (tenant_id) WHERE is_default;
ALTER TABLE app.bank_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.bank_accounts FORCE ROW LEVEL SECURITY;
CREATE POLICY bank_accounts_tenant ON app.bank_accounts
    USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant());
GRANT SELECT, INSERT, UPDATE, DELETE ON app.bank_accounts TO stayguard_app;
GRANT SELECT, DELETE ON app.bank_accounts TO stayguard_maint;

-- Replaced by bank_accounts (demo trials were the only rows with a value).
ALTER TABLE app.tenants DROP COLUMN bank_account_enc;

-- The webhook address and secret. The secret is written only by the installer CLI and never read back by an endpoint.
ALTER TABLE app.tenants
    ADD COLUMN hook_id text,
    ADD COLUMN sepay_secret_enc bytea,
    ADD COLUMN sepay_signature_ok boolean;
CREATE UNIQUE INDEX tenants_hook_id ON app.tenants (hook_id) WHERE hook_id IS NOT NULL;

-- A webhook knows only its hookId, so the tenant is unknown until it is read: SELECT only, the one tenant holding
-- that hook, and only while no tenant is set (same pattern as sessions_lookup and tenants_signin_lookup).
CREATE FUNCTION app.hook_lookup_id() RETURNS text
    LANGUAGE sql STABLE
    AS $$ SELECT nullif(current_setting('app.hook_id', true), '') $$;
CREATE POLICY tenants_hook_lookup ON app.tenants FOR SELECT
    USING (hook_id = app.hook_lookup_id() AND app.current_tenant() IS NULL);

ALTER TABLE app.properties
    ADD COLUMN address text,
    ADD COLUMN phone text,
    ADD COLUMN qr_expiry_minutes integer NOT NULL DEFAULT 30 CHECK (qr_expiry_minutes BETWEEN 5 AND 240),
    ADD COLUMN id_retention_days integer NOT NULL DEFAULT 30 CHECK (id_retention_days BETWEEN 1 AND 365),
    ADD COLUMN front_desk_history_days integer NOT NULL DEFAULT 7 CHECK (front_desk_history_days BETWEEN 1 AND 90);
