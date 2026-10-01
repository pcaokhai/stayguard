-- +goose Up
CREATE TABLE app.stays (
    id                 text PRIMARY KEY,
    tenant_id          text NOT NULL,
    unit_id            text NOT NULL,
    rental_type        text NOT NULL CHECK (rental_type IN ('HOURLY', 'OVERNIGHT', 'DAILY')),
    status             text NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'CHECKED_OUT')),
    deposit            bigint NOT NULL DEFAULT 0 CHECK (deposit >= 0),
    check_in_at        timestamptz NOT NULL DEFAULT now(),
    check_out_at       timestamptz,
    rate_plan_snapshot jsonb NOT NULL,
    id_number_enc      bytea,
    billing_mode       text NOT NULL DEFAULT 'SESSION' CHECK (billing_mode IN ('SESSION')),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, unit_id) REFERENCES app.units (tenant_id, id)
);
CREATE UNIQUE INDEX stays_one_active_per_unit ON app.stays (unit_id) WHERE status = 'ACTIVE';

CREATE TABLE app.services (
    id        text PRIMARY KEY,
    tenant_id text NOT NULL REFERENCES app.tenants (id),
    name      jsonb NOT NULL,
    price     bigint NOT NULL CHECK (price >= 0),
    stock     bigint NOT NULL DEFAULT 0 CHECK (stock >= 0),
    UNIQUE (tenant_id, id)
);

CREATE TABLE app.stay_extras (
    id          text PRIMARY KEY,
    tenant_id   text NOT NULL,
    stay_id     text NOT NULL,
    service_id  text NOT NULL,
    quantity    integer NOT NULL CHECK (quantity > 0),
    unit_amount bigint NOT NULL CHECK (unit_amount >= 0),
    amount      bigint NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    CHECK (amount = quantity * unit_amount),
    FOREIGN KEY (tenant_id, stay_id) REFERENCES app.stays (tenant_id, id),
    FOREIGN KEY (tenant_id, service_id) REFERENCES app.services (tenant_id, id)
);

CREATE TABLE app.invoices (
    id         text PRIMARY KEY,
    tenant_id  text NOT NULL,
    stay_id    text NOT NULL UNIQUE,
    bill_code  text NOT NULL,
    quote      jsonb NOT NULL,
    total      bigint NOT NULL CHECK (total >= 0),
    status     text NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'PAID')),
    created_at timestamptz NOT NULL DEFAULT now(),
    paid_at    timestamptz,
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, bill_code),
    CHECK ((status = 'PAID') = (paid_at IS NOT NULL)),
    FOREIGN KEY (tenant_id, stay_id) REFERENCES app.stays (tenant_id, id)
);
CREATE INDEX invoices_open_by_stay ON app.invoices (stay_id) WHERE status = 'OPEN';

CREATE TABLE app.payments (
    id             text PRIMARY KEY,
    tenant_id      text NOT NULL,
    invoice_id     text NOT NULL,
    method         text NOT NULL CHECK (method IN ('CASH', 'TRANSFER')),
    status         text NOT NULL CHECK (status IN ('PENDING', 'PAID', 'EXPIRED', 'MISMATCH')),
    amount         bigint NOT NULL CHECK (amount >= 0),
    reference_code text,
    created_at     timestamptz NOT NULL DEFAULT now(),
    paid_at        timestamptz,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, invoice_id) REFERENCES app.invoices (tenant_id, id)
);
CREATE UNIQUE INDEX payments_one_pending_per_invoice ON app.payments (invoice_id) WHERE status = 'PENDING';
CREATE INDEX payments_match ON app.payments (tenant_id, reference_code) WHERE status = 'PENDING';
CREATE INDEX payments_paid_at ON app.payments (tenant_id, paid_at);

-- tenant_id stays NULL until the event is matched (docs/05 §2); see the policies below.
CREATE TABLE app.payment_events (
    id             text PRIMARY KEY,
    tenant_id      text REFERENCES app.tenants (id),
    provider       text NOT NULL,
    external_id    text NOT NULL,
    amount         bigint NOT NULL CHECK (amount >= 0),
    reference_code text,
    result         text NOT NULL CHECK (result IN ('SETTLED', 'MISMATCH', 'UNMATCHED', 'DUPLICATE_IGNORED')),
    received_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, external_id)
);

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['stays', 'services', 'stay_extras', 'invoices', 'payments'] LOOP
        EXECUTE format('ALTER TABLE app.%I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE app.%I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('CREATE POLICY %I ON app.%I USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant())', t || '_tenant', t);
    END LOOP;
END
$$;
-- +goose StatementEnd

-- Ruling 7: NULL never equals the current tenant, so unmatched events are invisible and untouchable
-- to the app role; it may only insert them (NULL or own tenant) and never move a row to another tenant.
ALTER TABLE app.payment_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.payment_events FORCE ROW LEVEL SECURITY;
CREATE POLICY payment_events_select ON app.payment_events FOR SELECT
    USING (tenant_id = app.current_tenant());
CREATE POLICY payment_events_insert ON app.payment_events FOR INSERT
    WITH CHECK (tenant_id IS NULL OR tenant_id = app.current_tenant());
CREATE POLICY payment_events_update ON app.payment_events FOR UPDATE
    USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant());
CREATE POLICY payment_events_delete ON app.payment_events FOR DELETE
    USING (tenant_id = app.current_tenant());
