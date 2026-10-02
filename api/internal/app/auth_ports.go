package app

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrPinInvalid is the one answer for a wrong guesthouse code, user name or PIN.
	ErrPinInvalid = errors.New("pin invalid")
	// ErrTooManyRequests is a sign-in rate limit hit (per IP or per guesthouse code).
	ErrTooManyRequests = errors.New("too many requests")
	// ErrPinChangeRequired blocks every call but changeMyPin until a one-time PIN is replaced.
	ErrPinChangeRequired = errors.New("pin change required")
	ErrPinTooSimple      = errors.New("pin too simple")
)

// AccountLockedError carries the end of the lock for the problem body.
type AccountLockedError struct{ Until time.Time }

func (e *AccountLockedError) Error() string { return "account locked" }

// TenantByCode finds a tenant from its guesthouse code before any tenant is known (policy tenants_signin_lookup).
type TenantByCode interface {
	TenantByCode(ctx context.Context, code string) (tenantID string, found bool, err error)
}

// PinHasher is a slow salted hash.
type PinHasher interface {
	Hash(pin string) (string, error)
	Verify(hash, pin string) bool
}

// RateLimiter counts calls per key and says whether the call is within the limit.
type RateLimiter interface{ Allow(key string) bool }

// SignInUser is a user with the credential state needed to check a PIN.
type SignInUser struct {
	User
	Access string // users.app_access
	Status string // ACTIVE, LOCKED or REMOVED
	Pin    PinState
}

// PinState is one row of pin_credentials.
type PinState struct {
	Hash             string
	FailedCount      int
	FirstFailedAt    *time.Time
	LockedUntil      *time.Time
	MustChange       bool
	OneTimeExpiresAt *time.Time
}

// AuthRepo is tenant-scoped like IdentityRepo.
type AuthRepo interface {
	SignInUser(ctx context.Context, tx Tx, username string) (SignInUser, bool, error)
	// PinState locks the credential row so concurrent wrong PINs count one by one.
	PinState(ctx context.Context, tx Tx, userID string) (PinState, bool, error)
	SetPinFailures(ctx context.Context, tx Tx, userID string, count int, first, lockedUntil *time.Time) error
	// SetPin replaces the PIN and clears failures and the lock.
	SetPin(ctx context.Context, tx Tx, userID, hash string, mustChange bool, oneTimeExpiresAt *time.Time, now time.Time) error
	DeleteSession(ctx context.Context, tx Tx, tokenHash string) error
	DeleteOtherSessions(ctx context.Context, tx Tx, userID, keepHash string) error
}
