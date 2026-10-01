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
}

type callerKey struct{}

func WithCaller(ctx context.Context, c Caller) context.Context {
	return context.WithValue(ctx, callerKey{}, c)
}

func CallerFrom(ctx context.Context) (Caller, bool) {
	c, ok := ctx.Value(callerKey{}).(Caller)
	return c, ok
}
