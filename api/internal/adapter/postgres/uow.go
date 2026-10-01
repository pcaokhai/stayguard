package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pcaokhai/stayguard/api/internal/app"
)

// PanicError is returned when fn panics: the transaction is rolled back and the panic becomes an error.
type PanicError struct{ Value any }

func (e *PanicError) Error() string { return fmt.Sprintf("panic in unit of work: %v", e.Value) }

// Tx is the tenant-scoped handle. Repositories take it instead of the pool, so they cannot run
// outside a transaction that has the tenant set. It matches the sqlc DBTX shape.
type Tx struct {
	tx     pgx.Tx
	tenant string
}

// TenantID returns the tenant this transaction is scoped to.
func (t Tx) TenantID() string { return t.tenant }

func (t Tx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return t.tx.Exec(ctx, sql, args...)
}

func (t Tx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return t.tx.Query(ctx, sql, args...)
}

func (t Tx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return t.tx.QueryRow(ctx, sql, args...)
}

// UnitOfWork implements app.UnitOfWork on a pgx pool.
type UnitOfWork struct{ pool *pgxpool.Pool }

var _ app.UnitOfWork = (*UnitOfWork)(nil)

// NewUnitOfWork wraps a pool that connects as the application role.
func NewUnitOfWork(pool *pgxpool.Pool) *UnitOfWork { return &UnitOfWork{pool: pool} }

// Do runs fn in a transaction whose tenant is set transaction-locally (set_config(..., true)),
// so the setting dies with the transaction and cannot leak to the next user of a pooled connection.
func (u *UnitOfWork) Do(ctx context.Context, tenantID string, fn func(ctx context.Context, tx app.Tx) error) (err error) {
	if tenantID == "" {
		return app.ErrTenantRequired
	}
	ctx, cancel := context.WithTimeout(ctx, unitOfWorkTimeout)
	defer cancel()
	pgTx, err := u.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if r := recover(); r != nil {
			err = &PanicError{Value: r}
		}
		if err != nil {
			// A fresh context so a cancelled ctx still rolls back.
			if rbErr := pgTx.Rollback(context.WithoutCancel(ctx)); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
				err = errors.Join(err, fmt.Errorf("rollback: %w", rbErr))
			}
		}
	}()
	if _, err = pgTx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantID); err != nil {
		return fmt.Errorf("set tenant: %w", err)
	}
	if err = fn(ctx, Tx{tx: pgTx, tenant: tenantID}); err != nil {
		return err
	}
	if err = pgTx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
