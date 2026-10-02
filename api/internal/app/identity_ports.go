package app

import (
	"context"
	"errors"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

var (
	ErrUnauthenticated = errors.New("unauthenticated")
	ErrSessionExpired  = errors.New("session expired")
	ErrDemoDisabled    = errors.New("demo mode is disabled")
	// ErrTrialNotFound is the single answer for unknown, non-trial and expired tenants, so callers cannot probe.
	ErrTrialNotFound = errors.New("trial not found")
	// ErrConflict is a generic unique violation from the repository.
	ErrConflict = errors.New("conflict")
)

// ValidationError marks bad caller input (mapped to HTTP 422 by the adapter).
type ValidationError struct{ Field, Reason string }

func (e *ValidationError) Error() string { return e.Field + ": " + e.Reason }

type User struct {
	ID, Name string
	Role     access.Role
	Locale   string
	// Blocked is set by UserByID for a user who may not sign in (locked, removed or no app access);
	// MustChangePin while a one-time PIN is still in use.
	Blocked, MustChangePin bool
}

type TenantInfo struct{ ID, Name, Timezone, Currency string }

type Clock interface{ Now() time.Time }

type IDGenerator interface{ New(prefix string) string }

// TokenGenerator returns 256 random bits, URL-safe encoded.
type TokenGenerator interface{ New() (string, error) }

// IdentityRepo is tenant-scoped: every method runs inside the Tx of a UnitOfWork.
type IdentityRepo interface {
	CreateTrialTenant(ctx context.Context, tx Tx, id, name string, expiresAt time.Time) error
	// TrialTenant reports true only for a trial tenant with expires_at after now.
	TrialTenant(ctx context.Context, tx Tx, id string, now time.Time) (bool, error)
	UserByRole(ctx context.Context, tx Tx, role access.Role) (User, bool, error)
	CreateUser(ctx context.Context, tx Tx, id, name string, role access.Role, locale string) (User, error)
	InsertSession(ctx context.Context, tx Tx, tokenHash, userID string, expiresAt time.Time) error
	UserByID(ctx context.Context, tx Tx, id string) (User, bool, error)
	TenantInfo(ctx context.Context, tx Tx) (TenantInfo, error)
	BuildingIDs(ctx context.Context, tx Tx) ([]string, error)
	SetLocale(ctx context.Context, tx Tx, userID, locale string) error
	// UserBuildingLevels is the stored level per building of one user (a missing building is NONE).
	UserBuildingLevels(ctx context.Context, tx Tx, userID string) (map[string]access.Level, error)
	// GrantAllBuildings gives a demo user a level on every building of the tenant.
	GrantAllBuildings(ctx context.Context, tx Tx, userID string, level access.Level) error
}
