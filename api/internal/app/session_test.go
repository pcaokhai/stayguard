package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

const (
	rawToken   = "raw-token-value"
	sessionTTL = 12 * time.Hour
	trialTTL   = 24 * time.Hour
)

type rig struct {
	s   *Sessions
	uow *fakeUoW
	res *fakeResolver
	rep *fakeRepo
	ids *seqIDs
}

type noSeed struct{}

func (noSeed) Seed(context.Context, Tx, time.Time) error { return nil }

func newRig(demo bool) rig {
	r := rig{uow: &fakeUoW{}, res: &fakeResolver{}, rep: newFakeRepo(), ids: &seqIDs{}}
	r.s = NewSessions(SessionsConfig{DemoEnabled: demo, SessionTTL: sessionTTL, TrialTTL: trialTTL},
		r.uow, r.res, r.rep, fixedClock{t0}, r.ids, fixedToken{rawToken}, noSeed{})
	return r
}

func TestSessionToken_SG102_AC2(t *testing.T) {
	// SHA-256("abc") as lowercase hex.
	if got := HashToken("abc"); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("HashToken vector: %s", got)
	}
	r := newRig(true)
	ds, err := r.s.CreateDemo(context.Background(), access.RoleOwner, "vi", "")
	if err != nil || ds.Token != rawToken {
		t.Fatalf("token must be returned once: %+v %v", ds, err)
	}
	if len(r.rep.sessions) != 1 || r.rep.sessions[0].hash != HashToken(rawToken) || strings.Contains(r.rep.sessions[0].hash, rawToken) {
		t.Fatalf("repo must see only the hash: %+v", r.rep.sessions)
	}
	r.res.err = ErrSessionNotFound
	_, _ = r.s.Authenticate(context.Background(), rawToken)
	if len(r.res.hashes) != 1 || r.res.hashes[0] != HashToken(rawToken) {
		t.Fatalf("resolver must see only the hash: %v", r.res.hashes)
	}
}

func TestSessionExpiry_SG102_AC2(t *testing.T) {
	cases := []struct {
		name string
		exp  time.Time
		err  error
		want error
	}{
		{"valid", t0.Add(time.Second), nil, nil},
		{"at expiry", t0, nil, ErrSessionExpired},
		{"after expiry", t0.Add(-time.Second), nil, ErrSessionExpired},
		{"unknown", t0, ErrSessionNotFound, ErrUnauthenticated},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(true)
			r.res.ref, r.res.err = SessionRef{TenantID: "tn1", UserID: "us1", ExpiresAt: c.exp}, c.err
			r.rep.usersByID["us1"] = User{ID: "us1", Role: access.RoleOwner, Locale: "en"}
			got, err := r.s.Authenticate(context.Background(), rawToken)
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			if c.want == nil && (got.TenantID != "tn1" || got.UserID != "us1" || got.Role != access.RoleOwner || got.Locale != "en") {
				t.Fatalf("caller = %+v", got)
			}
		})
	}
	r := newRig(true)
	if _, err := r.s.Authenticate(context.Background(), ""); !errors.Is(err, ErrUnauthenticated) || len(r.res.hashes) != 0 {
		t.Fatalf("empty token: %v", err)
	}
	r.res.ref = SessionRef{TenantID: "tn1", UserID: "ghost", ExpiresAt: t0.Add(time.Hour)}
	if _, err := r.s.Authenticate(context.Background(), rawToken); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("missing user: %v", err)
	}
}

func TestCreateDemo_SG102_AC1(t *testing.T) {
	for _, role := range []access.Role{access.RoleOwner, access.RoleReceptionist, access.RoleHousekeeping} {
		for _, loc := range []string{"vi", "en"} {
			r := newRig(true)
			ds, err := r.s.CreateDemo(context.Background(), role, loc, "")
			if err != nil {
				t.Fatalf("%s/%s: %v", role, loc, err)
			}
			wantTenant := "tn_gen1"
			if ds.TenantID != wantTenant || ds.User.Role != role || ds.User.Locale != loc || ds.User.ID != "us_gen2" {
				t.Fatalf("%s/%s: %+v", role, loc, ds)
			}
			if !ds.ExpiresAt.Equal(t0.Add(sessionTTL)) || r.rep.sessions[0].userID != ds.User.ID {
				t.Fatalf("session = %+v / %+v", ds, r.rep.sessions)
			}
			if len(r.uow.tenants) != 1 || r.uow.tenants[0] != wantTenant {
				t.Fatalf("one unit of work on the new tenant, got %v", r.uow.tenants)
			}
		}
	}
}

func TestCreateDemoDisabled_SG102_AC1(t *testing.T) {
	r := newRig(false)
	_, err := r.s.CreateDemo(context.Background(), access.RoleOwner, "vi", "tn1")
	if !errors.Is(err, ErrDemoDisabled) {
		t.Fatalf("err = %v", err)
	}
	if r.rep.calls != 0 || len(r.uow.tenants) != 0 || len(r.res.hashes) != 0 || r.ids.n != 0 {
		t.Fatalf("disabled demo must touch nothing: %+v", r.rep)
	}
}

func TestCreateDemoValidation_SG102_AC1(t *testing.T) {
	r := newRig(true)
	for _, c := range []struct {
		role access.Role
		loc  string
	}{{"ADMIN", "vi"}, {access.RoleOwner, "fr"}, {access.RoleOwner, ""}} {
		var ve *ValidationError
		if _, err := r.s.CreateDemo(context.Background(), c.role, c.loc, ""); !errors.As(err, &ve) {
			t.Errorf("%v: err = %v", c, err)
		}
	}
	if r.rep.calls != 0 {
		t.Fatal("validation must precede any repository call")
	}
}

func TestCreateDemoTrialTenant_SG102_AC1(t *testing.T) {
	r := newRig(true)
	r.rep.trials["tn_ok"] = true
	ds, err := r.s.CreateDemo(context.Background(), access.RoleReceptionist, "en", "tn_ok")
	if err != nil || ds.TenantID != "tn_ok" || len(r.rep.createdTenant) != 0 || r.uow.tenants[0] != "tn_ok" {
		t.Fatalf("existing trial: %+v %v", ds, err)
	}
	// Unknown, expired and non-trial tenants are all "not usable" to the repo and must look identical.
	for _, id := range []string{"tn_unknown", "tn_expired", "tn_real"} {
		r2 := newRig(true)
		if _, err := r2.s.CreateDemo(context.Background(), access.RoleOwner, "vi", id); !errors.Is(err, ErrTrialNotFound) {
			t.Errorf("%s: err = %v", id, err)
		}
		if len(r2.rep.sessions) != 0 || len(r2.rep.createdUsers) != 0 {
			t.Errorf("%s: nothing may be created", id)
		}
	}
}

func TestCreateDemoServerIDs_SG102_AC1(t *testing.T) {
	r := newRig(true)
	ds, err := r.s.CreateDemo(context.Background(), access.RoleOwner, "vi", "")
	if err != nil || r.rep.createdTenant[0] != "tn_gen1" || r.rep.createdUsers[0].ID != "us_gen2" || ds.TenantID != "tn_gen1" {
		t.Fatalf("ids must come from the generator: %+v %+v %v", r.rep.createdTenant, r.rep.createdUsers, err)
	}
}

func TestCreateDemoConflictRetry_SG102_AC1(t *testing.T) {
	r := newRig(true)
	r.rep.conflictOnce = true
	ds, err := r.s.CreateDemo(context.Background(), access.RoleOwner, "vi", "")
	if err != nil || ds.User.ID != "us_winner" || r.rep.sessions[0].userID != "us_winner" {
		t.Fatalf("conflict must fall back to the winner: %+v %v", ds, err)
	}
}

func TestCreateDemoReusesUser_SG102_AC1(t *testing.T) {
	r := newRig(true)
	r.rep.trials["tn_ok"] = true
	r.rep.users[access.RoleOwner] = User{ID: "us_existing", Role: access.RoleOwner, Locale: "vi"}
	ds, err := r.s.CreateDemo(context.Background(), access.RoleOwner, "vi", "tn_ok")
	if err != nil || ds.User.ID != "us_existing" || len(r.rep.createdUsers) != 0 {
		t.Fatalf("existing role user must be reused: %+v %v", ds, err)
	}
}

func TestMe_SG102_AC3(t *testing.T) {
	r := newRig(true)
	r.rep.usersByID["us1"] = User{ID: "us1", Name: "Ann", Role: access.RoleReceptionist, Locale: "en"}
	r.rep.info = TenantInfo{ID: "tn1", Name: "Demo", Timezone: "Asia/Ho_Chi_Minh", Currency: "VND"}
	r.rep.buildings = []string{"b1", "b2"}
	c := Caller{TenantID: "tn1", UserID: "us1", Role: access.RoleReceptionist, Locale: "en"}
	me, err := r.s.Me(context.Background(), c)
	if err != nil || me.User.ID != "us1" || me.Tenant.Currency != "VND" {
		t.Fatalf("me = %+v %v", me, err)
	}
	if me.BuildingAccess["b1"] != access.NONE || me.BuildingAccess["b2"] != access.NONE {
		t.Fatalf("non-owner derives NONE: %+v", me.BuildingAccess)
	}
	c.Role = access.RoleOwner
	r.rep.usersByID["us1"] = User{ID: "us1", Role: access.RoleOwner}
	if me, _ = r.s.Me(context.Background(), c); me.BuildingAccess["b1"] != access.EDIT {
		t.Fatalf("owner derives EDIT: %+v", me.BuildingAccess)
	}
}

func TestSetLocale_SG102_AC3(t *testing.T) {
	r := newRig(true)
	c := Caller{TenantID: "tn1", UserID: "us1", Role: access.RoleOwner}
	if err := r.s.SetLocale(context.Background(), c, "en"); err != nil || r.rep.locale != "en" {
		t.Fatalf("valid: %v %q", err, r.rep.locale)
	}
	var ve *ValidationError
	if err := r.s.SetLocale(context.Background(), c, "fr"); !errors.As(err, &ve) || r.rep.locale != "en" {
		t.Fatalf("invalid must be a validation error and not stored: %v", err)
	}
}

func TestCallerContext_SG102_AC4(t *testing.T) {
	if _, ok := CallerFrom(context.Background()); ok {
		t.Fatal("empty context has no caller")
	}
	want := Caller{TenantID: "tn1", UserID: "us1", Role: access.RoleOwner, Locale: "vi"}
	if got, ok := CallerFrom(WithCaller(context.Background(), want)); !ok || got != want {
		t.Fatalf("got %+v %v", got, ok)
	}
}

func TestCreateDemoClockAndExpiry_SG102_AC1(t *testing.T) {
	r := newRig(true)
	if _, err := r.s.CreateDemo(context.Background(), access.RoleOwner, "vi", ""); err != nil || !r.rep.tenantExpires.Equal(t0.Add(trialTTL)) {
		t.Fatalf("trial expiry = %v err=%v", r.rep.tenantExpires, err)
	}
	r.rep.trials["tn_ok"] = true
	if _, err := r.s.CreateDemo(context.Background(), access.RoleOwner, "vi", "tn_ok"); err != nil || !r.rep.trialNow.Equal(t0) {
		t.Fatalf("TrialTenant now = %v err=%v", r.rep.trialNow, err)
	}
}

func TestCreateDemoExistingUserLocale_SG102_AC1(t *testing.T) {
	r := newRig(true)
	r.rep.trials["tn_ok"] = true
	r.rep.users[access.RoleOwner] = User{ID: "us_existing", Role: access.RoleOwner, Locale: "vi"}
	ds, err := r.s.CreateDemo(context.Background(), access.RoleOwner, "en", "tn_ok")
	if err != nil || ds.User.Locale != "en" || r.rep.locale != "en" {
		t.Fatalf("locale must follow the request: %+v stored=%q %v", ds.User, r.rep.locale, err)
	}
}

func TestCreateDemoTenantIDShape_SG102_AC1(t *testing.T) {
	r := newRig(true)
	for _, id := range []string{"tn-1", "tn 1", "tn'1", "t\u00e9", strings.Repeat("a", 65)} {
		if _, err := r.s.CreateDemo(context.Background(), access.RoleOwner, "vi", id); !errors.Is(err, ErrTrialNotFound) {
			t.Errorf("%q: err = %v", id, err)
		}
	}
	if len(r.uow.tenants) != 0 || r.rep.calls != 0 {
		t.Fatal("malformed ids must not reach the unit of work")
	}
	r.rep.trials[strings.Repeat("a", 64)] = true
	if _, err := r.s.CreateDemo(context.Background(), access.RoleOwner, "vi", strings.Repeat("a", 64)); err != nil {
		t.Fatalf("64-byte id is valid: %v", err)
	}
}

func TestDemoSessionRedacted_SG102_AC2(t *testing.T) {
	ds := DemoSession{Token: rawToken, TenantID: "tn1"}
	for _, out := range []string{fmt.Sprintf("%+v", ds), fmt.Sprintf("%v", ds), fmt.Sprintf("%#v", ds), fmt.Sprintf("%+v", &ds), ds.String()} {
		if strings.Contains(out, rawToken) {
			t.Fatalf("token leaked: %s", out)
		}
	}
	if v := ds.LogValue().String(); strings.Contains(v, rawToken) {
		t.Fatalf("token leaked to slog: %s", v)
	}
}

func TestCreateDemoConflictTwice_SG102_AC1(t *testing.T) {
	r := newRig(true)
	r.rep.conflictOnce, r.rep.hideWinner = true, true
	if _, err := r.s.CreateDemo(context.Background(), access.RoleOwner, "vi", ""); !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v", err)
	}
}
