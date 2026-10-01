package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/pcaokhai/stayguard/api/internal/app"
)

type fakePG struct {
	pgx.Tx // unused methods panic: the test only needs Begin
	sp     *fakeSP
}

func (f fakePG) Begin(context.Context) (pgx.Tx, error) { return f.sp, nil }

type fakeSP struct {
	pgx.Tx
	rollbackErr error
}

func (f *fakeSP) Rollback(context.Context) error { return f.rollbackErr }

func TestInSavepointRollbackFailureIsNotConflict_SG102_AC1(t *testing.T) {
	boom := errors.New("rollback failed")
	tx := Tx{tx: fakePG{sp: &fakeSP{rollbackErr: boom}}}
	err := inSavepoint(context.Background(), tx, func(pgx.Tx) error { return &pgconn.PgError{Code: pgUniqueViolation} })
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the rollback failure", err)
	}
	if got := wrap("x", err); errors.Is(got, app.ErrConflict) {
		t.Fatal("a failed savepoint rollback must not be reported as a conflict")
	}
}
