package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres/sqlcgen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// TenantResolver implements app.TenantByCode with the tenants_signin_lookup policy.
type TenantResolver struct{ pool *pgxpool.Pool }

var _ app.TenantByCode = (*TenantResolver)(nil)

func NewTenantResolver(pool *pgxpool.Pool) *TenantResolver { return &TenantResolver{pool: pool} }

// TenantByCode reads one tenant id in its own read-only transaction; the code setting is transaction-local.
func (r *TenantResolver) TenantByCode(ctx context.Context, code string) (id string, found bool, err error) {
	ctx, cancel := context.WithTimeout(ctx, unitOfWorkTimeout)
	defer cancel()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", false, fmt.Errorf("begin tenant lookup: %w", err)
	}
	defer func() {
		if rerr := rollback(ctx, tx); rerr != nil {
			err = errors.Join(err, rerr)
		}
	}()
	if _, err = tx.Exec(ctx, `SELECT set_config('app.signin_code', $1, true)`, code); err != nil {
		return "", false, fmt.Errorf("set sign-in code: %w", err)
	}
	err = tx.QueryRow(ctx, `SELECT id FROM app.tenants WHERE guesthouse_code = $1`, code).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("select tenant by code: %w", err)
	}
	return id, true, nil
}

// AuthRepo implements app.AuthRepo; the tenant always comes from the Tx.
type AuthRepo struct{}

var _ app.AuthRepo = AuthRepo{}

func NewAuthRepo() AuthRepo { return AuthRepo{} }

func nullTime(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func optTime(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func (AuthRepo) SignInUser(ctx context.Context, tx app.Tx, username string) (app.SignInUser, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.SignInUser{}, false, err
	}
	row, err := sqlcgen.New(t).GetSignInUser(ctx, sqlcgen.GetSignInUserParams{TenantID: t.tenant, Username: pgtype.Text{String: username, Valid: true}})
	u, ok, err := optUser(row.ID, row.Name, row.Role, row.Locale, err, "select sign-in user")
	if err != nil || !ok {
		return app.SignInUser{}, false, err
	}
	return app.SignInUser{User: u, Access: row.AppAccess, Status: row.Status, Pin: app.PinState{
		Hash: row.PinHash, FailedCount: int(row.FailedCount), FirstFailedAt: optTime(row.FirstFailedAt),
		LockedUntil: optTime(row.LockedUntil), MustChange: row.MustChange, OneTimeExpiresAt: optTime(row.OneTimeExpiresAt),
	}}, true, nil
}

func (AuthRepo) PinState(ctx context.Context, tx app.Tx, userID string) (app.PinState, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.PinState{}, false, err
	}
	row, err := sqlcgen.New(t).GetPinState(ctx, sqlcgen.GetPinStateParams{TenantID: t.tenant, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.PinState{}, false, nil
	}
	if err != nil {
		return app.PinState{}, false, wrap("select pin state", err)
	}
	return app.PinState{
		Hash: row.PinHash, FailedCount: int(row.FailedCount), FirstFailedAt: optTime(row.FirstFailedAt),
		LockedUntil: optTime(row.LockedUntil), MustChange: row.MustChange, OneTimeExpiresAt: optTime(row.OneTimeExpiresAt),
	}, true, nil
}

func (AuthRepo) SetPinFailures(ctx context.Context, tx app.Tx, userID string, count int, first, lockedUntil *time.Time) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("update pin failures", sqlcgen.New(t).UpdatePinFailures(ctx, sqlcgen.UpdatePinFailuresParams{
		FailedCount:   int32(count), //nolint:gosec // the use case keeps the count below five
		FirstFailedAt: nullTime(first), LockedUntil: nullTime(lockedUntil), TenantID: t.tenant, UserID: userID,
	}))
}

func (AuthRepo) SetPin(ctx context.Context, tx app.Tx, userID, hash string, mustChange bool, oneTimeExpiresAt *time.Time, now time.Time) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("set pin", sqlcgen.New(t).UpsertPin(ctx, sqlcgen.UpsertPinParams{
		TenantID: t.tenant, UserID: userID, PinHash: hash, MustChange: mustChange,
		OneTimeExpiresAt: nullTime(oneTimeExpiresAt), Now: pgtype.Timestamptz{Time: now, Valid: true},
	}))
}

func (AuthRepo) DeleteSession(ctx context.Context, tx app.Tx, tokenHash string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("delete session", sqlcgen.New(t).DeleteSession(ctx, sqlcgen.DeleteSessionParams{TenantID: t.tenant, TokenHash: tokenHash}))
}

func (AuthRepo) DeleteOtherSessions(ctx context.Context, tx app.Tx, userID, keepHash string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("delete other sessions", sqlcgen.New(t).DeleteOtherUserSessions(ctx, sqlcgen.DeleteOtherUserSessionsParams{
		TenantID: t.tenant, UserID: userID, KeepHash: keepHash,
	}))
}
