-- +goose Up
-- F-A3: who works which shift, and leave requests. A person may work two shifts on one day.
CREATE TABLE app.roster_assignments (
    tenant_id  text NOT NULL,
    user_id    text NOT NULL,
    work_date  date NOT NULL,
    shift      text NOT NULL CHECK (shift IN ('MORNING', 'AFTERNOON', 'NIGHT')),
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, user_id, work_date, shift),
    FOREIGN KEY (tenant_id, user_id) REFERENCES app.users (tenant_id, id)
);
CREATE INDEX roster_assignments_by_date ON app.roster_assignments (tenant_id, work_date);

CREATE TABLE app.leave_requests (
    id             text PRIMARY KEY,
    tenant_id      text NOT NULL,
    user_id        text NOT NULL,
    from_date      date NOT NULL,
    to_date        date NOT NULL,
    shift          text CHECK (shift IN ('MORNING', 'AFTERNOON', 'NIGHT')),
    kind           text NOT NULL CHECK (kind IN ('PAID', 'SICK', 'UNPAID')),
    reason         text,
    cover_user_id  text,
    status         text NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'APPROVED', 'DECLINED', 'CANCELLED', 'CANCEL_REQUESTED')),
    decline_reason text,
    created_at     timestamptz NOT NULL,
    decided_at     timestamptz,
    decided_by     text,
    UNIQUE (tenant_id, id),
    CHECK (to_date >= from_date),
    FOREIGN KEY (tenant_id, user_id) REFERENCES app.users (tenant_id, id)
);
CREATE INDEX leave_requests_by_user ON app.leave_requests (tenant_id, user_id, from_date);
CREATE INDEX leave_requests_by_status ON app.leave_requests (tenant_id, status, from_date);

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['roster_assignments', 'leave_requests'] LOOP
        EXECUTE format('ALTER TABLE app.%I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE app.%I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('CREATE POLICY %I ON app.%I USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant())', t || '_tenant', t);
        EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON app.%I TO stayguard_app', t);
        EXECUTE format('GRANT SELECT, DELETE ON app.%I TO stayguard_maint', t);
    END LOOP;
END
$$;
-- +goose StatementEnd
