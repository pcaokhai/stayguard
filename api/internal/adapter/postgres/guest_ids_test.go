//go:build integration

package postgres

import (
	"context"
	"slices"
	"testing"
	"time"
)

// The retention job finds tenants through one database function the application role can run: it returns tenant
// ids across tenants, while a plain query as that role (no tenant set) still sees no guest ID rows.
func TestTenantsDue_AppRoleGetsIdsOnly_SG805_AC5(t *testing.T) {
	ctx := context.Background()
	db := migratedDB(t)
	const due, young = "tnt_gid_due", "tnt_gid_young"
	seedAll(t, db, due)
	seedAll(t, db, young)
	owner := connAs(t, db, "owner")
	now := time.Now().UTC()
	set := func(tenant string, ago time.Duration) {
		t.Helper()
		if _, err := owner.Exec(ctx, `UPDATE app.stays SET status = 'CHECKED_OUT', check_out_at = $2 WHERE tenant_id = $1`, tenant, now.Add(-ago)); err != nil {
			t.Fatal(err)
		}
	}
	set(due, 31*24*time.Hour)
	set(young, 5*24*time.Hour)
	appConn := connAs(t, db, appRole)
	var rows int
	if err := appConn.QueryRow(ctx, `SELECT count(*) FROM app.guest_ids`).Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("app role sees %d guest id rows without a tenant (err=%v)", rows, err)
	}
	got, err := TenantsDue(ctx, appConn, now)
	if err != nil || !slices.Contains(got, due) || slices.Contains(got, young) {
		t.Fatalf("due = %v err=%v", got, err)
	}
	// A shorter property setting brings the young tenant due.
	if _, err = owner.Exec(ctx, `UPDATE app.properties SET id_retention_days = 3 WHERE tenant_id = $1`, young); err != nil {
		t.Fatal(err)
	}
	if got, err = TenantsDue(ctx, appConn, now); err != nil || !slices.Contains(got, young) {
		t.Fatalf("after shortening: %v %v", got, err)
	}
}
