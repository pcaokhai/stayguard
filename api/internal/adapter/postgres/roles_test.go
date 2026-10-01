//go:build integration

package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/testcontainers/testcontainers-go"
)

const insufficientPrivilege = "42501"

func wantSQLState(t testing.TB, what string, err error, code string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Errorf("%s: want SQLSTATE %s, got %v", what, code, err)
	}
}

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
	assertAppPrivileges(t, db)

	app := connAs(t, db, appRole)
	for _, ddl := range []string{
		"CREATE TABLE app.rogue (id int)",
		"CREATE TABLE public.rogue (id int)",
		"DROP TABLE app.users",
		"CREATE SCHEMA rogue",
	} {
		_, err := app.Exec(ctx, ddl)
		wantSQLState(t, "app role: "+ddl, err, insufficientPrivilege)
	}
	assertMaintScope(t, db)
}

// assertAppPrivileges checks no TRUNCATE anywhere and no UPDATE or DELETE on audit_logs.
func assertAppPrivileges(t testing.TB, db string) {
	t.Helper()
	ctx := context.Background()
	owner := connAs(t, db, "owner")
	var truncate []string
	rows, err := owner.Query(ctx, `SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'app' AND c.relkind = 'r' AND has_table_privilege($1, c.oid, 'TRUNCATE')`, appRole)
	if err != nil {
		t.Fatalf("truncate privilege query: %v", err)
	}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan: %v", err)
		}
		truncate = append(truncate, s)
	}
	rows.Close()
	if len(truncate) != 0 {
		t.Errorf("app role must not hold TRUNCATE: %v", truncate)
	}
	for _, priv := range []string{"UPDATE", "DELETE"} {
		var has bool
		if err := owner.QueryRow(ctx, `SELECT has_table_privilege($1, 'app.audit_logs', $2)`, appRole, priv).Scan(&has); err != nil || has {
			t.Errorf("app role must not hold %s on audit_logs: has=%v err=%v", priv, has, err)
		}
	}
}

// assertMaintScope checks the maintenance role deletes across tenants but cannot write or change schema.
func assertMaintScope(t testing.TB, db string) {
	t.Helper()
	ctx := context.Background()
	owner := connAs(t, db, "owner") // fixtures come from the owner, never from the maintenance role
	ids := []string{"tnt_ac3_a", "tnt_ac3_b"}
	for _, id := range ids {
		if _, err := owner.Exec(ctx, `INSERT INTO app.tenants (id, name) VALUES ($1, $1)`, id); err != nil {
			t.Fatalf("insert %s: %v", id, err)
		}
		if _, err := owner.Exec(ctx, `INSERT INTO app.users (id, tenant_id, name, role) VALUES ($1, $2, 'u', 'OWNER')`, "usr_"+id, id); err != nil {
			t.Fatalf("insert user of %s: %v", id, err)
		}
	}
	maint := connAs(t, db, maintRole)
	_, err := maint.Exec(ctx, `UPDATE app.invoices SET status = 'PAID'`)
	wantSQLState(t, "maint UPDATE", err, insufficientPrivilege)
	_, err = maint.Exec(ctx, `INSERT INTO app.tenants (id, name) VALUES ('tnt_ac3_c', 'c')`)
	wantSQLState(t, "maint INSERT", err, insufficientPrivilege)
	_, err = maint.Exec(ctx, "CREATE TABLE app.rogue (id int)")
	wantSQLState(t, "maint DDL", err, insufficientPrivilege)

	tag, err := maint.Exec(ctx, `DELETE FROM app.users WHERE tenant_id IN ('tnt_ac3_a', 'tnt_ac3_b')`)
	if err != nil || tag.RowsAffected() != 2 {
		t.Errorf("maintenance role must delete rows across tenants: affected=%d err=%v", tag.RowsAffected(), err)
	}
	if _, err := maint.Exec(ctx, `DELETE FROM app.tenants WHERE id IN ('tnt_ac3_a', 'tnt_ac3_b')`); err != nil {
		t.Errorf("fixture cleanup: %v", err)
	}
}

// A pre-created app role with a privileged attribute must stop the migration, not be trusted.
// It uses its own container because roles are cluster-wide and must not pollute the shared one.
func TestRolesPrivilegedAppRole_SG003_AC3(t *testing.T) {
	ctr, url, err := startPostgres()
	if err != nil {
		t.Fatalf("start container: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(ctr) })
	ctx := context.Background()
	admin := connAtURL(t, url)
	if _, err := admin.Exec(ctx, "CREATE ROLE stayguard_app NOLOGIN BYPASSRLS"); err != nil {
		t.Fatalf("pre-create role: %v", err)
	}
	_, err = migrateURL(ctx, url)
	if err == nil || !strings.Contains(err.Error(), "privileged attribute") {
		t.Fatalf("migration must fail on a BYPASSRLS app role, got %v", err)
	}
}

// The maintenance role is the only BYPASSRLS role: a pre-created one with more power must stop the migration.
func TestRolesPrivilegedMaintRole_SG003_AC3(t *testing.T) {
	url := freshCluster(t)
	ctx := context.Background()
	if _, err := connAtURL(t, url).Exec(ctx, "CREATE ROLE stayguard_maint NOLOGIN BYPASSRLS CREATEROLE"); err != nil {
		t.Fatalf("pre-create role: %v", err)
	}
	if _, err := migrateURL(ctx, url); err == nil || !strings.Contains(err.Error(), "stayguard_maint") {
		t.Fatalf("migration must fail on a CREATEROLE maint role, got %v", err)
	}
}

// An app role that is a member of a BYPASSRLS or superuser role inherits its power: refuse it at migration.
func TestRolesAppMemberOfPrivileged_SG003_AC3(t *testing.T) {
	url := freshCluster(t)
	ctx := context.Background()
	admin := connAtURL(t, url)
	for _, q := range []string{
		"CREATE ROLE stayguard_maint NOLOGIN BYPASSRLS",
		"CREATE ROLE stayguard_app NOLOGIN",
		"GRANT stayguard_maint TO stayguard_app",
	} {
		if _, err := admin.Exec(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if _, err := migrateURL(ctx, url); err == nil || !strings.Contains(err.Error(), "member") {
		t.Fatalf("migration must fail when the app role is a member of a privileged role, got %v", err)
	}
}
