-- +goose Up
-- F-A4: expenses (manual, recurring and written by the system), payroll inputs and paid state, and the marker that keeps
-- the monthly copy of recurring expenses from running twice.
CREATE TABLE app.expenses (
    id                  text PRIMARY KEY,
    tenant_id           text NOT NULL REFERENCES app.tenants (id),
    month               text NOT NULL CHECK (month ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'),
    category            text NOT NULL CHECK (category IN ('STAFF_PAY', 'RENT', 'ELECTRICITY', 'WATER', 'LAUNDRY', 'MAINTENANCE',
        'SUPPLIES', 'COST_OF_GOODS', 'TAX_FEES', 'INTERNET_TV', 'PAYMENT_FEES', 'OTHER')),
    amount              bigint NOT NULL CHECK (amount >= 0),
    paid_on             date,
    note                text,
    recurring           boolean NOT NULL DEFAULT false,
    source              text NOT NULL CHECK (source IN ('MANUAL', 'RECURRING', 'PAYROLL', 'MAINTENANCE', 'STOCK')),
    -- What an automatic or copied line came from (a payroll line, a ticket, a stock movement, a recurring chain and month):
    -- one line per source and reference, so a retry or a second run cannot post it twice.
    ref_id              text,
    -- The first line of a recurring chain; copies carry it forward.
    root_id             text,
    attachment_asset_id text,
    created_by          text,
    created_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id)
);
CREATE UNIQUE INDEX expenses_one_per_ref ON app.expenses (tenant_id, source, ref_id) WHERE ref_id IS NOT NULL;
CREATE INDEX expenses_by_month ON app.expenses (tenant_id, month);

CREATE TABLE app.payroll_lines (
    tenant_id text NOT NULL,
    user_id   text NOT NULL,
    month     text NOT NULL CHECK (month ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'),
    bonus     bigint NOT NULL DEFAULT 0 CHECK (bonus >= 0),
    deduction bigint NOT NULL DEFAULT 0 CHECK (deduction >= 0),
    note      text,
    status    text NOT NULL DEFAULT 'UNPAID' CHECK (status IN ('UNPAID', 'PAID')),
    -- The line as it was when paid, so a later change of contract or roster does not rewrite what was paid.
    frozen    jsonb,
    paid_at   timestamptz,
    paid_by   text,
    PRIMARY KEY (tenant_id, user_id, month),
    CHECK ((status = 'PAID') = (frozen IS NOT NULL)),
    FOREIGN KEY (tenant_id, user_id) REFERENCES app.users (tenant_id, id)
);

CREATE TABLE app.recurring_runs (
    tenant_id text NOT NULL REFERENCES app.tenants (id),
    month     text NOT NULL CHECK (month ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'),
    ran_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, month)
);

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['expenses', 'payroll_lines', 'recurring_runs'] LOOP
        EXECUTE format('ALTER TABLE app.%I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE app.%I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('CREATE POLICY %I ON app.%I USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant())', t || '_tenant', t);
        EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON app.%I TO stayguard_app', t);
        EXECUTE format('GRANT SELECT, DELETE ON app.%I TO stayguard_maint', t);
    END LOOP;
END
$$;
-- +goose StatementEnd
