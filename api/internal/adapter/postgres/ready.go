package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

// readyTimeout bounds one readiness check (ping plus the migration lookup).
const readyTimeout = 2 * time.Second

// ErrMigrationsPending means the binary embeds migrations the database has not applied.
var ErrMigrationsPending = errors.New("migrations pending")

// ReadinessProbe implements app.ReadinessProbe over the application pool: the database answers and
// no embedded migration is pending. It reads goose_db_version as the application role (migration 0003 grants that).
type ReadinessProbe struct{ pool *pgxpool.Pool }

func NewReadinessProbe(pool *pgxpool.Pool) *ReadinessProbe { return &ReadinessProbe{pool: pool} }

func (r *ReadinessProbe) Check(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, readyTimeout)
	defer cancel()
	if err := r.pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	db := stdlib.OpenDBFromPool(r.pool) // a view on the pool: closing it does not close the pool's connections
	defer func() { _ = db.Close() }()
	p, err := newProvider(db, true)
	if err != nil {
		return err
	}
	pending, err := p.HasPending(ctx)
	if err != nil {
		return fmt.Errorf("check migrations: %w", err)
	}
	if pending {
		return ErrMigrationsPending
	}
	return nil
}
