package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pcaokhai/stayguard/api/internal/app"
)

// SessionResolver implements app.SessionResolver with the sessions_lookup policy (ADR-015).
type SessionResolver struct{ pool *pgxpool.Pool }

var _ app.SessionResolver = (*SessionResolver)(nil)

// NewSessionResolver wraps a pool that connects as the application role.
func NewSessionResolver(pool *pgxpool.Pool) *SessionResolver { return &SessionResolver{pool: pool} }

// Resolve reads one session in its own read-only transaction. The hash setting is transaction-local
// and the transaction is always rolled back, so nothing leaks to the next user of the connection.
// The hash is never logged or put into an error.
func (r *SessionResolver) Resolve(ctx context.Context, tokenHash string) (ref app.SessionRef, err error) {
	ctx, cancel := context.WithTimeout(ctx, unitOfWorkTimeout)
	defer cancel()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ref, fmt.Errorf("begin session lookup: %w", err)
	}
	defer func() { err = errors.Join(err, rollback(ctx, tx)) }()
	if _, err = tx.Exec(ctx, `SELECT set_config('app.session_hash', $1, true)`, tokenHash); err != nil {
		return ref, fmt.Errorf("set session hash: %w", err)
	}
	// The explicit filter repeats the policy: tenant scoping never rests on RLS alone.
	err = tx.QueryRow(ctx, `SELECT tenant_id, user_id, expires_at FROM app.sessions WHERE token_hash = $1`, tokenHash).
		Scan(&ref.TenantID, &ref.UserID, &ref.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.SessionRef{}, app.ErrSessionNotFound
	}
	if err != nil {
		return app.SessionRef{}, fmt.Errorf("select session: %w", err)
	}
	return ref, nil
}
