//go:build integration

package postgres

import (
	"context"
	"testing"
)

// fixture inserts one row into a table for tenant $1 with ids derived from prefix $2.
// Every statement uses both parameters so the same text seeds a tenant (owner connection) and
// builds the "insert naming another tenant" attack (app connection).
type fixture struct{ table, sql string }

// isolationFixtures holds one row per table of schema app, parents first. A table in the catalog
// without an entry fails TS-08 until someone adds a fixture, so new tables cannot skip the suite.
var isolationFixtures = []fixture{
	{"tenants", `INSERT INTO app.tenants (id, name) VALUES ($1::text, $2::text)`},
	{"users", `INSERT INTO app.users (id, tenant_id, name, role) VALUES ($2::text || '_u', $1::text, 'u', 'OWNER')`},
	{"pin_credentials", `INSERT INTO app.pin_credentials (tenant_id, user_id, pin_hash) VALUES ($1::text, $2::text || '_u', 'h')`},
	{"staff_profiles", `INSERT INTO app.staff_profiles (tenant_id, user_id, position, pay_type, rate, fixed_allowance, standard_shifts, start_date, annual_leave_days) VALUES ($1::text, $2::text || '_u', 'OTHER', 'MONTHLY', 0, 0, 0, '2026-01-01', 0)`},
	{"bank_accounts", `INSERT INTO app.bank_accounts (id, tenant_id, bank_bin, bank_name, account_enc, account_no_masked, account_name, account_fp) VALUES ($2::text || '_ba', $1::text, '970436', 'B', '\x00', '****', 'n', $2::text || '_fp')`},
	{"sessions", `INSERT INTO app.sessions (token_hash, tenant_id, user_id, expires_at) VALUES ($2::text || '_s', $1::text, $2::text || '_u', now() + interval '1 hour')`},
	{"properties", `INSERT INTO app.properties (id, tenant_id, name) VALUES ($2::text || '_p', $1::text, 'p')`},
	{"buildings", `INSERT INTO app.buildings (id, tenant_id, property_id, code, name) VALUES ($2::text || '_b', $1::text, $2::text || '_p', 'B', 'b')`},
	{"floors", `INSERT INTO app.floors (id, tenant_id, building_id, level) VALUES ($2::text || '_f', $1::text, $2::text || '_b', 1)`},
	{"unit_types", `INSERT INTO app.unit_types (id, tenant_id, code, name, rate_plan, rate_plan_version) VALUES ($2::text || '_ut', $1::text, 'STD', '{}', '{}', 1)`},
	{"building_permissions", `INSERT INTO app.building_permissions (tenant_id, user_id, building_id, level) VALUES ($1::text, $2::text || '_u', $2::text || '_b', 'VIEW')`},
	{"units", `INSERT INTO app.units (id, tenant_id, building_id, floor_id, unit_type_id, code) VALUES ($2::text || '_un', $1::text, $2::text || '_b', $2::text || '_f', $2::text || '_ut', 'R1')`},
	{"stays", `INSERT INTO app.stays (id, tenant_id, unit_id, rental_type, guest_name, guest_phone, rate_plan_snapshot, rate_plan_schema) VALUES ($2::text || '_st', $1::text, $2::text || '_un', 'DAILY', 'g', '0900000000', '{}', 1)`},
	{"services", `INSERT INTO app.services (id, tenant_id, code, name, price) VALUES ($2::text || '_sv', $1::text, 'SV', '{}', 1)`},
	{"stay_extras", `INSERT INTO app.stay_extras (id, tenant_id, stay_id, service_id, quantity, unit_amount, amount) VALUES ($2::text || '_se', $1::text, $2::text || '_st', $2::text || '_sv', 1, 1, 1)`},
	{"invoices", `INSERT INTO app.invoices (id, tenant_id, stay_id, bill_code, quote, total) VALUES ($2::text || '_iv', $1::text, $2::text || '_st', 'BC', '{}', 1)`},
	{"payments", `INSERT INTO app.payments (id, tenant_id, invoice_id, method, status, amount) VALUES ($2::text || '_pm', $1::text, $2::text || '_iv', 'CASH', 'PENDING', 1)`},
	{"payment_events", `INSERT INTO app.payment_events (id, tenant_id, provider, external_id, amount, result) VALUES ($2::text || '_pe', $1::text, 'sim', $2::text || '_x', 1, 'SETTLED')`},
	{"stay_edits", `INSERT INTO app.stay_edits (id, tenant_id, stay_id, kind) VALUES ($2::text || '_sd', $1::text, $2::text || '_st', 'CHECK_IN')`},
	{"alerts", `INSERT INTO app.alerts (id, tenant_id, kind) VALUES ($2::text || '_al', $1::text, 'CASH_OVER')`},
	{"shifts", `INSERT INTO app.shifts (id, tenant_id, user_id, shift_code, opened_at) VALUES ($2::text || '_sh', $1::text, $2::text || '_u', 'MORNING', now())`},
	{"cash_entries", `INSERT INTO app.cash_entries (id, tenant_id, shift_id, kind, amount, created_at) VALUES ($2::text || '_ce', $1::text, $2::text || '_sh', 'PAYOUT', 1, now())`},
	{"audit_logs", `INSERT INTO app.audit_logs (id, tenant_id, action, entity_type, entity_id) VALUES ($2::text || '_au', $1::text, 'a', 'e', $2::text)`},
	{"idempotency_keys", `INSERT INTO app.idempotency_keys (tenant_id, route, key, request_hash, expires_at) VALUES ($1::text, 'r', $2::text, 'h', now() + interval '1 hour')`},
}

// seedAll inserts every fixture for the tenant through the owner connection (not the maintenance role).
func seedAll(t testing.TB, db, tenant string) {
	t.Helper()
	owner := connAs(t, db, "owner")
	for _, f := range isolationFixtures {
		if _, err := owner.Exec(context.Background(), f.sql, tenant, tenant); err != nil {
			t.Fatalf("seed %s for %s: %v", f.table, tenant, err)
		}
	}
}

// catalogTables lists every ordinary table of schema app with the column holding its tenant.
func catalogTables(t testing.TB, db string) map[string]string {
	t.Helper()
	rows, err := connAs(t, db, "owner").Query(context.Background(), `
		SELECT c.relname,
		       CASE WHEN EXISTS (SELECT 1 FROM pg_attribute a WHERE a.attrelid = c.oid AND a.attname = 'tenant_id' AND NOT a.attisdropped)
		            THEN 'tenant_id' WHEN c.relname = 'tenants' THEN 'id' ELSE '' END
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'app' AND c.relkind IN ('r', 'p')`)
	if err != nil {
		t.Fatalf("catalog query: %v", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var table, col string
		if err := rows.Scan(&table, &col); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out[table] = col
	}
	return out
}
