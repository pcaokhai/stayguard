//go:build integration

package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go"
)

func poolErr(t testing.TB, url string, allow bool) error {
	t.Helper()
	pool, err := NewPool(context.Background(), PoolConfig{URL: url, MaxConns: 1, AllowPrivileged: allow})
	if err == nil {
		pool.Close()
	}
	return err
}

func TestPrivilegedRoleRefused_SG003_AC3(t *testing.T) {
	db := migratedDB(t)
	if err := poolErr(t, urlFor(db, appRole, testSecret), false); err != nil {
		t.Fatalf("app role must pass: %v", err)
	}
	if err := poolErr(t, urlFor(db, "owner", testSecret), false); !errors.Is(err, ErrPrivilegedRole) {
		t.Fatalf("owner (superuser) must be refused, got %v", err)
	}
	if err := poolErr(t, urlFor(db, "owner", testSecret), true); err != nil {
		t.Fatalf("opt-out must allow the owner: %v", err)
	}
}

// Roles are cluster-wide, so the membership cases use their own container (as TestRolesPrivilegedAppRole does).
func TestPrivilegedMembershipRefused_SG003_AC3(t *testing.T) {
	url := freshCluster(t)
	ctx := context.Background()
	if _, err := migrateURL(ctx, url); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	admin := connAtURL(t, url)
	for _, q := range []string{
		"ALTER ROLE stayguard_app LOGIN PASSWORD '" + testSecret + "'",
		"GRANT stayguard_maint TO stayguard_app",
	} {
		if _, err := admin.Exec(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	appURL := strings.Replace(url, "owner:", "stayguard_app:", 1)
	if err := poolErr(t, appURL, false); !errors.Is(err, ErrPrivilegedRole) {
		t.Fatalf("member of the BYPASSRLS role must be refused, got %v", err)
	}
}

func TestPrivilegedTableOwnerRefused_SG003_AC3(t *testing.T) {
	url := freshCluster(t)
	ctx := context.Background()
	if _, err := migrateURL(ctx, url); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	admin := connAtURL(t, url)
	for _, q := range []string{
		"ALTER ROLE stayguard_app LOGIN PASSWORD '" + testSecret + "'",
		"ALTER TABLE app.users OWNER TO stayguard_app",
	} {
		if _, err := admin.Exec(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if err := poolErr(t, strings.Replace(url, "owner:", "stayguard_app:", 1), false); !errors.Is(err, ErrPrivilegedRole) {
		t.Fatalf("table owner must be refused, got %v", err)
	}
}

// freshCluster starts a container used by one test and returns its superuser URL.
func freshCluster(t testing.TB) string {
	t.Helper()
	ctr, url, err := startPostgres()
	if err != nil {
		t.Fatalf("start container: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(ctr) })
	return url
}
