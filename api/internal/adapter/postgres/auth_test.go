//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/app"
)

type movingClock struct{ now time.Time }

func (c *movingClock) Now() time.Time { return c.now }

// An adapter test may not import other adapters, so the PIN hash, ids and tokens are local fakes.
type testHasher struct{}

func (testHasher) Hash(pin string) (string, error) { return "hashed:" + pin, nil }
func (testHasher) Verify(hash, pin string) bool    { return hash == "hashed:"+pin }

type testGen struct{ n atomic.Int32 }

func (g *testGen) New(prefix string) string { return fmt.Sprintf("%s_%d", prefix, g.n.Add(1)) }

// tokenSeq is package-wide: the database is shared, and a session token hash is unique across tenants.
var tokenSeq atomic.Int32

type testTokens struct{}

func (testTokens) New() (string, error) { return fmt.Sprintf("auth-tok-%d", tokenSeq.Add(1)), nil }

type allowAllLimiter struct{}

func (allowAllLimiter) Allow(string) bool { return true }

type noSeeder struct{}

func (noSeeder) Seed(context.Context, app.Tx, time.Time) error { return nil }

const (
	authPinA = "482915"
	authPinB = "736201"
)

var authSeq atomic.Int32

type authEnv struct {
	// tenantA/tenantB and codeA/codeB are unique per test: the database is shared by the package.
	tenantA, tenantB, codeA, codeB string
	db                             string
	uow                            *UnitOfWork
	auth                           *app.Auth
	sess                           *app.Sessions
	clock                          *movingClock
}

// newAuthEnv builds the real use cases on a migrated database with two guesthouses, each with user "ann"
// and a different PIN.
func newAuthEnv(t *testing.T) authEnv {
	t.Helper()
	ctx := context.Background()
	db := migratedDB(t)
	pool := newAppPool(t, db, 4)
	uow := NewUnitOfWork(pool)
	clk := &movingClock{now: time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)}
	sess := app.NewSessions(app.SessionsConfig{SessionTTL: time.Hour}, uow, NewSessionResolver(pool), NewIdentityRepo(), clk, &testGen{}, testTokens{}, noSeeder{})
	auth, err := app.NewAuth(sess, NewTenantResolver(pool), NewAuthRepo(), testHasher{}, NewAuditWriter(), AlertWriter{}, allowAllLimiter{}, allowAllLimiter{})
	if err != nil {
		t.Fatal(err)
	}
	n := authSeq.Add(1)
	e := authEnv{db: db, uow: uow, auth: auth, sess: sess, clock: clk,
		tenantA: fmt.Sprintf("tnt_auth%d_a", n), tenantB: fmt.Sprintf("tnt_auth%d_b", n),
		codeA: fmt.Sprintf("casa%d", n), codeB: fmt.Sprintf("villa%d", n)}
	owner := connAs(t, db, "owner")
	hasher := testHasher{}
	for tenant, c := range map[string]struct{ code, pin string }{e.tenantA: {e.codeA, authPinA}, e.tenantB: {e.codeB, authPinB}} {
		for _, q := range []string{
			`INSERT INTO app.tenants (id, name, guesthouse_code) VALUES ($1, 'T', $2)`,
			`INSERT INTO app.users (id, tenant_id, name, role, app_access, username) VALUES ($1 || '_ann', $1, 'Ann', 'RECEPTIONIST', 'RECEPTIONIST', 'ann')`,
		} {
			args := []any{tenant, c.code}
			if strings.Contains(q, "users") {
				args = []any{tenant}
			}
			if _, err = owner.Exec(ctx, q, args...); err != nil {
				t.Fatal(err)
			}
		}
		h, herr := hasher.Hash(c.pin)
		if herr != nil {
			t.Fatal(herr)
		}
		err = uow.Do(ctx, tenant, func(ctx context.Context, tx app.Tx) error {
			return NewAuthRepo().SetPin(ctx, tx, tenant+"_ann", h, false, nil, clk.now)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return e
}

func TestSignInPostgres_FlowAndTenantIsolation_SG701(t *testing.T) {
	ctx := context.Background()
	e := newAuthEnv(t)
	res, err := e.auth.SignIn(ctx, "1.1.1.1", e.codeA, "ann", authPinA)
	if err != nil || res.TenantID != e.tenantA || res.User.ID != e.tenantA+"_ann" {
		t.Fatalf("sign in: %+v %v", res, err)
	}
	c, err := e.sess.Authenticate(ctx, res.Token)
	if err != nil || c.TenantID != e.tenantA || c.PinChangeRequired {
		t.Fatalf("authenticate: %+v %v", c, err)
	}
	// Guesthouse B's PIN does not open guesthouse A, and the other way round; unknown code and user look the same.
	for _, in := range [][3]string{{e.codeA, "ann", authPinB}, {e.codeB, "ann", authPinA}, {"nowhere", "ann", authPinA}, {e.codeA, "zed", authPinA}} {
		if _, err = e.auth.SignIn(ctx, "1.1.1.1", in[0], in[1], in[2]); !errors.Is(err, app.ErrPinInvalid) {
			t.Errorf("%v: %v", in, err)
		}
	}
	// signOut revokes the session.
	if err = e.auth.SignOut(ctx, c); err != nil {
		t.Fatal(err)
	}
	if _, err = e.sess.Authenticate(ctx, res.Token); !errors.Is(err, app.ErrUnauthenticated) {
		t.Fatalf("after sign-out: %v", err)
	}
}

func TestSignInPostgres_LockoutPersists_SG701_AC2(t *testing.T) {
	ctx := context.Background()
	e := newAuthEnv(t)
	var locked *app.AccountLockedError
	var err error
	for i := 0; i < 5; i++ {
		_, err = e.auth.SignIn(ctx, "1.1.1.1", e.codeA, "ann", "159357")
	}
	if !errors.As(err, &locked) {
		t.Fatalf("fifth wrong PIN: %v", err)
	}
	// The failure counts were committed although every call returned an error.
	var n int
	owner := connAs(t, e.db, "owner")
	if err = owner.QueryRow(ctx, `SELECT count(*) FROM app.pin_credentials WHERE tenant_id = $1 AND locked_until IS NOT NULL`, e.tenantA).Scan(&n); err != nil || n != 1 {
		t.Fatalf("locked rows %d %v", n, err)
	}
	var action, after string
	if err = owner.QueryRow(ctx, `SELECT action, after::text FROM app.audit_logs WHERE tenant_id = $1`, e.tenantA).Scan(&action, &after); err != nil || action != "ACCOUNT_LOCKED" {
		t.Fatalf("audit %q %v", action, err)
	}
	if strings.Contains(after, authPinA) || strings.Contains(after, "159357") {
		t.Fatal("PIN in audit row")
	}
	var alerts int
	if err = owner.QueryRow(ctx, `SELECT count(*) FROM app.alerts WHERE tenant_id = $1 AND kind = 'ACCOUNT_LOCKED' AND actor_id = $2`, e.tenantA, e.tenantA+"_ann").Scan(&alerts); err != nil || alerts != 1 {
		t.Fatalf("ACCOUNT_LOCKED alert rows = %d %v", alerts, err)
	}
	if _, err = e.auth.SignIn(ctx, "1.1.1.1", e.codeA, "ann", authPinA); !errors.As(err, &locked) {
		t.Fatalf("right PIN while locked: %v", err)
	}
	// The other guesthouse's ann is untouched.
	if _, err = e.auth.SignIn(ctx, "1.1.1.1", e.codeB, "ann", authPinB); err != nil {
		t.Fatalf("other tenant: %v", err)
	}
	e.clock.now = locked.Until
	if _, err = e.auth.SignIn(ctx, "1.1.1.1", e.codeA, "ann", authPinA); err != nil {
		t.Fatalf("after the lock: %v", err)
	}
}

func TestSignInPostgres_OneTimePinAndBlockedUser_SG701_AC3(t *testing.T) {
	ctx := context.Background()
	e := newAuthEnv(t)
	owner := connAs(t, e.db, "owner")
	if _, err := owner.Exec(ctx, `UPDATE app.pin_credentials SET must_change = true, one_time_expires_at = $2 WHERE tenant_id = $1`, e.tenantA, e.clock.now.Add(app.OneTimePinTTL)); err != nil {
		t.Fatal(err)
	}
	res, err := e.auth.SignIn(ctx, "1.1.1.1", e.codeA, "ann", authPinA)
	if err != nil || !res.MustChangePin {
		t.Fatalf("%+v %v", res, err)
	}
	c, err := e.sess.Authenticate(ctx, res.Token)
	if err != nil || !c.PinChangeRequired {
		t.Fatalf("session must be limited to changing the PIN: %+v %v", c, err)
	}
	if err = e.auth.ChangePin(ctx, c, authPinA, "123456"); !errors.Is(err, app.ErrPinTooSimple) {
		t.Fatalf("run: %v", err)
	}
	if err = e.auth.ChangePin(ctx, c, authPinA, "260814"); err != nil {
		t.Fatal(err)
	}
	if c, err = e.sess.Authenticate(ctx, res.Token); err != nil || c.PinChangeRequired {
		t.Fatalf("after change: %+v %v", c, err)
	}
	// Removing the user ends the session at once and blocks sign-in.
	if _, err = owner.Exec(ctx, `UPDATE app.users SET status = 'REMOVED' WHERE tenant_id = $1`, e.tenantA); err != nil {
		t.Fatal(err)
	}
	if _, err = e.sess.Authenticate(ctx, res.Token); !errors.Is(err, app.ErrUnauthenticated) {
		t.Fatalf("removed user session: %v", err)
	}
	if _, err = e.auth.SignIn(ctx, "1.1.1.1", e.codeA, "ann", "260814"); !errors.Is(err, app.ErrPinInvalid) {
		t.Fatalf("removed user sign-in: %v", err)
	}
}
