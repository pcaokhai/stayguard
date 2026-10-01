//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pcaokhai/stayguard/api/internal/app"
)

const (
	tenantA         = "tnt_ts08_a"
	tenantB         = "tnt_ts08_b"
	foreignKeyCode  = "23503"
	unmatchedEvents = "pe_ts08_null"
)

// actor runs statements as the app role for one tenant, or with no tenant when tenant is empty.
type actor struct {
	pool   *pgxpool.Pool
	uow    *UnitOfWork
	tenant string
}

// exec runs a statement and returns the rows it touched.
func (a actor) exec(ctx context.Context, sql string, args ...any) (n int64, err error) {
	if a.tenant == "" {
		tag, err := a.pool.Exec(ctx, sql, args...)
		return tag.RowsAffected(), err
	}
	err = a.uow.Do(ctx, a.tenant, func(ctx context.Context, tx app.Tx) error {
		tag, err := tx.(Tx).Exec(ctx, sql, args...)
		n = tag.RowsAffected()
		return err
	})
	return n, err
}

// count runs a count(*) query.
func (a actor) count(ctx context.Context, sql string, args ...any) (n int64, err error) {
	if a.tenant == "" {
		err = a.pool.QueryRow(ctx, sql, args...).Scan(&n)
		return n, err
	}
	err = a.uow.Do(ctx, a.tenant, func(ctx context.Context, tx app.Tx) error {
		return tx.(Tx).QueryRow(ctx, sql, args...).Scan(&n)
	})
	return n, err
}

func sqlState(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// wantUntouched accepts zero rows, or a privilege denial (the app role has no UPDATE or DELETE on audit_logs).
func wantUntouched(t testing.TB, what string, n int64, err error) {
	t.Helper()
	if err != nil && sqlState(err) != insufficientPrivilege {
		t.Errorf("%s: unexpected error %v", what, err)
	} else if err == nil && n != 0 {
		t.Errorf("%s: touched %d rows of another tenant", what, n)
	}
}

func TestTenantIsolation_SG003_TS08(t *testing.T) {
	ctx := context.Background()
	db := migratedDB(t)
	seedAll(t, db, tenantA)
	seedAll(t, db, tenantB)
	pool := newAppPool(t, db, 2)
	uow := NewUnitOfWork(pool)
	a, b, none := actor{pool, uow, tenantA}, actor{pool, uow, tenantB}, actor{pool, uow, ""}

	tables := catalogTables(t, db)
	assertFixturesCoverCatalog(ctx, t, db, tables)
	for table, col := range tables {
		q := pgx.Identifier{"app", table}.Sanitize()
		t.Run("A vs B "+table, func(t *testing.T) { checkOtherTenant(ctx, t, a, q, col, tenantB) })
		t.Run("B vs A "+table, func(t *testing.T) { checkOtherTenant(ctx, t, b, q, col, tenantA) })
		t.Run("no tenant "+table, func(t *testing.T) { checkOtherTenant(ctx, t, none, q, col, tenantA) })
	}
	t.Run("insert naming another tenant", func(t *testing.T) {
		for _, f := range isolationFixtures {
			_, err := a.exec(ctx, f.sql, tenantB, "bad")
			wantSQLState(t, "A inserts into "+f.table+" for B", err, insufficientPrivilege)
		}
	})
	t.Run("composite foreign key", func(t *testing.T) { checkCompositeFK(ctx, t, a) })
	t.Run("NULL tenant payment events", func(t *testing.T) { checkNullTenantEvents(ctx, t, db, a, b, none) })
}

// assertFixturesCoverCatalog fails when a catalog table has no fixture, or a seeded tenant has no row in it.
func assertFixturesCoverCatalog(ctx context.Context, t *testing.T, db string, tables map[string]string) {
	t.Helper()
	owner := connAs(t, db, "owner")
	have := map[string]bool{}
	for _, f := range isolationFixtures {
		have[f.table] = true
	}
	for table, col := range tables {
		if col == "" {
			t.Errorf("table %s has no tenant column: it cannot be isolated by tenant", table)
			continue
		}
		if !have[table] {
			t.Errorf("table %s has no fixture in isolationFixtures: add one so TS-08 covers it", table)
			continue
		}
		for _, tenant := range []string{tenantA, tenantB} {
			var n int
			q := fmt.Sprintf("SELECT count(*) FROM %s WHERE %s = $1", pgx.Identifier{"app", table}.Sanitize(), col)
			if err := owner.QueryRow(ctx, q, tenant).Scan(&n); err != nil || n == 0 {
				t.Errorf("fixture for %s left no row for %s: n=%d err=%v", table, tenant, n, err)
			}
		}
	}
}

// checkOtherTenant proves the actor cannot list, fetch, update or delete the victim's rows, and that
// a query with no tenant filter at all (RLS as the second guard) returns at most the actor's own rows.
func checkOtherTenant(ctx context.Context, t *testing.T, who actor, table, col, victim string) {
	t.Helper()
	n, err := who.count(ctx, fmt.Sprintf("SELECT count(*) FROM %s WHERE %s = $1", table, col), victim)
	if err != nil || n != 0 {
		t.Errorf("fetch by victim's tenant: n=%d err=%v", n, err)
	}
	wantAll := int64(1)
	if who.tenant == "" {
		wantAll = 0
	}
	n, err = who.count(ctx, fmt.Sprintf("SELECT count(*) FROM %s", table)) // naive: no tenant filter
	if err != nil || n != wantAll {
		t.Errorf("naive select: want %d rows, got n=%d err=%v", wantAll, n, err)
	}
	n, err = who.exec(ctx, fmt.Sprintf("UPDATE %s SET %s = %s WHERE %s = $1", table, col, col, col), victim)
	wantUntouched(t, "update", n, err)
	n, err = who.exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE %s = $1", table, col), victim)
	wantUntouched(t, "delete", n, err)
}

// checkCompositeFK: a unit of A pointing at B's building is refused by the (tenant_id, id) foreign key.
func checkCompositeFK(ctx context.Context, t *testing.T, a actor) {
	t.Helper()
	_, err := a.exec(ctx, `INSERT INTO app.units (id, tenant_id, building_id, floor_id, unit_type_id, code)
		VALUES ('un_ts08_x', $1, $2, $3, $4, 'X1')`, tenantA, tenantB+"_b", tenantA+"_f", tenantA+"_ut")
	wantSQLState(t, "unit of A in B's building", err, foreignKeyCode)
}

// checkNullTenantEvents covers Ruling 7: an unmatched event (NULL tenant) exists but is invisible and
// untouchable to the app role, which can neither name another tenant nor move a row there.
func checkNullTenantEvents(ctx context.Context, t *testing.T, db string, a, b, none actor) {
	t.Helper()
	// No RETURNING: the select policy hides the row from its own inserter.
	if _, err := none.exec(ctx, `INSERT INTO app.payment_events (id, provider, external_id, amount, result)
		VALUES ($1, 'sim', 'x_ts08_null', 1, 'UNMATCHED')`, unmatchedEvents); err != nil {
		t.Fatalf("app role inserts unmatched event: %v", err)
	}
	for _, who := range []actor{a, b, none} {
		n, err := who.count(ctx, `SELECT count(*) FROM app.payment_events WHERE id = $1`, unmatchedEvents)
		if err != nil || n != 0 {
			t.Errorf("%q sees unmatched event: n=%d err=%v", who.tenant, n, err)
		}
		n, err = who.exec(ctx, `UPDATE app.payment_events SET amount = 2 WHERE id = $1`, unmatchedEvents)
		if err != nil || n != 0 {
			t.Errorf("%q updated unmatched event: n=%d err=%v", who.tenant, n, err)
		}
		n, err = who.exec(ctx, `DELETE FROM app.payment_events WHERE id = $1`, unmatchedEvents)
		if err != nil || n != 0 {
			t.Errorf("%q deleted unmatched event: n=%d err=%v", who.tenant, n, err)
		}
	}
	maint := connAs(t, db, maintRole)
	var n int
	if err := maint.QueryRow(ctx, `SELECT count(*) FROM app.payment_events WHERE id = $1`, unmatchedEvents).Scan(&n); err != nil || n != 1 {
		t.Fatalf("maintenance role must see the unmatched event: n=%d err=%v", n, err)
	}
	_, err := a.exec(ctx, `UPDATE app.payment_events SET tenant_id = $1 WHERE id = $2`, tenantB, tenantA+"_pe")
	wantSQLState(t, "A moves its event to B", err, insufficientPrivilege)
	_, err = a.exec(ctx, `UPDATE app.payment_events SET tenant_id = NULL WHERE id = $1`, tenantA+"_pe")
	wantSQLState(t, "A unmatches its own event", err, insufficientPrivilege)
	n2, err := b.count(ctx, `SELECT count(*) FROM app.payment_events WHERE id = $1`, tenantA+"_pe")
	if err != nil || n2 != 0 {
		t.Errorf("B sees A's matched event: n=%d err=%v", n2, err)
	}
	if _, err := maint.Exec(ctx, `DELETE FROM app.payment_events WHERE id = $1`, unmatchedEvents); err != nil {
		t.Errorf("cleanup unmatched event: %v", err)
	}
}
