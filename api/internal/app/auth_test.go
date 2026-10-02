package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

type authKey struct{ tenant, id string }

type fakeAuthRepo struct {
	users   map[authKey]SignInUser // by tenant+user id
	byName  map[authKey]string     // tenant+username -> user id
	deleted []string
	kept    string
	setPins []string
}

func (r *fakeAuthRepo) SignInUser(_ context.Context, tx Tx, username string) (SignInUser, bool, error) {
	id, ok := r.byName[authKey{tx.TenantID(), username}]
	if !ok {
		return SignInUser{}, false, nil
	}
	return r.users[authKey{tx.TenantID(), id}], true, nil
}

func (r *fakeAuthRepo) PinState(_ context.Context, tx Tx, id string) (PinState, bool, error) {
	u, ok := r.users[authKey{tx.TenantID(), id}]
	return u.Pin, ok, nil
}

func (r *fakeAuthRepo) SetPinFailures(_ context.Context, tx Tx, id string, n int, first, until *time.Time) error {
	k := authKey{tx.TenantID(), id}
	u := r.users[k]
	u.Pin.FailedCount, u.Pin.FirstFailedAt, u.Pin.LockedUntil = n, first, until
	r.users[k] = u
	return nil
}

func (r *fakeAuthRepo) SetPin(_ context.Context, tx Tx, id, hash string, must bool, once *time.Time, _ time.Time) error {
	k := authKey{tx.TenantID(), id}
	u := r.users[k]
	u.Pin = PinState{Hash: hash, MustChange: must, OneTimeExpiresAt: once}
	r.users[k] = u
	r.setPins = append(r.setPins, hash)
	return nil
}

func (r *fakeAuthRepo) DeleteSession(_ context.Context, _ Tx, h string) error {
	r.deleted = append(r.deleted, h)
	return nil
}

func (r *fakeAuthRepo) DeleteOtherSessions(_ context.Context, _ Tx, _, keep string) error {
	r.kept = keep
	return nil
}

type fakeTenants map[string]string

func (f fakeTenants) TenantByCode(_ context.Context, code string) (string, bool, error) {
	id, ok := f[code]
	return id, ok, nil
}

// fakeHasher is reversible on purpose: tests assert the stored value is not the PIN itself.
type fakeHasher struct{}

func (fakeHasher) Hash(pin string) (string, error) { return "hash(" + pin + ")", nil }
func (fakeHasher) Verify(h, pin string) bool       { return h == "hash("+pin+")" }

type allowAll struct{}

func (allowAll) Allow(string) bool { return true }

type denyKey struct{ key string }

func (d denyKey) Allow(k string) bool { return k != d.key }

type authRig struct {
	alerts *fakeAlerts
	a      *Auth
	repo   *fakeAuthRepo
	audit  *fakeAudit
	ids    *fakeRepo
	clock  *clockBox
}

type clockBox struct{ now time.Time }

func (c *clockBox) Now() time.Time { return c.now }

const (
	pinOK    = "482915"
	pinWrong = "159357"
)

func newAuthRig(t *testing.T, byIP, byCode RateLimiter) authRig {
	t.Helper()
	clk := &clockBox{now: t0}
	ir := newFakeRepo()
	s := NewSessions(SessionsConfig{SessionTTL: 12 * time.Hour}, &fakeUoW{}, &fakeResolver{}, ir, clk, &seqIDs{}, fixedToken{rawToken}, noSeed{})
	repo := &fakeAuthRepo{users: map[authKey]SignInUser{}, byName: map[authKey]string{}}
	// Tenant A has "ann"; tenant B has "bob". Same PIN for both.
	for tenant, name := range map[string]string{"tn_a": "ann", "tn_b": "bob"} {
		id := "us_" + name
		repo.users[authKey{tenant, id}] = SignInUser{
			User: User{ID: id, Name: name, Role: access.RoleReceptionist, Locale: "vi"}, Access: "RECEPTIONIST", Status: "ACTIVE",
			Pin: PinState{Hash: "hash(" + pinOK + ")"},
		}
		repo.byName[authKey{tenant, name}] = id
	}
	audit := &fakeAudit{}
	alerts := &fakeAlerts{}
	a, err := NewAuth(s, fakeTenants{"casa": "tn_a", "villa": "tn_b"}, repo, fakeHasher{}, audit, alerts, byIP, byCode)
	if err != nil {
		t.Fatal(err)
	}
	return authRig{alerts: alerts, a: a, repo: repo, audit: audit, ids: ir, clock: clk}
}

func (r authRig) signIn(code, user, pin string) (SignInResult, error) {
	return r.a.SignIn(context.Background(), "1.2.3.4", code, user, pin)
}

func TestSignIn_Success_SG701_AC1(t *testing.T) {
	r := newAuthRig(t, allowAll{}, allowAll{})
	res, err := r.signIn(" Casa ", "ANN", pinOK) // code and name are case-insensitive
	if err != nil || res.Token != rawToken || res.TenantID != "tn_a" || res.User.ID != "us_ann" || res.MustChangePin {
		t.Fatalf("got %+v %v", res, err)
	}
	if len(r.ids.sessions) != 1 || r.ids.sessions[0].hash != HashToken(rawToken) {
		t.Fatalf("session must be stored by hash: %+v", r.ids.sessions)
	}
}

func TestSignIn_AnyWrongPartIsTheSameError_SG701_AC1(t *testing.T) {
	r := newAuthRig(t, allowAll{}, allowAll{})
	for name, c := range map[string][3]string{
		"unknown code": {"nowhere", "ann", pinOK}, "unknown user": {"casa", "zed", pinOK}, "wrong pin": {"casa", "ann", pinWrong},
	} {
		if _, err := r.signIn(c[0], c[1], c[2]); !errors.Is(err, ErrPinInvalid) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

func TestSignIn_OtherTenantUserCannotSignIn_SG701(t *testing.T) {
	r := newAuthRig(t, allowAll{}, allowAll{})
	// bob exists only in tenant B: guesthouse A with bob's name and the right PIN must fail, and vice versa.
	if _, err := r.signIn("casa", "bob", pinOK); !errors.Is(err, ErrPinInvalid) {
		t.Fatalf("A with bob: %v", err)
	}
	if _, err := r.signIn("villa", "ann", pinOK); !errors.Is(err, ErrPinInvalid) {
		t.Fatalf("B with ann: %v", err)
	}
	if res, err := r.signIn("villa", "bob", pinOK); err != nil || res.TenantID != "tn_b" {
		t.Fatalf("B with bob: %+v %v", res, err)
	}
}

func TestSignIn_FifthWrongPinLocks_SG701_AC2(t *testing.T) {
	r := newAuthRig(t, allowAll{}, allowAll{})
	for i := 1; i <= 4; i++ {
		if _, err := r.signIn("casa", "ann", pinWrong); !errors.Is(err, ErrPinInvalid) {
			t.Fatalf("attempt %d: %v", i, err)
		}
		r.clock.now = r.clock.now.Add(time.Minute)
	}
	_, err := r.signIn("casa", "ann", pinWrong)
	var locked *AccountLockedError
	if !errors.As(err, &locked) || !locked.Until.Equal(r.clock.now.Add(15*time.Minute)) {
		t.Fatalf("fifth must lock for 15 minutes: %v", err)
	}
	if len(r.audit.entries) != 1 || r.audit.entries[0].Action != "ACCOUNT_LOCKED" || r.audit.entries[0].EntityID != "us_ann" {
		t.Fatalf("audit: %+v", r.audit.entries)
	}
	if len(r.alerts.raised) != 1 || r.alerts.raised[0].Kind != AlertAccountLocked || r.alerts.raised[0].By != "us_ann" {
		t.Fatalf("alert: %+v", r.alerts.raised)
	}
	// Locked: even the right PIN is refused, with the same end time.
	if _, err = r.signIn("casa", "ann", pinOK); !errors.As(err, &locked) {
		t.Fatalf("right PIN while locked: %v", err)
	}
	// After the lock ends the right PIN works and the counter is clear.
	r.clock.now = locked.Until
	if _, err = r.signIn("casa", "ann", pinOK); err != nil {
		t.Fatalf("after lock: %v", err)
	}
	if u := r.repo.users[authKey{"tn_a", "us_ann"}]; u.Pin.FailedCount != 0 || u.Pin.LockedUntil != nil {
		t.Fatalf("state not cleared: %+v", u.Pin)
	}
}

func TestSignIn_WrongPinsOutsideWindowDoNotLock_SG701_AC2(t *testing.T) {
	r := newAuthRig(t, allowAll{}, allowAll{})
	for i := 0; i < 8; i++ { // 8 wrong PINs, one every 4 minutes: never 5 inside 15 minutes
		if _, err := r.signIn("casa", "ann", pinWrong); !errors.Is(err, ErrPinInvalid) {
			t.Fatalf("attempt %d: %v", i, err)
		}
		r.clock.now = r.clock.now.Add(4 * time.Minute)
	}
}

func TestSignIn_OneTimePin_SG701_AC3(t *testing.T) {
	r := newAuthRig(t, allowAll{}, allowAll{})
	k := authKey{"tn_a", "us_ann"}
	u := r.repo.users[k]
	exp := t0.Add(OneTimePinTTL)
	u.Pin.MustChange, u.Pin.OneTimeExpiresAt = true, &exp
	r.repo.users[k] = u
	if res, err := r.signIn("casa", "ann", pinOK); err != nil || !res.MustChangePin {
		t.Fatalf("must change: %+v %v", res, err)
	}
	r.clock.now = exp // expired exactly at its instant
	if _, err := r.signIn("casa", "ann", pinOK); !errors.Is(err, ErrPinInvalid) {
		t.Fatalf("expired one-time PIN: %v", err)
	}
}

func TestSignIn_RemovedOrNoAccessUserIsInvalid_SG701(t *testing.T) {
	for _, mut := range []func(*SignInUser){func(u *SignInUser) { u.Status = "REMOVED" }, func(u *SignInUser) { u.Access = "NONE" }, func(u *SignInUser) { u.Status = "LOCKED" }} {
		r := newAuthRig(t, allowAll{}, allowAll{})
		k := authKey{"tn_a", "us_ann"}
		u := r.repo.users[k]
		mut(&u)
		r.repo.users[k] = u
		if _, err := r.signIn("casa", "ann", pinOK); !errors.Is(err, ErrPinInvalid) {
			t.Fatalf("got %v", err)
		}
	}
}

func TestSignIn_BadInputIs422_SG701(t *testing.T) {
	r := newAuthRig(t, allowAll{}, allowAll{})
	var ve *ValidationError
	for _, pin := range []string{"", "12345", "1234567", "12345a"} {
		if _, err := r.signIn("casa", "ann", pin); !errors.As(err, &ve) {
			t.Errorf("%q: %v", pin, err)
		}
	}
}

func TestSignIn_RateLimited_SG701_AC5(t *testing.T) {
	for name, r := range map[string]authRig{
		"ip":   newAuthRig(t, denyKey{"ip:1.2.3.4"}, allowAll{}),
		"code": newAuthRig(t, allowAll{}, denyKey{"code:casa"}),
	} {
		if _, err := r.signIn("casa", "ann", pinOK); !errors.Is(err, ErrTooManyRequests) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestSignOut_RevokesSession_SG701_AC5(t *testing.T) {
	r := newAuthRig(t, allowAll{}, allowAll{})
	if err := r.a.SignOut(context.Background(), Caller{TenantID: "tn_a", UserID: "us_ann", SessionHash: "h1"}); err != nil || len(r.repo.deleted) != 1 || r.repo.deleted[0] != "h1" {
		t.Fatalf("%v %v", err, r.repo.deleted)
	}
}

func TestChangePin_SG701_AC3(t *testing.T) {
	c := Caller{TenantID: "tn_a", UserID: "us_ann", SessionHash: "h1"}
	r := newAuthRig(t, allowAll{}, allowAll{})
	ctx := context.Background()
	for _, bad := range []string{"123456", "654321", "111111", pinOK} { // runs, repeats, unchanged
		if err := r.a.ChangePin(ctx, c, pinOK, bad); !errors.Is(err, ErrPinTooSimple) {
			t.Errorf("new PIN %q: %v", bad, err)
		}
	}
	if err := r.a.ChangePin(ctx, c, pinWrong, "736201"); !errors.Is(err, ErrPinInvalid) {
		t.Fatalf("wrong current PIN: %v", err)
	}
	if len(r.repo.setPins) != 0 {
		t.Fatal("nothing may be stored yet")
	}
	if err := r.a.ChangePin(ctx, c, pinOK, "736201"); err != nil {
		t.Fatal(err)
	}
	if u := r.repo.users[authKey{"tn_a", "us_ann"}]; u.Pin.MustChange || u.Pin.Hash != "hash(736201)" || r.repo.kept != "h1" {
		t.Fatalf("pin %+v kept %q", u.Pin, r.repo.kept)
	}
	if _, err := r.signIn("casa", "ann", "736201"); err != nil {
		t.Fatalf("new PIN signs in: %v", err)
	}
}

func TestChangePin_WrongCurrentPinCountsTowardsLock_SG701_AC2(t *testing.T) {
	r := newAuthRig(t, allowAll{}, allowAll{})
	c := Caller{TenantID: "tn_a", UserID: "us_ann", SessionHash: "h1"}
	var locked *AccountLockedError
	var err error
	for i := 0; i < 5; i++ {
		err = r.a.ChangePin(context.Background(), c, pinWrong, "736201")
	}
	if !errors.As(err, &locked) {
		t.Fatalf("fifth wrong current PIN must lock: %v", err)
	}
}

func TestPinNeverStoredOrAuditedInClear_SG701_AC4(t *testing.T) {
	r := newAuthRig(t, allowAll{}, allowAll{})
	c := Caller{TenantID: "tn_a", UserID: "us_ann", SessionHash: "h1"}
	_ = r.a.ChangePin(context.Background(), c, pinOK, "736201")
	for i := 0; i < 5; i++ {
		_, _ = r.signIn("casa", "ann", pinWrong)
	}
	for _, h := range r.repo.setPins {
		if h == "736201" {
			t.Fatal("PIN stored in clear")
		}
	}
	for _, e := range r.audit.entries {
		if strings.Contains(string(e.After)+string(e.Before), pinWrong) || strings.Contains(string(e.After), "736201") {
			t.Fatalf("PIN in audit entry: %s", e.After)
		}
	}
}
