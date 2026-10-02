package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres/sqlcgen"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/money"
)

// StaffRepo implements app.StaffRepo; the tenant always comes from the Tx.
type StaffRepo struct{}

var _ app.StaffRepo = StaffRepo{}

func text(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func textPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

func date(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }

func optDate(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return date(*t)
}

func optInt8(v *money.Vnd) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: v.Int64(), Valid: true}
}

func optInt4(v *int) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*v), Valid: true} //nolint:gosec // validated non-negative counts from a JSON integer
}

func (StaffRepo) ListStaff(ctx context.Context, tx app.Tx, position, userID *string) ([]app.StaffRow, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListStaffRows(ctx, sqlcgen.ListStaffRowsParams{TenantID: t.tenant, Position: text(position), UserID: text(userID)})
	if err != nil {
		return nil, wrap("list staff", err)
	}
	out := make([]app.StaffRow, len(rows))
	for i, r := range rows {
		out[i] = app.StaffRow{
			ID: r.ID, Name: r.Name, Role: r.Role, AppAccess: r.AppAccess, Status: r.Status,
			Username: textPtr(r.Username), Phone: textPtr(r.Phone), Position: r.Position,
			Contract: app.Contract{
				PayType: r.PayType, Rate: money.Vnd(r.Rate), FixedAllowance: money.Vnd(r.FixedAllowance),
				StandardShifts: int(r.StandardShifts), StartDate: r.StartDate.Time, AnnualLeaveDays: int(r.AnnualLeaveDays),
			},
			PinLockedUntil: optTime(r.LockedUntil), LastActivityAt: optTime(r.LastActivityAt),
		}
	}
	return out, nil
}

func (StaffRepo) StaffLevels(ctx context.Context, tx app.Tx) (map[string]map[string]access.Level, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListStaffBuildingLevels(ctx, t.tenant)
	if err != nil {
		return nil, wrap("list building levels", err)
	}
	out := map[string]map[string]access.Level{}
	for _, r := range rows {
		l, err := access.ParseLevel(r.Level)
		if err != nil {
			return nil, wrap("building level", err)
		}
		if out[r.UserID] == nil {
			out[r.UserID] = map[string]access.Level{}
		}
		out[r.UserID][r.BuildingID] = l
	}
	return out, nil
}

func (StaffRepo) InsertStaff(ctx context.Context, tx app.Tx, s app.StaffInsert) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	q := sqlcgen.New(t)
	if err = q.InsertStaffUser(ctx, sqlcgen.InsertStaffUserParams{
		ID: s.ID, TenantID: t.tenant, Name: s.Name, Role: s.Role, AppAccess: s.AppAccess, Username: text(s.Username),
	}); err != nil {
		return wrap("insert staff user", err)
	}
	k := s.Contract
	return wrap("insert staff profile", q.InsertStaffProfile(ctx, sqlcgen.InsertStaffProfileParams{
		TenantID: t.tenant, UserID: s.ID, Phone: text(s.Phone), Position: s.Position, PayType: k.PayType,
		Rate: k.Rate.Int64(), FixedAllowance: k.FixedAllowance.Int64(),
		StandardShifts: int32(k.StandardShifts),                                      //nolint:gosec // validated non-negative counts from a JSON integer
		StartDate:      date(k.StartDate), AnnualLeaveDays: int32(k.AnnualLeaveDays), //nolint:gosec // as above
	}))
}

func (StaffRepo) InsertOwner(ctx context.Context, tx app.Tx, id, name, username string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("insert owner", sqlcgen.New(t).InsertStaffUser(ctx, sqlcgen.InsertStaffUserParams{
		ID: id, TenantID: t.tenant, Name: name, Role: string(access.RoleOwner), AppAccess: string(access.RoleOwner), Username: text(&username),
	}))
}

func (StaffRepo) UpdateStaff(ctx context.Context, tx app.Tx, id string, p app.StaffPatch) (bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return false, err
	}
	q := sqlcgen.New(t)
	n, err := q.UpdateStaffUser(ctx, sqlcgen.UpdateStaffUserParams{
		Name: text(p.Name), Role: text(p.Role), AppAccess: text(p.AppAccess), TenantID: t.tenant, ID: id,
	})
	if err != nil || n == 0 {
		return false, wrap("update staff user", err)
	}
	return true, wrap("update staff profile", q.UpdateStaffProfile(ctx, sqlcgen.UpdateStaffProfileParams{
		Phone: text(p.Phone), Position: text(p.Position), PayType: text(p.PayType), Rate: optInt8(p.Rate),
		FixedAllowance: optInt8(p.FixedAllowance), StandardShifts: optInt4(p.StandardShifts), StartDate: optDate(p.StartDate),
		AnnualLeaveDays: optInt4(p.AnnualLeaveDays), TenantID: t.tenant, UserID: id,
	}))
}

func (StaffRepo) SetStatus(ctx context.Context, tx app.Tx, id, status string) (bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return false, err
	}
	n, err := sqlcgen.New(t).SetStaffStatus(ctx, sqlcgen.SetStaffStatusParams{Status: status, TenantID: t.tenant, ID: id})
	return n > 0, wrap("set staff status", err)
}

func (StaffRepo) DeleteUserSessions(ctx context.Context, tx app.Tx, userID string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("delete user sessions", sqlcgen.New(t).DeleteUserSessions(ctx, sqlcgen.DeleteUserSessionsParams{TenantID: t.tenant, UserID: userID}))
}

func (StaffRepo) SetBuildingLevel(ctx context.Context, tx app.Tx, userID, buildingID string, level access.Level, now time.Time) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("set building level", sqlcgen.New(t).UpsertBuildingPermission(ctx, sqlcgen.UpsertBuildingPermissionParams{
		TenantID: t.tenant, UserID: userID, BuildingID: buildingID, Level: level.String(), Now: pgtype.Timestamptz{Time: now, Valid: true},
	}))
}

func (StaffRepo) BuildingLevel(ctx context.Context, tx app.Tx, userID, buildingID string) (access.Level, error) {
	t, err := pgTx(tx)
	if err != nil {
		return access.NONE, err
	}
	s, err := sqlcgen.New(t).GetBuildingPermission(ctx, sqlcgen.GetBuildingPermissionParams{TenantID: t.tenant, UserID: userID, BuildingID: buildingID})
	if errors.Is(err, pgx.ErrNoRows) {
		return access.NONE, nil
	}
	if err != nil {
		return access.NONE, wrap("get building level", err)
	}
	return access.ParseLevel(s)
}

func (StaffRepo) BuildingExists(ctx context.Context, tx app.Tx, id string) (bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return false, err
	}
	ok, err := sqlcgen.New(t).BuildingExists(ctx, sqlcgen.BuildingExistsParams{TenantID: t.tenant, ID: id})
	return ok, wrap("building exists", err)
}

func (StaffRepo) BuildingIDs(ctx context.Context, tx app.Tx) ([]string, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	ids, err := sqlcgen.New(t).ListBuildingIDs(ctx, t.tenant)
	return ids, wrap("list building ids", err)
}

func (StaffRepo) UserRole(ctx context.Context, tx app.Tx, id string) (access.Role, string, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return "", "", false, err
	}
	row, err := sqlcgen.New(t).GetUserRoleStatus(ctx, sqlcgen.GetUserRoleStatusParams{TenantID: t.tenant, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, wrap("get user role", err)
	}
	r, err := access.ParseRole(row.Role)
	return r, row.Status, err == nil, wrap("user role", err)
}

func (StaffRepo) PermissionUsers(ctx context.Context, tx app.Tx) ([]app.User, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListPermissionUsers(ctx, t.tenant)
	if err != nil {
		return nil, wrap("list permission users", err)
	}
	out := make([]app.User, 0, len(rows))
	for _, r := range rows {
		role, err := access.ParseRole(r.Role)
		if err != nil {
			return nil, wrap("permission user role", err)
		}
		out = append(out, app.User{ID: r.ID, Name: r.Name, Role: role})
	}
	return out, nil
}
