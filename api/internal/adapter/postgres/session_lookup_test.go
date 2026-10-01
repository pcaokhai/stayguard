//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/pcaokhai/stayguard/api/internal/app"
)

const (
	lookupTenantA = "tnt_sl_a"
	lookupTenantB = "tnt_sl_b"
)

// withSettings runs fn in a rolled-back app-role transaction carrying only the given settings.
func withSettings(ctx context.Context, t testing.TB, conn *pgx.Conn, settings map[string]string, fn func(tx pgx.Tx)) {
	t.Helper()
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for k, v := range settings {
		if _, err := tx.Exec(ctx, `SELECT set_config($1, $2, true)`, k, v); err != nil {
			t.Fatalf("set %s: %v", k, err)
		}
	}
	fn(tx)
}

func countRows(ctx context.Context, t testing.TB, tx pgx.Tx, table string) int {
	t.Helper()
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.`+table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestSessionLookupIsolation_SG102_TS08(t *testing.T) {
	ctx := context.Background()
	db := migratedDB(t)
	seedAll(t, db, lookupTenantA)
	seedAll(t, db, lookupTenantB)
	appConn := connAs(t, db, appRole)
	hashA := lookupTenantA + "_s"

	cases := []struct {
		name     string
		settings map[string]string
		want     int // rows of A's session visible
	}{
		{"own hash", map[string]string{"app.session_hash": hashA}, 1},
		{"unknown hash", map[string]string{"app.session_hash": "nope"}, 0},
		{"hash with another tenant set", map[string]string{"app.session_hash": hashA, "app.tenant_id": lookupTenantB}, 0},
		{"no setting", nil, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withSettings(ctx, t, appConn, c.settings, func(tx pgx.Tx) {
				var got int
				if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.sessions WHERE token_hash = $1`, hashA).Scan(&got); err != nil || got != c.want {
					t.Errorf("A's session visible = %d err=%v, want %d", got, err, c.want)
				}
			})
		})
	}

	t.Run("returns only A's row", func(t *testing.T) {
		withSettings(ctx, t, appConn, map[string]string{"app.session_hash": hashA}, func(tx pgx.Tx) {
			var tenant string
			if err := tx.QueryRow(ctx, `SELECT tenant_id FROM app.sessions`).Scan(&tenant); err != nil || tenant != lookupTenantA {
				t.Errorf("tenant = %q err=%v, want %q", tenant, err, lookupTenantA)
			}
		})
	})

	// Only meaningful next to the positive control above: the hash is valid and sessions does return a row.
	t.Run("no other table is visible", func(t *testing.T) {
		withSettings(ctx, t, appConn, map[string]string{"app.session_hash": hashA}, func(tx pgx.Tx) {
			for table := range catalogTables(t, db) {
				if table == "sessions" {
					continue
				}
				if got := countRows(ctx, t, tx, table); got != 0 {
					t.Errorf("%s visible rows = %d, want 0", table, got)
				}
			}
		})
	})

	t.Run("update delete insert stay closed", func(t *testing.T) {
		withSettings(ctx, t, appConn, map[string]string{"app.session_hash": hashA}, func(tx pgx.Tx) {
			tag, err := tx.Exec(ctx, `UPDATE app.sessions SET expires_at = now()`)
			if err != nil || tag.RowsAffected() != 0 {
				t.Errorf("update: n=%d err=%v, want 0 rows", tag.RowsAffected(), err)
			}
			tag, err = tx.Exec(ctx, `DELETE FROM app.sessions`)
			if err != nil || tag.RowsAffected() != 0 {
				t.Errorf("delete: n=%d err=%v, want 0 rows", tag.RowsAffected(), err)
			}
		})
		withSettings(ctx, t, appConn, map[string]string{"app.session_hash": hashA}, func(tx pgx.Tx) {
			_, err := tx.Exec(ctx, `INSERT INTO app.sessions (token_hash, tenant_id, user_id, expires_at)
				VALUES ($1, $2, $3, now() + interval '1 hour')`, hashA, lookupTenantA, lookupTenantA+"_u") // token_hash equals the set hash and the pair is valid: only RLS can reject
			wantRLSViolation(t, "insert with hash only", err)
		})
	})

	t.Run("policies", func(t *testing.T) {
		rows, err := connAs(t, db, "owner").Query(ctx, `SELECT polname, polcmd::text FROM pg_policy WHERE polrelid = 'app.sessions'::regclass ORDER BY polname`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		got := map[string]string{}
		for rows.Next() {
			var name, cmd string
			if err := rows.Scan(&name, &cmd); err != nil {
				t.Fatal(err)
			}
			got[name] = cmd
		}
		if len(got) != 2 || got["sessions_tenant"] != "*" || got["sessions_lookup"] != "r" {
			t.Errorf("policies = %v, want sessions_tenant(*) and sessions_lookup(r)", got)
		}
	})
}

func TestSessionResolver_SG102_AC4(t *testing.T) {
	ctx := context.Background()
	db := migratedDB(t)
	tenant := "tnt_sr_a"
	seedAll(t, db, tenant)
	pool := newAppPool(t, db, 1)
	r := NewSessionResolver(pool)

	ref, err := r.Resolve(ctx, tenant+"_s")
	if err != nil || ref.TenantID != tenant || ref.UserID != tenant+"_u" || ref.ExpiresAt.IsZero() {
		t.Fatalf("found: ref=%+v err=%v", ref, err)
	}
	if _, err := r.Resolve(ctx, "unknown"); !errors.Is(err, app.ErrSessionNotFound) {
		t.Errorf("unknown hash: err=%v, want ErrSessionNotFound", err)
	}

	// max conns 1: this reuses the resolver's connection, so a leaked setting would show here.
	var setting string
	if err := pool.QueryRow(ctx, `SELECT coalesce(current_setting('app.session_hash', true), '')`).Scan(&setting); err != nil || setting != "" {
		t.Errorf("session_hash leaked to next use: %q err=%v", setting, err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM app.sessions`).Scan(&n); err != nil || n != 0 {
		t.Errorf("tenantless pooled read: n=%d err=%v, want 0", n, err)
	}
}
