//go:build integration

package postgres

import (
	"context"
	"slices"
	"testing"
	"time"
)

// The retention job finds tenants through the maintenance role, which can read guest ID rows across tenants and
// nothing more: it cannot change or delete them (the deletion goes through the application role with an audit entry).
func TestTenantsDue_MaintenanceRoleReadsOnly_SG805_AC5(t *testing.T) {
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
	maint := connAs(t, db, maintRole)
	got, err := TenantsDue(ctx, maint, now)
	if err != nil || !slices.Contains(got, due) || slices.Contains(got, young) {
		t.Fatalf("due = %v err=%v", got, err)
	}
	for _, q := range []string{`DELETE FROM app.guest_ids`, `UPDATE app.guest_ids SET number_enc = NULL`, `DELETE FROM app.guest_id_photos`, `INSERT INTO app.guest_ids (tenant_id, stay_id, consent_at) VALUES ('x', 'y', now())`} {
		_, err := maint.Exec(ctx, q)
		wantSQLState(t, "maint: "+q, err, insufficientPrivilege)
	}
	// A shorter property setting brings the young tenant due.
	if _, err = owner.Exec(ctx, `UPDATE app.properties SET id_retention_days = 3 WHERE tenant_id = $1`, young); err != nil {
		t.Fatal(err)
	}
	if got, err = TenantsDue(ctx, maint, now); err != nil || !slices.Contains(got, young) {
		t.Fatalf("after shortening: %v %v", got, err)
	}
}
