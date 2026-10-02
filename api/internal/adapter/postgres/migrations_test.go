//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// appTables are the tables SG-003 creates (plan Files); tenants is scoped by its own id.
var appTables = []string{
	"tenants", "users", "sessions", "properties", "buildings", "floors", "unit_types", "units",
	"stays", "services", "stay_extras", "invoices", "payments", "payment_events",
	"audit_logs", "idempotency_keys",
	"pin_credentials", "staff_profiles", "building_permissions", "bank_accounts", "stock_movements", "stocktakes",
	"stay_edits", "alerts", // 0013 (L-B1)
	"shifts", "cash_entries", // 0014 (L-B2)
	"maintenance_tickets",                  // 0016 (L-B4)
	"roster_assignments", "leave_requests", // 0019 (F-A3)
	"expenses", "payroll_lines", "recurring_runs", // 0020 (F-A4)
}

func queryStrings(t testing.TB, db, q string) []string {
	t.Helper()
	ctx := context.Background()
	rows, err := connAs(t, db, "owner").Query(ctx, q)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return out
}

func TestMigrations_SG003_AC1(t *testing.T) {
	ctx := context.Background()
	db := newEmptyDB(t)

	if n, err := migrate(ctx, db); err != nil || n == 0 {
		t.Fatalf("first up: applied=%d err=%v", n, err)
	}
	if n, err := migrate(ctx, db); err != nil || n != 0 {
		t.Fatalf("second up must be a no-op: applied=%d err=%v", n, err)
	}

	got := queryStrings(t, db, `SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'app' AND c.relkind = 'r' ORDER BY 1`)
	want := append([]string(nil), appTables...)
	sortStrings(want)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("tables in schema app (-want +got):\n%s", diff)
	}

	noTenant := queryStrings(t, db, `SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'app' AND c.relkind = 'r' AND c.relname <> 'tenants'
		AND NOT EXISTS (SELECT 1 FROM pg_attribute a WHERE a.attrelid = c.oid AND a.attname = 'tenant_id' AND NOT a.attisdropped)
		ORDER BY 1`)
	if len(noTenant) != 0 {
		t.Errorf("tables without tenant_id: %v", noTenant)
	}
	if got := rlsOffenders(t, db); len(got) != 0 {
		t.Errorf("tables without enforced RLS and a tenant policy: %v", got)
	}
}
