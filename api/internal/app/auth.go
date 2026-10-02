package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

const (
	// maxWrongPins wrong PINs within pinWindow lock the account for lockDuration (docs/15 rule 12).
	maxWrongPins = 5
	pinWindow    = 15 * time.Minute
	lockDuration = 15 * time.Minute

	// OneTimePinTTL is how long a one-time PIN stays valid (docs/15 rule 12).
	OneTimePinTTL = 24 * time.Hour

	statusActive = "ACTIVE"
	accessNone   = "NONE"

	auditAccountLocked = "ACCOUNT_LOCKED"
	auditPrefix        = "au"
)

// Auth is PIN sign-in, sign-out and PIN change. It shares the session plumbing of Sessions.
type Auth struct {
	sessions *Sessions
	tenants  TenantByCode
	repo     AuthRepo
	hasher   PinHasher
	audit    AuditWriter
	alerts   AlertWriter
	byIP     RateLimiter
	byCode   RateLimiter
	// dummyHash is checked when the user does not exist, so timing does not reveal it.
	dummyHash string
	// gate caps concurrent bcrypt work: above it a request is refused with 429 instead of queueing CPU.
	gate chan struct{}
}

// maxConcurrentHashes bounds bcrypt operations running at once (about 250 ms of one core each).
const maxConcurrentHashes = 4

func NewAuth(s *Sessions, tenants TenantByCode, repo AuthRepo, hasher PinHasher, audit AuditWriter, alerts AlertWriter, byIP, byCode RateLimiter) (*Auth, error) {
	dummy, err := hasher.Hash("000000")
	if err != nil {
		return nil, err
	}
	return &Auth{s, tenants, repo, hasher, audit, alerts, byIP, byCode, dummy, make(chan struct{}, maxConcurrentHashes)}, nil
}

// verify and hash run the slow hash under the gate; ErrTooManyRequests when the gate is full. Callers count a wrong
// PIN only after verify answered, so a refused request never counts as an attempt.
func (a *Auth) verify(hash, pin string) (bool, error) {
	select {
	case a.gate <- struct{}{}:
		defer func() { <-a.gate }()
		return a.hasher.Verify(hash, pin), nil
	default:
		return false, ErrTooManyRequests
	}
}

func (a *Auth) hash(pin string) (string, error) {
	select {
	case a.gate <- struct{}{}:
		defer func() { <-a.gate }()
		return a.hasher.Hash(pin)
	default:
		return "", ErrTooManyRequests
	}
}

// SignInResult is a new session; the raw token exists only in this value.
type SignInResult struct {
	DemoSession
	MustChangePin bool
}

// signInOutcome is decided inside the transaction so failure counts commit even when the answer is an error.
type signInOutcome struct {
	session SignInResult
	err     error
}

// SignIn checks guesthouse code, user name and PIN. Every wrong part gives ErrPinInvalid.
func (a *Auth) SignIn(ctx context.Context, ip, code, username, pin string) (SignInResult, error) {
	code, username = strings.ToLower(strings.TrimSpace(code)), strings.ToLower(strings.TrimSpace(username))
	if code == "" || username == "" {
		return SignInResult{}, &ValidationError{"guesthouseCode", "required"}
	}
	if access.ValidatePinFormat(pin) != nil {
		return SignInResult{}, &ValidationError{"pin", "must be six digits"}
	}
	if !a.byIP.Allow("ip:"+ip) || !a.byCode.Allow("code:"+code) {
		return SignInResult{}, ErrTooManyRequests
	}
	// A cancelled request still counts its wrong PIN: the transaction below must be able to commit.
	ctx = context.WithoutCancel(ctx)
	tenantID, found, err := a.tenants.TenantByCode(ctx, code)
	if err != nil {
		return SignInResult{}, err
	}
	if !found {
		if _, err = a.verify(a.dummyHash, pin); err != nil {
			return SignInResult{}, err
		}
		return SignInResult{}, ErrPinInvalid
	}
	var out signInOutcome
	err = a.sessions.uow.Do(ctx, tenantID, func(ctx context.Context, tx Tx) error {
		out = a.signInTx(ctx, tx, tenantID, username, pin)
		if out.err != nil && !errors.Is(out.err, ErrPinInvalid) && !isLocked(out.err) {
			return out.err // infrastructure failure: roll back
		}
		return nil
	})
	if err != nil {
		return SignInResult{}, err
	}
	return out.session, out.err
}

func isLocked(err error) bool {
	var l *AccountLockedError
	return errors.As(err, &l)
}

func (a *Auth) signInTx(ctx context.Context, tx Tx, tenantID, username, pin string) signInOutcome {
	now := a.sessions.clock.Now()
	u, ok, err := a.repo.SignInUser(ctx, tx, username)
	if err != nil {
		return signInOutcome{err: err}
	}
	if !ok || u.Status != statusActive || u.Access == accessNone {
		if _, err = a.verify(a.dummyHash, pin); err != nil {
			return signInOutcome{err: err}
		}
		return signInOutcome{err: ErrPinInvalid}
	}
	state, ok, err := a.repo.PinState(ctx, tx, u.ID)
	if err != nil || !ok {
		return signInOutcome{err: errOr(err, ErrPinInvalid)}
	}
	if state.LockedUntil != nil && now.Before(*state.LockedUntil) {
		return signInOutcome{err: &AccountLockedError{Until: *state.LockedUntil}}
	}
	good, err := a.verify(state.Hash, pin)
	if err != nil {
		return signInOutcome{err: err}
	}
	if !good {
		return signInOutcome{err: a.recordWrongPin(ctx, tx, u.ID, state, now)}
	}
	if state.OneTimeExpiresAt != nil && !now.Before(*state.OneTimeExpiresAt) {
		return signInOutcome{err: ErrPinInvalid}
	}
	if state.FailedCount > 0 || state.LockedUntil != nil {
		if err = a.repo.SetPinFailures(ctx, tx, u.ID, 0, nil, nil); err != nil {
			return signInOutcome{err: err}
		}
	}
	if a.hasher.NeedsRehash(state.Hash) {
		if err = a.upgradeHash(ctx, tx, u.ID, pin); err != nil {
			return signInOutcome{err: err}
		}
	}
	s, err := a.sessions.openSession(ctx, tx, tenantID, u.User, now)
	return signInOutcome{session: SignInResult{DemoSession: s, MustChangePin: state.MustChange}, err: err}
}

func errOr(err, fallback error) error {
	if err != nil {
		return err
	}
	return fallback
}

// upgradeHash replaces a hash of an older scheme by the current one, once the PIN is known to be right.
func (a *Auth) upgradeHash(ctx context.Context, tx Tx, userID, pin string) error {
	h, err := a.hash(pin)
	if err != nil {
		return err
	}
	return a.repo.UpdatePinHash(ctx, tx, userID, h)
}

// recordWrongPin counts the failure and, on the fifth within the window, locks the account and audits it.
// It returns ErrPinInvalid, or AccountLockedError when this failure locked it.
func (a *Auth) recordWrongPin(ctx context.Context, tx Tx, userID string, st PinState, now time.Time) error {
	count, first := st.FailedCount+1, st.FirstFailedAt
	if first == nil || !now.Before(first.Add(pinWindow)) {
		count, first = 1, &now
	}
	if count < maxWrongPins {
		return errOr(a.repo.SetPinFailures(ctx, tx, userID, count, first, nil), ErrPinInvalid)
	}
	until := now.Add(lockDuration)
	if err := a.repo.SetPinFailures(ctx, tx, userID, 0, nil, &until); err != nil {
		return err
	}
	// The owner sees the alert; the audit row keeps the event in the activity log too.
	err := a.alerts.Raise(ctx, tx, AlertDraft{ID: a.sessions.ids.New(alertIDPrefix), Kind: AlertAccountLocked, By: userID,
		Details: map[string]string{"lockedUntil": until.UTC().Format(time.RFC3339)}})
	if err != nil {
		return err
	}
	after, _ := json.Marshal(map[string]any{"lockedUntil": until})
	err = a.audit.Append(ctx, tx, AuditEntry{
		ID: a.sessions.ids.New(auditPrefix), Action: auditAccountLocked, EntityType: "user", EntityID: userID, After: after,
	})
	if err != nil {
		return err
	}
	return &AccountLockedError{Until: until}
}

// SignOut revokes the session the request came with.
func (a *Auth) SignOut(ctx context.Context, c Caller) error {
	return a.sessions.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		return a.repo.DeleteSession(ctx, tx, c.SessionHash)
	})
}

// ChangePin replaces the caller's PIN. A wrong current PIN counts like a wrong sign-in PIN.
// Other sessions of the user end; this one stays.
func (a *Auth) ChangePin(ctx context.Context, c Caller, current, next string) error {
	if err := access.ValidatePinFormat(current); err != nil {
		return &ValidationError{"currentPin", "must be six digits"}
	}
	switch err := access.ValidateNewPin(next); {
	case errors.Is(err, access.ErrPinTooSimple):
		return ErrPinTooSimple
	case err != nil:
		return &ValidationError{"newPin", "must be six digits"}
	}
	if current == next {
		return ErrPinTooSimple
	}
	hash, err := a.hash(next)
	if err != nil {
		return err
	}
	ctx = context.WithoutCancel(ctx) // a cancelled request still counts a wrong current PIN
	var outcome error
	err = a.sessions.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		now := a.sessions.clock.Now()
		if outcome = a.checkPin(ctx, tx, c.UserID, current, now); outcome != nil {
			return pinFailureOrErr(outcome) // a wrong PIN commits its count
		}
		if err = a.repo.SetPin(ctx, tx, c.UserID, hash, false, nil, now); err != nil {
			return err
		}
		return a.repo.DeleteOtherSessions(ctx, tx, c.UserID, c.SessionHash)
	})
	return errOr(err, outcome)
}

// checkPin verifies a user's PIN inside tx. It returns nil, ErrPinInvalid or AccountLockedError (the failure
// is counted, and the caller must commit the transaction to keep the count), or an infrastructure error.
func (a *Auth) checkPin(ctx context.Context, tx Tx, userID, pin string, now time.Time) error {
	st, ok, err := a.repo.PinState(ctx, tx, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrPinInvalid
	}
	if st.LockedUntil != nil && now.Before(*st.LockedUntil) {
		return &AccountLockedError{Until: *st.LockedUntil}
	}
	good, err := a.verify(st.Hash, pin)
	if err != nil {
		return err
	}
	if !good {
		return a.recordWrongPin(ctx, tx, userID, st, now)
	}
	return nil
}

// pinFailureOrErr is nil for a counted PIN failure (commit it) and the error itself otherwise (roll back).
func pinFailureOrErr(err error) error {
	if errors.Is(err, ErrPinInvalid) || isLocked(err) {
		return nil
	}
	return err
}

// requireOwnerPin checks the PIN the owner re-enters for a sensitive change. failure is ErrOwnerPinInvalid or an
// AccountLockedError (the wrong try is counted, so the caller commits and returns it); err is an infrastructure
// error (roll back). Both nil means the PIN is right.
func (a *Auth) requireOwnerPin(ctx context.Context, tx Tx, userID, pin string, now time.Time) (failure, err error) {
	switch e := a.checkPin(ctx, tx, userID, pin, now); {
	case e == nil:
		return nil, nil
	case errors.Is(e, ErrPinInvalid):
		return ErrOwnerPinInvalid, nil
	case isLocked(e):
		return e, nil
	default:
		return nil, e
	}
}
