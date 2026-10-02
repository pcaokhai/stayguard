package app

import (
	"context"
	"errors"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

// countingHasher counts slow-hash work and can pretend every stored hash is a legacy one.
type countingHasher struct {
	fakeHasher
	verifies, hashes int
	legacy           bool
}

func (h *countingHasher) Verify(hash, pin string) bool {
	h.verifies++
	return h.fakeHasher.Verify(hash, pin)
}
func (h *countingHasher) Hash(pin string) (string, error) {
	h.hashes++
	return h.fakeHasher.Hash(pin)
}
func (h *countingHasher) NeedsRehash(string) bool { return h.legacy }

func hardenedRig(t *testing.T, byIP, byCode RateLimiter, h PinHasher) authRig {
	t.Helper()
	r := newAuthRig(t, byIP, byCode)
	r.a.hasher = h
	return r
}

// Hardening 1: a hash of an older scheme verifies and is replaced by the current one on the next successful sign-in.
func TestSignIn_UpgradesLegacyHash_Hardening1(t *testing.T) {
	h := &countingHasher{legacy: true}
	r := hardenedRig(t, allowAll{}, allowAll{}, h)
	before := h.hashes
	if _, err := r.signIn("casa", "ann", pinOK); err != nil {
		t.Fatal(err)
	}
	if h.hashes != before+1 || len(r.repo.upgraded) != 1 || r.repo.upgraded[0] != "hash("+pinOK+")" {
		t.Fatalf("hash not upgraded: hashes=%d upgraded=%v", h.hashes, r.repo.upgraded)
	}
	// A wrong PIN never upgrades anything.
	r.repo.upgraded = nil
	_, _ = r.signIn("casa", "ann", pinWrong)
	if len(r.repo.upgraded) != 0 {
		t.Fatal("upgrade on a wrong PIN")
	}
}

// Hardening 8: rate limits are checked before any bcrypt work, the dummy compare included.
func TestSignIn_RateLimitBeforeBcrypt_Hardening8(t *testing.T) {
	h := &countingHasher{}
	r := hardenedRig(t, denyKey{"ip:1.2.3.4"}, allowAll{}, h)
	base := h.verifies
	for _, code := range []string{"casa", "nowhere"} {
		if _, err := r.signIn(code, "ghost", pinOK); !errors.Is(err, ErrTooManyRequests) {
			t.Fatalf("%s: %v", code, err)
		}
	}
	if h.verifies != base {
		t.Fatalf("bcrypt ran %d times behind a rate limit", h.verifies-base)
	}
}

// Hardening 8: above the global cap on concurrent hashing a request is refused (429) and is not counted as a wrong PIN.
func TestSignIn_BcryptGate_Hardening8(t *testing.T) {
	r := hardenedRig(t, allowAll{}, allowAll{}, &countingHasher{})
	for i := 0; i < maxConcurrentHashes; i++ {
		r.a.gate <- struct{}{}
	}
	if _, err := r.signIn("casa", "ann", pinWrong); !errors.Is(err, ErrTooManyRequests) {
		t.Fatalf("busy gate: %v", err)
	}
	if got := r.repo.users[authKey{"tn_a", "us_ann"}].Pin.FailedCount; got != 0 {
		t.Fatalf("a refused request must not count as a wrong PIN, got %d", got)
	}
	if _, err := r.signIn("nowhere", "ann", pinWrong); !errors.Is(err, ErrTooManyRequests) {
		t.Fatalf("busy gate, unknown guesthouse: %v", err)
	}
	<-r.a.gate
	if _, err := r.signIn("casa", "ann", pinOK); err != nil {
		t.Fatalf("a free slot works again: %v", err)
	}
}

// Hardening 10: an unknown user does the same slow compare as a known one, and every wrong part gives one answer.
func TestSignIn_UnknownUserDoesDummyCompare_Hardening10(t *testing.T) {
	h := &countingHasher{}
	r := hardenedRig(t, allowAll{}, allowAll{}, h)
	base := h.verifies
	for name, c := range map[string][3]string{
		"unknown user": {"casa", "zed", pinOK}, "unknown code": {"nowhere", "ann", pinOK}, "wrong pin": {"casa", "ann", pinWrong},
	} {
		before := h.verifies
		_, err := r.signIn(c[0], c[1], c[2])
		if !errors.Is(err, ErrPinInvalid) || h.verifies != before+1 {
			t.Errorf("%s: err=%v verifies=%d, want exactly one compare", name, err, h.verifies-before)
		}
	}
	if h.verifies != base+3 {
		t.Fatal("compare count")
	}
}

// Hardening 3: the owner's PIN re-entry shares the sign-in lockout counter.
func TestOwnerPinReentryCountsTowardsSignInLock_Hardening3(t *testing.T) {
	r := newStaffRig(t)
	ctx := context.Background()
	v, _, _ := r.s.CreateStaff(ctx, r.owner, "k1", okInput())
	for i := 0; i < 3; i++ {
		_, _ = r.auth.signIn("casa", "ann", pinWrong)
	}
	for i := 0; i < 2; i++ {
		_ = r.s.RemoveStaff(ctx, r.owner, v.ID, pinWrong)
	}
	if _, err := r.auth.signIn("casa", "ann", pinOK); !isLocked(err) {
		t.Fatalf("3 wrong sign-ins plus 2 wrong owner PINs must lock the account: %v", err)
	}
}

// Hardening 2: a MANAGER cannot touch an OWNER or another MANAGER, and cannot grant access above RECEPTIONIST.
func TestStaffTargetGuard_Table_Hardening2(t *testing.T) {
	r := newStaffRig(t)
	ctx := context.Background()
	make := func(key, user, access, position string) string {
		in := okInput()
		in.Username = &user
		in.AppAccess, in.Position = access, position
		v, _, err := r.s.CreateStaff(ctx, r.owner, key, in)
		if err != nil {
			t.Fatal(err)
		}
		return v.ID
	}
	targets := map[string]string{
		"owner": "us_owner", "manager": make("k1", "mgr1", "MANAGER", "MANAGER"),
		"receptionist": make("k2", "rec1", "RECEPTIONIST", "FRONT_DESK"), "housekeeping": make("k3", "hk1", "HOUSEKEEPING", "HOUSEKEEPING"),
	}
	rec := "RECEPTIONIST"
	ops := map[string]func(c Caller, id string) error{
		"reset":  func(c Caller, id string) error { _, err := r.s.ResetPin(ctx, c, id); return err },
		"lock":   func(c Caller, id string) error { _, err := r.s.LockStaff(ctx, c, id); return err },
		"unlock": func(c Caller, id string) error { _, err := r.s.UnlockStaff(ctx, c, id); return err },
		"update": func(c Caller, id string) error {
			_, err := r.s.UpdateStaff(ctx, c, id, StaffUpdate{AppAccess: &rec})
			return err
		},
		"remove": func(c Caller, id string) error { return r.s.RemoveStaff(ctx, c, id, ownerPin) },
	}
	managerMay := map[string]bool{"reset": true, "lock": true, "unlock": true}
	actors := map[string]Caller{
		"manager":      r.mgr,
		"receptionist": {TenantID: "tn_a", UserID: "us_rec", Role: access.RoleReceptionist},
		"housekeeping": {TenantID: "tn_a", UserID: "us_hk", Role: access.RoleHousekeeping},
	}
	for an, actor := range actors {
		for tn, id := range targets {
			for on, op := range ops {
				err := op(actor, id)
				wantForbidden := an != "manager" || !managerMay[on] || tn == "owner" || tn == "manager"
				if wantForbidden != errors.Is(err, access.ErrRoleForbidden) {
					t.Errorf("%s %s on %s: got %v, forbidden=%v expected", an, on, tn, err, wantForbidden)
				}
			}
		}
	}
	// The ceiling holds even if the role table ever let a manager create or promote.
	mgr := "MANAGER"
	if err := managerCeiling(r.mgr, &mgr); !errors.Is(err, access.ErrRoleForbidden) {
		t.Fatalf("ceiling: %v", err)
	}
	if err := managerCeiling(r.owner, &mgr); err != nil {
		t.Fatalf("owner may grant MANAGER: %v", err)
	}
}
