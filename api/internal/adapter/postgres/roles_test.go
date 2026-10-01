//go:build integration

package postgres

import (
	"context"
	"testing"
)

func TestRoles_SG003_AC3(t *testing.T) {
	ctx := context.Background()
	db := migratedDB(t)

	// Roles are cluster-wide: migrating a second database must reuse them without error.
	if _, err := migrate(ctx, newEmptyDB(t)); err != nil {
		t.Fatalf("migrate with pre-existing roles: %v", err)
	}

	owner := connAs(t, db, "owner")
	var super, bypass bool
	err := owner.QueryRow(ctx, `SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname = $1`, appRole).Scan(&super, &bypass)
	if err != nil || super || bypass {
		t.Fatalf("app role must be neither superuser nor BYPASSRLS: super=%v bypass=%v err=%v", super, bypass, err)
	}

	app := connAs(t, db, appRole)
	for _, ddl := range []string{
		"CREATE TABLE app.rogue (id int)",
		"CREATE TABLE public.rogue (id int)",
		"DROP TABLE app.users",
		"CREATE SCHEMA rogue",
	} {
		if _, err := app.Exec(ctx, ddl); err == nil {
			t.Errorf("app role must be refused: %s", ddl)
		}
	}

	maint := connAs(t, db, maintRole)
	for _, id := range []string{"tnt_a", "tnt_b"} {
		if _, err := maint.Exec(ctx, `INSERT INTO app.tenants (id, name) VALUES ($1, $1)`, id); err != nil {
			t.Fatalf("maintenance insert %s: %v", id, err)
		}
		if _, err := maint.Exec(ctx, `INSERT INTO app.users (id, tenant_id, name, role) VALUES ($1, $2, 'u', 'OWNER')`, "usr_"+id, id); err != nil {
			t.Fatalf("maintenance insert user of %s: %v", id, err)
		}
	}
	tag, err := maint.Exec(ctx, `DELETE FROM app.users WHERE tenant_id IN ('tnt_a', 'tnt_b')`)
	if err != nil || tag.RowsAffected() != 2 {
		t.Fatalf("maintenance role must delete rows across tenants: affected=%d err=%v", tag.RowsAffected(), err)
	}
}
