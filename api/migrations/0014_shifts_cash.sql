-- +goose Up
-- L-B2: front-desk shifts and the cash ledger that expected cash is computed from.
CREATE TABLE app.shifts (
    id                  text PRIMARY KEY,
    tenant_id           text NOT NULL,
    user_id             text NOT NULL,
    status              text NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'CLOSED')),
    shift_code          text NOT NULL CHECK (shift_code IN ('MORNING', 'AFTERNOON', 'NIGHT')),
    opened_at           timestamptz NOT NULL,
    closed_at           timestamptz,
    opening_float       bigint NOT NULL DEFAULT 0 CHECK (opening_float >= 0),
    -- Figures below are written once, at close, and then locked by the trigger.
    expected_cash       bigint,
    counted_cash        bigint CHECK (counted_cash >= 0),
    difference          bigint,
    reason              text,
    reason_recorded_at  timestamptz,
    float_left          bigint CHECK (float_left >= 0),
    handover_to_user_id text,
    counts              jsonb,
    UNIQUE (tenant_id, id),
    CHECK ((status = 'CLOSED') = (closed_at IS NOT NULL)),
    CHECK (status = 'OPEN' OR (expected_cash IS NOT NULL AND counted_cash IS NOT NULL AND difference IS NOT NULL AND float_left IS NOT NULL)),
    FOREIGN KEY (tenant_id, user_id) REFERENCES app.users (tenant_id, id)
);
-- A user has at most one open shift: the backstop when two cash actions race to open it.
CREATE UNIQUE INDEX shifts_one_open_per_user ON app.shifts (tenant_id, user_id) WHERE status = 'OPEN';
CREATE INDEX shifts_closed ON app.shifts (tenant_id, closed_at DESC, id DESC) WHERE status = 'CLOSED';

CREATE TABLE app.cash_entries (
    id          text PRIMARY KEY,
    tenant_id   text NOT NULL,
    shift_id    text NOT NULL,
    kind        text NOT NULL CHECK (kind IN ('DEPOSIT', 'PAYMENT', 'REFUND', 'PAYOUT')),
    amount      bigint NOT NULL CHECK (amount > 0),
    stay_id     text,
    payment_id  text,
    description text,
    created_by  text,
    created_at  timestamptz NOT NULL,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, shift_id) REFERENCES app.shifts (tenant_id, id)
);
CREATE INDEX cash_entries_by_shift ON app.cash_entries (tenant_id, shift_id, created_at);

-- A closed shift cannot change: its figures, and the entries behind them, are locked in the database too.
-- +goose StatementBegin
CREATE FUNCTION app.shifts_reject_closed_change() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF OLD.status = 'CLOSED' THEN
        RAISE EXCEPTION 'a closed shift cannot change' USING ERRCODE = 'restrict_violation';
    END IF;
    RETURN NEW;
END
$$;
-- +goose StatementEnd
CREATE TRIGGER shifts_closed_immutable BEFORE UPDATE ON app.shifts
    FOR EACH ROW EXECUTE FUNCTION app.shifts_reject_closed_change();

-- +goose StatementBegin
CREATE FUNCTION app.cash_entries_require_open_shift() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM app.shifts s WHERE s.tenant_id = NEW.tenant_id AND s.id = NEW.shift_id AND s.status = 'OPEN') THEN
        RAISE EXCEPTION 'cash entries need an open shift' USING ERRCODE = 'restrict_violation';
    END IF;
    RETURN NEW;
END
$$;
-- +goose StatementEnd
-- AFTER, so row-level security judges the row before this check does.
CREATE TRIGGER cash_entries_open_shift AFTER INSERT ON app.cash_entries
    FOR EACH ROW EXECUTE FUNCTION app.cash_entries_require_open_shift();

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['shifts', 'cash_entries'] LOOP
        EXECUTE format('ALTER TABLE app.%I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE app.%I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('CREATE POLICY %I ON app.%I USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant())', t || '_tenant', t);
    END LOOP;
END
$$;
-- +goose StatementEnd

GRANT SELECT, INSERT, UPDATE, DELETE ON app.shifts, app.cash_entries TO stayguard_app;
GRANT SELECT, DELETE ON app.shifts, app.cash_entries TO stayguard_maint;
