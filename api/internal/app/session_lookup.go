package app

import (
	"context"
	"errors"
	"time"
)

// ErrSessionNotFound means no session row matches the token hash. Expiry is the caller's check.
var ErrSessionNotFound = errors.New("session not found")

// SessionRef is what a token hash resolves to before any tenant is known.
type SessionRef struct {
	TenantID  string
	UserID    string
	ExpiresAt time.Time
}

// SessionResolver finds a session by the hash of its bearer token (ADR-015).
type SessionResolver interface {
	Resolve(ctx context.Context, tokenHash string) (SessionRef, error)
}
