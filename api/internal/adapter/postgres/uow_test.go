//go:build integration

package postgres

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pcaokhai/stayguard/api/internal/app"
)

// newAppPool opens a pool as the application role against db.
func newAppPool(t testing.TB, db string, maxConns int32) *pgxpool.Pool {
	t.Helper()
	pool, err := NewPool(context.Background(), PoolConfig{URL: urlFor(db, appRole, testSecret), MaxConns: maxConns})
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedTenant creates a tenant with one user through the owner connection.
func seedTenant(t testing.TB, db, id string) {
	t.Helper()
	ctx := context.Background()
	owner := connAs(t, db, "owner")
	if _, err := owner.Exec(ctx, `INSERT INTO app.tenants (id, name) VALUES ($1, $1)`, id); err != nil {
		t.Fatalf("seed tenant %s: %v", id, err)
	}
	if _, err := owner.Exec(ctx, `INSERT INTO app.users (id, tenant_id, name, role) VALUES ($1, $2, 'u', 'OWNER')`, "usr_"+id, id); err != nil {
		t.Fatalf("seed user %s: %v", id, err)
	}
}

func userIDs(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}) ([]string, error) {
	rows, err := q.Query(ctx, `SELECT id FROM app.users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// assertOnlySeededUser checks, in a fresh unit of work, that earlier rolled-back inserts left nothing.
func assertOnlySeededUser(ctx context.Context, t *testing.T, uow *UnitOfWork) {
	t.Helper()
	ran := false
	err := uow.Do(ctx, "tnt_ac4_a", func(ctx context.Context, tx app.Tx) error {
		ran = true
		ids, err := userIDs(ctx, tx.(Tx))
		if err != nil || len(ids) != 1 {
			t.Errorf("rolled-back insert must not persist, got %v err=%v", ids, err)
		}
		return nil
	})
	if err != nil || !ran {
		t.Fatalf("verification unit of work: ran=%v err=%v", ran, err)
	}
}

func TestUnitOfWork_SG003_AC4(t *testing.T) {
	ctx := context.Background()
	db := migratedDB(t)
	seedTenant(t, db, "tnt_ac4_a")
	seedTenant(t, db, "tnt_ac4_b")
	pool := newAppPool(t, db, 1) // one connection forces reuse between transactions
	uow := NewUnitOfWork(pool)

	t.Run("tenant sees only its rows", func(t *testing.T) {
		err := uow.Do(ctx, "tnt_ac4_a", func(ctx context.Context, tx app.Tx) error {
			ids, err := userIDs(ctx, tx.(Tx))
			if err != nil || len(ids) != 1 || ids[0] != "usr_tnt_ac4_a" {
				t.Errorf("want only tenant A's user, got %v err=%v", ids, err)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("no tenant set returns zero rows without error", func(t *testing.T) {
		ids, err := userIDs(ctx, pool)
		if err != nil || len(ids) != 0 {
			t.Errorf("want zero rows and no error, got %v err=%v", ids, err)
		}
	})

	t.Run("setting does not leak to the next transaction on the same connection", func(t *testing.T) {
		if err := uow.Do(ctx, "tnt_ac4_a", func(context.Context, app.Tx) error { return nil }); err != nil {
			t.Fatal(err)
		}
		var unset bool
		if err := pool.QueryRow(ctx, `SELECT app.current_tenant() IS NULL`).Scan(&unset); err != nil || !unset {
			t.Errorf("tenant leaked past the transaction: unset=%v err=%v", unset, err)
		}
		if ids, err := userIDs(ctx, pool); err != nil || len(ids) != 0 {
			t.Errorf("want zero rows after Do, got %v err=%v", ids, err)
		}
	})

	t.Run("insert for another tenant is rejected", func(t *testing.T) {
		err := uow.Do(ctx, "tnt_ac4_a", func(ctx context.Context, tx app.Tx) error {
			_, err := tx.(Tx).Exec(ctx, `INSERT INTO app.users (id, tenant_id, name, role) VALUES ('usr_x', 'tnt_ac4_b', 'x', 'OWNER')`)
			return err
		})
		wantRLSViolation(t, "cross-tenant insert", err)
	})

	t.Run("empty tenant fails before any query", func(t *testing.T) {
		called := false
		err := uow.Do(ctx, "", func(context.Context, app.Tx) error { called = true; return nil })
		if !errors.Is(err, app.ErrTenantRequired) || called {
			t.Errorf("want ErrTenantRequired and no call, got %v called=%v", err, called)
		}
	})

	t.Run("rolls back on error and on panic", func(t *testing.T) {
		insert := func(ctx context.Context, tx app.Tx) error {
			_, err := tx.(Tx).Exec(ctx, `INSERT INTO app.users (id, tenant_id, name, role) VALUES ('usr_rb', 'tnt_ac4_a', 'rb', 'OWNER')`)
			return err
		}
		boom := errors.New("boom")
		err := uow.Do(ctx, "tnt_ac4_a", func(ctx context.Context, tx app.Tx) error {
			if err := insert(ctx, tx); err != nil {
				return err
			}
			return boom
		})
		if !errors.Is(err, boom) {
			t.Errorf("want fn error back, got %v", err)
		}
		err = uow.Do(ctx, "tnt_ac4_a", func(ctx context.Context, tx app.Tx) error {
			if err := insert(ctx, tx); err != nil {
				return err
			}
			panic("kaboom")
		})
		var pe *PanicError
		if !errors.As(err, &pe) {
			t.Errorf("want PanicError, got %v", err)
		} else if strings.Contains(err.Error(), "kaboom") {
			t.Errorf("error text must not carry the panic value: %v", err)
		}
		assertOnlySeededUser(ctx, t, uow)
	})

	t.Run("rolls back on runtime.Goexit and frees the connection", func(t *testing.T) {
		done := make(chan struct{})
		go func() {
			defer close(done)
			_ = uow.Do(ctx, "tnt_ac4_a", func(ctx context.Context, tx app.Tx) error {
				if _, err := tx.(Tx).Exec(ctx, `INSERT INTO app.users (id, tenant_id, name, role) VALUES ('usr_rb', 'tnt_ac4_a', 'rb', 'OWNER')`); err != nil {
					t.Errorf("insert: %v", err)
				}
				runtime.Goexit()
				return nil
			})
		}()
		<-done
		assertOnlySeededUser(ctx, t, uow) // would block forever on MaxConns=1 if the tx stayed open
	})
}
