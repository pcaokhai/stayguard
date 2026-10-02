package app

import (
	"context"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

// Caller is the authenticated identity the auth middleware puts on the request context.
type Caller struct {
	TenantID, UserID string
	Role             access.Role
	Locale           string
	// SessionHash identifies the session of this request (never the raw token).
	SessionHash string
	// PinChangeRequired is true until a one-time PIN is replaced.
	PinChangeRequired bool
	// Levels are the stored building levels of the user, read on every request so a change applies
	// at once. The owner's implicit EDIT is added by permissions.Stored, not stored here.
	Levels map[string]access.Level
}

type callerKey struct{}

func WithCaller(ctx context.Context, c Caller) context.Context {
	return context.WithValue(ctx, callerKey{}, c)
}

func CallerFrom(ctx context.Context) (Caller, bool) {
	c, ok := ctx.Value(callerKey{}).(Caller)
	return c, ok
}
