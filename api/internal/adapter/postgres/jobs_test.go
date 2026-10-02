//go:build integration

package postgres

import (
	"context"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/pcaokhai/stayguard/api/internal/app"
)

// The daily jobs get tenant ids from one narrow function and then work inside a normal tenant scope: the function
// returns ids only, the application role cannot use the cross-tenant policy itself, and a job in tenant A cannot see B.
func TestJobTenantIDs_NarrowAndTenantScoped_SG805(t *testing.T) {
	ctx := context.Background()
	db := migratedDB(t)
	const a, b = "tnt_job_a", "tnt_job_b"
	seedAll(t, db, a)
	seedAll(t, db, b)
	pool := newAppPool(t, db, 4)

	ids, err := TenantIDs(ctx, pool)
	if err != nil || !slices.Contains(ids, a) || !slices.Contains(ids, b) {
		t.Fatalf("ids = %v err=%v", ids, err)
	}

	appConn := connAs(t, db, appRole)
	// Without a tenant the application role sees no tenant rows, with or without the job setting.
	withSettings(ctx, t, appConn, nil, func(tx pgx.Tx) {
		if n := countRows(ctx, t, tx, "tenants"); n != 0 {
			t.Errorf("tenants visible without a tenant: %d", n)
		}
	})
	withSettings(ctx, t, appConn, map[string]string{"app.job_scan": "on"}, func(tx pgx.Tx) {
		if n := countRows(ctx, t, tx, "tenants"); n != 0 {
			t.Errorf("the app role switched the job policy on itself: %d tenant rows", n)
		}
	})
	// The function returns ids only: its result type is text, and it is not callable without the grant.
	maint := connAs(t, db, maintRole)
	_, err = maint.Exec(ctx, `SELECT app.job_tenant_ids()`)
	wantSQLState(t, "maint calls the job function", err, insufficientPrivilege)

	// Inside tenant A's scope a job sees A's rows and none of B's.
	uow := NewUnitOfWork(pool)
	err = uow.Do(ctx, a, func(ctx context.Context, tx app.Tx) error {
		t2, err := pgTx(tx)
		if err != nil {
			return err
		}
		queries := map[string]string{
			"guest_ids":       `SELECT count(*) FROM app.guest_ids WHERE tenant_id <> $1`,
			"guest_id_photos": `SELECT count(*) FROM app.guest_id_photos WHERE tenant_id <> $1`,
			"stays":           `SELECT count(*) FROM app.stays WHERE tenant_id <> $1`,
			"tenants":         `SELECT count(*) FROM app.tenants WHERE id <> $1`,
		}
		for table, q := range queries {
			var other int
			if err = t2.QueryRow(ctx, q, a).Scan(&other); err != nil {
				return err
			}
			if other != 0 {
				t.Errorf("tenant A's job reads %d foreign rows of %s", other, table)
			}
		}
		var own int
		if err = t2.QueryRow(ctx, `SELECT count(*) FROM app.guest_ids`).Scan(&own); err != nil || own != 1 {
			t.Errorf("tenant A's own guest id rows = %d %v", own, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
