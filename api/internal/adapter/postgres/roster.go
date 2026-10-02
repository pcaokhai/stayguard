package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres/sqlcgen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// RosterRepo implements app.RosterRepo. The tenant always comes from the Tx.
type RosterRepo struct{}

var _ app.RosterRepo = RosterRepo{}

func dateOf(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }

func (RosterRepo) Timezone(ctx context.Context, tx app.Tx) (string, error) {
	return RoomRepo{}.Timezone(ctx, tx)
}

func (RosterRepo) Assignments(ctx context.Context, tx app.Tx, from, to time.Time) ([]app.RosterCell, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListRosterAssignments(ctx, sqlcgen.ListRosterAssignmentsParams{TenantID: t.tenant, FromDate: dateOf(from), ToDate: dateOf(to)})
	if err != nil {
		return nil, wrap("list roster", err)
	}
	out := make([]app.RosterCell, len(rows))
	for i, r := range rows {
		out[i] = app.RosterCell{UserID: r.UserID, UserName: r.UserName, Date: r.WorkDate.Time, Shift: r.Shift}
	}
	return out, nil
}

func (RosterRepo) ActiveUsers(ctx context.Context, tx app.Tx, ids []string) (map[string]bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	found, err := sqlcgen.New(t).ListActiveUserIDs(ctx, sqlcgen.ListActiveUserIDsParams{TenantID: t.tenant, UserIds: ids})
	if err != nil {
		return nil, wrap("list active users", err)
	}
	out := make(map[string]bool, len(found))
	for _, id := range found {
		out[id] = true
	}
	return out, nil
}

func (RosterRepo) Remove(ctx context.Context, tx app.Tx, cells []app.RosterCell) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	q := sqlcgen.New(t)
	for _, c := range cells {
		if _, err := q.DeleteRosterAssignment(ctx, sqlcgen.DeleteRosterAssignmentParams{TenantID: t.tenant, UserID: c.UserID, WorkDate: dateOf(c.Date), Shift: c.Shift}); err != nil {
			return wrap("delete roster assignment", err)
		}
	}
	return nil
}

func (RosterRepo) Add(ctx context.Context, tx app.Tx, cells []app.RosterCell, by string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	q := sqlcgen.New(t)
	for _, c := range cells {
		if _, err := q.InsertRosterAssignment(ctx, sqlcgen.InsertRosterAssignmentParams{TenantID: t.tenant, UserID: c.UserID, WorkDate: dateOf(c.Date),
			Shift: c.Shift, CreatedBy: optText(by)}); err != nil {
			return writeFailure("insert roster assignment", err)
		}
	}
	return nil
}

func (RosterRepo) CountBetween(ctx context.Context, tx app.Tx, from, to time.Time) (int, error) {
	t, err := pgTx(tx)
	if err != nil {
		return 0, err
	}
	n, err := sqlcgen.New(t).CountRosterBetween(ctx, sqlcgen.CountRosterBetweenParams{TenantID: t.tenant, FromDate: dateOf(from), ToDate: dateOf(to)})
	return int(n), wrap("count roster", err)
}

func (RosterRepo) CopyWeek(ctx context.Context, tx app.Tx, srcFrom time.Time, by string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	_, err = sqlcgen.New(t).CopyRosterWeek(ctx, sqlcgen.CopyRosterWeekParams{TenantID: t.tenant, SrcFrom: dateOf(srcFrom), CreatedBy: optText(by)})
	return wrap("copy roster week", err)
}

func (RosterRepo) Scheduled(ctx context.Context, tx app.Tx, userID string, day time.Time) ([]string, error) {
	return ShiftRepo{}.Scheduled(ctx, tx, userID, day)
}

func toLeaveRow(r sqlcgen.ListLeaveRow) app.LeaveRow {
	return app.LeaveRow{ID: r.ID, UserID: r.UserID, UserName: r.UserName, Shift: r.Shift, Kind: r.Kind, Reason: r.Reason, CoverUserID: r.CoverUserID,
		Status: r.Status, DeclineReason: r.DeclineReason, From: r.FromDate.Time, To: r.ToDate.Time, CreatedAt: r.CreatedAt.Time, DecidedAt: timePtr(r.DecidedAt)}
}

func (RosterRepo) Leave(ctx context.Context, tx app.Tx, f app.LeaveFilter) ([]app.LeaveRow, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	arg := sqlcgen.ListLeaveParams{TenantID: t.tenant, Status: optText(f.Status), UserID: optText(f.UserID), StandingOnly: f.Standing}
	if f.From != nil && f.To != nil {
		arg.FromDate, arg.ToDate = dateOf(*f.From), dateOf(*f.To)
	}
	rows, err := sqlcgen.New(t).ListLeave(ctx, arg)
	if err != nil {
		return nil, wrap("list leave", err)
	}
	out := make([]app.LeaveRow, len(rows))
	for i, r := range rows {
		out[i] = toLeaveRow(r)
	}
	return out, nil
}

func (RosterRepo) LeaveByID(ctx context.Context, tx app.Tx, id string) (app.LeaveRow, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.LeaveRow{}, false, err
	}
	r, err := sqlcgen.New(t).GetLeave(ctx, sqlcgen.GetLeaveParams{TenantID: t.tenant, LeaveID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.LeaveRow{}, false, nil
	}
	if err != nil {
		return app.LeaveRow{}, false, wrap("select leave", err)
	}
	return toLeaveRow(sqlcgen.ListLeaveRow(r)), true, nil
}

func (RosterRepo) LockLeave(ctx context.Context, tx app.Tx, id string) (bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return false, err
	}
	_, err = sqlcgen.New(t).LockLeave(ctx, sqlcgen.LockLeaveParams{TenantID: t.tenant, LeaveID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, wrap("lock leave", err)
}

func (RosterRepo) InsertLeave(ctx context.Context, tx app.Tx, l app.LeaveRow) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return writeFailure("insert leave", sqlcgen.New(t).InsertLeave(ctx, sqlcgen.InsertLeaveParams{ID: l.ID, TenantID: t.tenant, UserID: l.UserID,
		FromDate: dateOf(l.From), ToDate: dateOf(l.To), Shift: optText(l.Shift), Kind: l.Kind, Reason: optText(l.Reason),
		CoverUserID: optText(l.CoverUserID), CreatedAt: ts(l.CreatedAt)}))
}

func (RosterRepo) SetLeaveStatus(ctx context.Context, tx app.Tx, id, status, declineReason string, at time.Time, by string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("set leave status", sqlcgen.New(t).SetLeaveStatus(ctx, sqlcgen.SetLeaveStatusParams{TenantID: t.tenant, LeaveID: id, Status: status,
		DeclineReason: optText(declineReason), DecidedAt: ts(at), DecidedBy: optText(by)}))
}

func (RosterRepo) OverlappingLeave(ctx context.Context, tx app.Tx, userID string, from, to time.Time) (int, error) {
	t, err := pgTx(tx)
	if err != nil {
		return 0, err
	}
	n, err := sqlcgen.New(t).CountOverlappingLeave(ctx, sqlcgen.CountOverlappingLeaveParams{TenantID: t.tenant, UserID: userID, FromDate: dateOf(from), ToDate: dateOf(to)})
	return int(n), wrap("count overlapping leave", err)
}

func (RosterRepo) PaidLeaveDays(ctx context.Context, tx app.Tx, userID string, yearStart, yearEnd time.Time) (int, error) {
	t, err := pgTx(tx)
	if err != nil {
		return 0, err
	}
	n, err := sqlcgen.New(t).SumPaidLeaveDays(ctx, sqlcgen.SumPaidLeaveDaysParams{TenantID: t.tenant, UserID: userID, YearStart: dateOf(yearStart), YearEnd: dateOf(yearEnd)})
	return int(n), wrap("sum paid leave", err)
}

func (RosterRepo) AnnualLeaveDays(ctx context.Context, tx app.Tx, userID string) (int, error) {
	t, err := pgTx(tx)
	if err != nil {
		return 0, err
	}
	n, err := sqlcgen.New(t).GetAnnualLeaveDays(ctx, sqlcgen.GetAnnualLeaveDaysParams{TenantID: t.tenant, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil // the owner has no contract
	}
	return int(n), wrap("select annual leave", err)
}
