-- +goose Up
-- F-A2: guest ID numbers and photos move out of the stays table into tables that only the guest ID code reads.
-- Stay queries keep only yes/no indicators (an EXISTS), so no front-desk query can select the data.

CREATE TABLE app.guest_ids (
    tenant_id  text NOT NULL,
    stay_id    text NOT NULL,
    -- AES-GCM of the number; NULL when only the consent (for photos) is on file.
    number_enc bytea,
    consent_at timestamptz NOT NULL,
    consent_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, stay_id),
    FOREIGN KEY (tenant_id, stay_id) REFERENCES app.stays (tenant_id, id)
);

CREATE TABLE app.guest_id_photos (
    tenant_id   text NOT NULL,
    stay_id     text NOT NULL,
    side        text NOT NULL CHECK (side IN ('FRONT', 'BACK')),
    -- AES-GCM of the re-encoded JPEG; the bytes never leave the API except through the owner endpoints.
    image_enc   bytea NOT NULL,
    bytes       integer NOT NULL CHECK (bytes > 0),
    uploaded_at timestamptz NOT NULL,
    uploaded_by text,
    PRIMARY KEY (tenant_id, stay_id, side),
    FOREIGN KEY (tenant_id, stay_id) REFERENCES app.stays (tenant_id, id)
);

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['guest_ids', 'guest_id_photos'] LOOP
        EXECUTE format('ALTER TABLE app.%I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE app.%I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('CREATE POLICY %I ON app.%I USING (tenant_id = app.current_tenant()) WITH CHECK (tenant_id = app.current_tenant())', t || '_tenant', t);
        EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON app.%I TO stayguard_app', t);
    END LOOP;
END
$$;
-- +goose StatementEnd

-- Retention needs the tenant ids with expired data, across tenants. The application role gets only this function
-- (ids, no rows); it is owned by the BYPASSRLS role and the job then deletes tenant by tenant under RLS.
CREATE FUNCTION app.tenants_with_expired_guest_ids(p_now timestamptz) RETURNS SETOF text
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, app
    AS $$
    SELECT DISTINCT g.tenant_id
    FROM app.guest_ids g
    JOIN app.stays s ON s.tenant_id = g.tenant_id AND s.id = g.stay_id
    JOIN app.properties pr ON pr.tenant_id = g.tenant_id
    WHERE s.check_out_at IS NOT NULL AND s.check_out_at + pr.id_retention_days * interval '1 day' < p_now
    ORDER BY 1
$$;
REVOKE ALL ON FUNCTION app.tenants_with_expired_guest_ids(timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION app.tenants_with_expired_guest_ids(timestamptz) TO stayguard_app;
GRANT SELECT ON app.guest_ids, app.guest_id_photos, app.stays, app.properties TO stayguard_maint;
ALTER FUNCTION app.tenants_with_expired_guest_ids(timestamptz) OWNER TO stayguard_maint;

-- Numbers entered at check-in so far keep their ciphertext (same tenant and field binding) and count as consented then.
INSERT INTO app.guest_ids (tenant_id, stay_id, number_enc, consent_at)
SELECT tenant_id, id, id_number_enc, check_in_at FROM app.stays WHERE id_number_enc IS NOT NULL;
ALTER TABLE app.stays DROP COLUMN id_number_enc;

-- Daily jobs (guest ID retention, later recurring expenses and leave status) visit every tenant, one normal tenant-scoped
-- transaction each. The only cross-tenant read is this function: it returns tenant ids and nothing else, and only the
-- application role may call it. It works under any table owner: the policy below lets rows through only while the
-- definer function is running (current_user differs from session_user there), so the application role cannot
-- switch it on itself.
CREATE FUNCTION app.job_scan() RETURNS text
    LANGUAGE sql STABLE
    AS $$ SELECT nullif(current_setting('app.job_scan', true), '') $$;

CREATE POLICY tenants_job_scan ON app.tenants FOR SELECT
    USING (app.current_tenant() IS NULL AND app.job_scan() = 'on' AND current_user <> session_user);

-- +goose StatementBegin
CREATE FUNCTION app.job_tenant_ids() RETURNS SETOF text
    LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, app
    AS $$
BEGIN
    PERFORM set_config('app.job_scan', 'on', true);
    RETURN QUERY SELECT t.id FROM app.tenants t ORDER BY t.id;
    PERFORM set_config('app.job_scan', '', true);
END
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION app.job_tenant_ids() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION app.job_tenant_ids() TO stayguard_app;
