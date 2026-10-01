//go:build integration

package postgres

import (
	"context"
	"slices"
	"testing"
)

func sortStrings(s []string) { slices.Sort(s) }

// rlsOffenders lists tables in schema app whose RLS is off or not forced, that have no policy, or
// that have a policy not keyed on the current tenant.
func rlsOffenders(t testing.TB, db string) []string {
	t.Helper()
	return queryStrings(t, db, rlsOffenderSQL)
}

const rlsOffenderSQL = `SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
	WHERE n.nspname = 'app' AND c.relkind IN ('r', 'p') AND (
		NOT c.relrowsecurity OR NOT c.relforcerowsecurity
		OR NOT EXISTS (SELECT 1 FROM pg_policy p WHERE p.polrelid = c.oid)
		OR EXISTS (SELECT 1 FROM pg_policy p WHERE p.polrelid = c.oid
			AND coalesce(pg_get_expr(p.polqual, p.polrelid), '') || coalesce(pg_get_expr(p.polwithcheck, p.polrelid), '')
			NOT LIKE '%app.current_tenant()%'))
	ORDER BY 1`

func TestRlsCoverage_SG003_AC2(t *testing.T) {
	db := migratedDB(t)
	if got := rlsOffenders(t, db); len(got) != 0 {
		t.Fatalf("tables without enforced RLS and a tenant policy: %v", got)
	}

	// Red proof: a table added without a policy must be reported, then discarded.
	ctx := context.Background()
	conn := connAs(t, db, "owner")
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "CREATE TABLE app.throwaway (id text PRIMARY KEY, tenant_id text)"); err != nil {
		t.Fatalf("create throwaway: %v", err)
	}
	rows, err := tx.Query(ctx, rlsOffenderSQL)
	if err != nil {
		t.Fatalf("offender query: %v", err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, s)
	}
	if !slices.Equal(got, []string{"throwaway"}) {
		t.Errorf("offender check must flag exactly the throwaway table, got %v", got)
	}
}
