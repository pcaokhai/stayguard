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
	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

// ErrTenantMismatch means a tenant id argument differs from the transaction's tenant. The message
// carries no ids.
var ErrTenantMismatch = errors.New("tenant id does not match the transaction tenant")

// IdentityRepo implements app.IdentityRepo. The tenant always comes from the Tx, never from callers.
type IdentityRepo struct{}

var _ app.IdentityRepo = IdentityRepo{}

// NewIdentityRepo returns the repo; it holds no state.
func NewIdentityRepo() IdentityRepo { return IdentityRepo{} }

func (IdentityRepo) CreateTrialTenant(ctx context.Context, tx app.Tx, id, name string, expiresAt time.Time) error {
	t, err := ownTx(tx, id)
	if err != nil {
		return err
	}
	err = sqlcgen.New(t).InsertTrialTenant(ctx, sqlcgen.InsertTrialTenantParams{
		ID: t.tenant, Name: name, ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	return wrap("insert trial tenant", err)
}

func (IdentityRepo) TrialTenant(ctx context.Context, tx app.Tx, id string, now time.Time) (bool, error) {
	t, err := ownTx(tx, id)
	if err != nil {
		return false, err
	}
	ok, err := sqlcgen.New(t).TrialTenantActive(ctx, sqlcgen.TrialTenantActiveParams{
		TenantID: t.tenant, Now: pgtype.Timestamptz{Time: now, Valid: true},
	})
	return ok, wrap("select trial tenant", err)
}

func (IdentityRepo) UserByRole(ctx context.Context, tx app.Tx, role access.Role) (app.User, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.User{}, false, err
	}
	row, err := sqlcgen.New(t).GetUserByRole(ctx, sqlcgen.GetUserByRoleParams{TenantID: t.tenant, Role: string(role)})
	return optUser(row.ID, row.Name, row.Role, row.Locale, err, "select user by role")
}

func (IdentityRepo) UserByID(ctx context.Context, tx app.Tx, id string) (app.User, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.User{}, false, err
	}
	row, err := sqlcgen.New(t).GetUserByID(ctx, sqlcgen.GetUserByIDParams{TenantID: t.tenant, ID: id})
	u, ok, err := optUser(row.ID, row.Name, row.Role, row.Locale, err, "select user")
	u.Blocked, u.MustChangePin = row.Blocked, row.MustChangePin
	return u, ok, err
}

// CreateUser runs in a savepoint: a unique violation must not abort the caller's transaction,
// because app.Sessions reads the winner's user right after a conflict.
func (IdentityRepo) CreateUser(ctx context.Context, tx app.Tx, id, name string, role access.Role, locale string) (app.User, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.User{}, err
	}
	var row sqlcgen.InsertUserRow
	err = inSavepoint(ctx, t, func(q pgx.Tx) error {
		var qerr error
		row, qerr = sqlcgen.New(q).InsertUser(ctx, sqlcgen.InsertUserParams{
			ID: id, TenantID: t.tenant, Name: name, Role: string(role), Locale: locale,
		})
		return qerr
	})
	if err != nil {
		return app.User{}, wrap("insert user", err)
	}
	u, _, err := optUser(row.ID, row.Name, row.Role, row.Locale, nil, "insert user")
	return u, err
}

func (IdentityRepo) InsertSession(ctx context.Context, tx app.Tx, tokenHash, userID string, expiresAt time.Time) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	err = sqlcgen.New(t).InsertSession(ctx, sqlcgen.InsertSessionParams{
		TokenHash: tokenHash, TenantID: t.tenant, UserID: userID, ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	return wrap("insert session", err)
}

func (IdentityRepo) TenantInfo(ctx context.Context, tx app.Tx) (app.TenantInfo, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.TenantInfo{}, err
	}
	row, err := sqlcgen.New(t).GetTenantInfo(ctx, t.tenant)
	if err != nil {
		return app.TenantInfo{}, wrap("select tenant info", err)
	}
	return app.TenantInfo{ID: row.ID, Name: row.Name, Timezone: row.TimeZone, Currency: row.Currency}, nil
}

func (IdentityRepo) BuildingIDs(ctx context.Context, tx app.Tx) ([]string, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	ids, err := sqlcgen.New(t).ListBuildingIDs(ctx, t.tenant)
	return ids, wrap("list buildings", err)
}

func (IdentityRepo) SetLocale(ctx context.Context, tx app.Tx, userID, locale string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	n, err := sqlcgen.New(t).UpdateUserLocale(ctx, sqlcgen.UpdateUserLocaleParams{TenantID: t.tenant, ID: userID, Locale: locale})
	if err != nil {
		return wrap("update locale", err)
	}
	if n == 0 {
		return errors.New("update locale: user not found")
	}
	return nil
}

// wrap adds context and maps unique violations; nil stays nil.
func wrap(what string, err error) error {
	if err == nil {
		return nil
	}
	if cerr := mapUnique(err); errors.Is(cerr, app.ErrConflict) {
		return cerr
	}
	return fmt.Errorf("%s: %w", what, err)
}

// optUser maps a row to a user; no rows is (zero, false, nil).
func optUser(id, name, role, locale string, err error, what string) (app.User, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.User{}, false, nil
	}
	if err != nil {
		return app.User{}, false, wrap(what, err)
	}
	r, err := access.ParseRole(role)
	if err != nil {
		return app.User{}, false, fmt.Errorf("%s: %w", what, err)
	}
	return app.User{ID: id, Name: name, Role: r, Locale: locale}, true, nil
}

// ownTx is pgTx plus a check that id is the transaction's own tenant, so the SQL filter comes from the Tx.
func ownTx(tx app.Tx, id string) (Tx, error) {
	t, err := pgTx(tx)
	if err != nil {
		return Tx{}, err
	}
	if id != t.tenant {
		return Tx{}, ErrTenantMismatch
	}
	return t, nil
}

func (IdentityRepo) UserBuildingLevels(ctx context.Context, tx app.Tx, userID string) (map[string]access.Level, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListUserBuildingLevels(ctx, sqlcgen.ListUserBuildingLevelsParams{TenantID: t.tenant, UserID: userID})
	if err != nil {
		return nil, wrap("select building levels", err)
	}
	out := make(map[string]access.Level, len(rows))
	for _, r := range rows {
		l, err := access.ParseLevel(r.Level)
		if err != nil {
			return nil, wrap("building level", err)
		}
		out[r.BuildingID] = l
	}
	return out, nil
}

func (IdentityRepo) GrantAllBuildings(ctx context.Context, tx app.Tx, userID string, level access.Level) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("grant all buildings", sqlcgen.New(t).GrantAllBuildings(ctx, sqlcgen.GrantAllBuildingsParams{
		UserID: userID, Level: level.String(), TenantID: t.tenant,
	}))
}
