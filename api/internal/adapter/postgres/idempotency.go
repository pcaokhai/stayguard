package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres/sqlcgen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// DefaultIdempotencyTTL is how long a key is remembered (plan SG-003 Ruling 4).
const DefaultIdempotencyTTL = 24 * time.Hour

const (
	minHTTPStatus = 100
	maxHTTPStatus = 599
)

// IdempotencyStore implements app.IdempotencyStore on the unit-of-work transaction.
type IdempotencyStore struct{ ttl time.Duration }

var _ app.IdempotencyStore = (*IdempotencyStore)(nil)

// NewIdempotencyStore uses DefaultIdempotencyTTL when ttl is not positive.
func NewIdempotencyStore(ttl time.Duration) *IdempotencyStore {
	if ttl <= 0 {
		ttl = DefaultIdempotencyTTL
	}
	return &IdempotencyStore{ttl: ttl}
}

// Begin claims the key. A concurrent first call blocks on the primary key until the other transaction
// ends, then sees its stored response (replay) instead of running the effect twice.
func (s *IdempotencyStore) Begin(ctx context.Context, tx app.Tx, route, key, requestHash string) (app.IdempotencyOutcome, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.IdempotencyOutcome{}, err
	}
	q := sqlcgen.New(t)
	_, err = q.BeginIdempotencyKey(ctx, sqlcgen.BeginIdempotencyKeyParams{
		TenantID: t.tenant, Route: route, Key: key, RequestHash: requestHash, TtlSeconds: s.ttl.Seconds(),
	})
	if err == nil {
		return app.IdempotencyOutcome{}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return app.IdempotencyOutcome{}, fmt.Errorf("begin idempotency key: %w", err)
	}
	return s.existing(ctx, q, t.tenant, route, key, requestHash)
}

func (s *IdempotencyStore) existing(ctx context.Context, q *sqlcgen.Queries, tenant, route, key, requestHash string) (app.IdempotencyOutcome, error) {
	row, err := q.GetIdempotencyKey(ctx, sqlcgen.GetIdempotencyKeyParams{TenantID: tenant, Route: route, Key: key})
	if err != nil {
		return app.IdempotencyOutcome{}, fmt.Errorf("read idempotency key: %w", err)
	}
	if row.RequestHash != requestHash {
		return app.IdempotencyOutcome{}, app.ErrIdempotencyKeyReused
	}
	if !row.StatusCode.Valid {
		return app.IdempotencyOutcome{}, app.ErrIdempotencyIncomplete
	}
	return app.IdempotencyOutcome{Replay: true, Status: int(row.StatusCode.Int32), Body: row.ResponseBody}, nil
}

// Complete stores the response; call it in the transaction that made the effect.
func (s *IdempotencyStore) Complete(ctx context.Context, tx app.Tx, route, key string, status int, body []byte) error {
	if status < minHTTPStatus || status > maxHTTPStatus {
		return fmt.Errorf("complete idempotency key: status %d is not an HTTP status", status)
	}
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	n, err := sqlcgen.New(t).CompleteIdempotencyKey(ctx, sqlcgen.CompleteIdempotencyKeyParams{
		TenantID: t.tenant, Route: route, Key: key, StatusCode: pgtype.Int4{Int32: int32(status), Valid: true}, ResponseBody: body,
	})
	if err != nil {
		return fmt.Errorf("complete idempotency key: %w", err)
	}
	if n != 1 {
		return fmt.Errorf("complete idempotency key: %d rows updated, Begin was not called", n)
	}
	return nil
}
