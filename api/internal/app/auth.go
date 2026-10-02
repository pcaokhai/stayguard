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
	byIP     RateLimiter
	byCode   RateLimiter
	// dummyHash is checked when the user does not exist, so timing does not reveal it.
	dummyHash string
}

func NewAuth(s *Sessions, tenants TenantByCode, repo AuthRepo, hasher PinHasher, audit AuditWriter, byIP, byCode RateLimiter) (*Auth, error) {
	dummy, err := hasher.Hash("000000")
	if err != nil {
		return nil, err
	}
	return &Auth{s, tenants, repo, hasher, audit, byIP, byCode, dummy}, nil
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
	tenantID, found, err := a.tenants.TenantByCode(ctx, code)
	if err != nil {
		return SignInResult{}, err
	}
	if !found {
		a.hasher.Verify(a.dummyHash, pin)
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
		a.hasher.Verify(a.dummyHash, pin)
		return signInOutcome{err: ErrPinInvalid}
	}
	state, ok, err := a.repo.PinState(ctx, tx, u.ID)
	if err != nil || !ok {
		return signInOutcome{err: errOr(err, ErrPinInvalid)}
	}
	if state.LockedUntil != nil && now.Before(*state.LockedUntil) {
		return signInOutcome{err: &AccountLockedError{Until: *state.LockedUntil}}
	}
	if !a.hasher.Verify(state.Hash, pin) {
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
	s, err := a.sessions.openSession(ctx, tx, tenantID, u.User, now)
	return signInOutcome{session: SignInResult{DemoSession: s, MustChangePin: state.MustChange}, err: err}
}

func errOr(err, fallback error) error {
	if err != nil {
		return err
	}
	return fallback
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
	// ponytail: audit_logs until L-B1 adds alerts, then write the ACCOUNT_LOCKED alert here instead.
	after, _ := json.Marshal(map[string]any{"lockedUntil": until})
	err := a.audit.Append(ctx, tx, AuditEntry{
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
	hash, err := a.hasher.Hash(next)
	if err != nil {
		return err
	}
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
	if !a.hasher.Verify(st.Hash, pin) {
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
