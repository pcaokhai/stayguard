package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/pcaokhai/stayguard/api/internal/app"
)

// pgTx recovers the adapter transaction from the opaque port handle.
func pgTx(tx app.Tx) (Tx, error) {
	t, ok := tx.(Tx)
	if !ok {
		return Tx{}, fmt.Errorf("transaction was not created by the postgres unit of work (%T)", tx)
	}
	return t, nil
}

// pgUniqueViolation is SQLSTATE 23505.
const pgUniqueViolation = "23505"

// mapUnique turns a unique violation into the generic app.ErrConflict. The pg error carries the
// constraint name, detail and offending values, so none of it is wrapped.
func mapUnique(err error) error {
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == pgUniqueViolation {
		return app.ErrConflict
	}
	return err
}

// inSavepoint runs fn in a savepoint (pgx nested Begin). A failed statement aborts a PostgreSQL
// transaction; rolling back to the savepoint keeps the caller's transaction usable.
func inSavepoint(ctx context.Context, t Tx, fn func(q pgx.Tx) error) error {
	sp, err := t.tx.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin savepoint: %w", err)
	}
	if err := fn(sp); err != nil {
		if rerr := rollback(ctx, sp); rerr != nil {
			err = errors.Join(err, rerr)
		}
		return err
	}
	if err := sp.Commit(ctx); err != nil {
		return fmt.Errorf("release savepoint: %w", err)
	}
	return nil
}
