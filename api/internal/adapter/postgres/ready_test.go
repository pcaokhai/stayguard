//go:build integration

package postgres

import (
	"context"
	"testing"
)

// probeFor builds a probe over a pool connected as the application role.
func probeFor(t testing.TB, db string) *ReadinessProbe {
	t.Helper()
	return probeAs(t, db, appRole)
}

func probeAs(t testing.TB, db, role string) *ReadinessProbe {
	t.Helper()
	pool, err := NewPool(context.Background(), PoolConfig{URL: urlFor(db, role, testSecret), MaxConns: 2, AllowPrivileged: role == "owner"})
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return NewReadinessProbe(pool)
}

func TestReadyz_SG003_AC6(t *testing.T) {
	ctx := context.Background()

	// Unmigrated: the database answers but no goose table exists (the app role does not exist yet, so
	// the owner connects).
	if err := probeAs(t, newEmptyDB(t), "owner").Check(ctx); err == nil {
		t.Fatal("unmigrated database must not be ready")
	}

	// Migrated: ready.
	if err := probeFor(t, migratedDB(t)).Check(ctx); err != nil {
		t.Fatalf("migrated database must be ready: %v", err)
	}
}

func TestReadyzPending_SG003_AC6(t *testing.T) {
	ctx := context.Background()
	db := newEmptyDB(t)
	if err := Migrate(ctx, urlFor(db, "owner", testSecret), false); err != nil {
		t.Fatal(err)
	}
	if err := enableLogins(ctx); err != nil {
		t.Fatal(err)
	}
	// Forget the last applied migration: the binary knows a version the database lacks.
	owner := connAs(t, db, "owner")
	if _, err := owner.Exec(ctx, "DELETE FROM goose_db_version WHERE version_id = (SELECT max(version_id) FROM goose_db_version)"); err != nil {
		t.Fatal(err)
	}
	if err := probeFor(t, db).Check(ctx); err == nil {
		t.Fatal("pending migration must not be ready")
	}
}

func TestMigrateCommand_SG003_AC6(t *testing.T) {
	ctx := context.Background()
	db := newEmptyDB(t)
	url := urlFor(db, "owner", testSecret)
	if err := Migrate(ctx, url, false); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := Migrate(ctx, url, false); err != nil {
		t.Fatalf("second migrate must be a no-op: %v", err)
	}
	if err := enableLogins(ctx); err != nil {
		t.Fatal(err)
	}
	if err := probeFor(t, db).Check(ctx); err != nil {
		t.Fatalf("not ready after migrate: %v", err)
	}
}
