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

// FinanceRepo implements app.ExpenseRepo, app.ExpenseLedger, app.ReportRepo and app.PayrollRepo. The tenant always
// comes from the Tx.
type FinanceRepo struct{}

var (
	_ app.ExpenseRepo   = FinanceRepo{}
	_ app.ExpenseLedger = FinanceRepo{}
	_ app.ReportRepo    = FinanceRepo{}
	_ app.PayrollRepo   = FinanceRepo{}
)

func (FinanceRepo) Timezone(ctx context.Context, tx app.Tx) (string, error) {
	return RoomRepo{}.Timezone(ctx, tx)
}

func datePtr(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	t := d.Time
	return &t
}

func textOf(t pgtype.Text) string { return t.String }

func toExpenseRow(id, month, category string, amount int64, paidOn pgtype.Date, note pgtype.Text, recurring bool, source string, asset pgtype.Text) app.ExpenseRow {
	return app.ExpenseRow{ID: id, Month: month, Category: category, Amount: amount, PaidOn: datePtr(paidOn), Note: textOf(note),
		Recurring: recurring, Source: source, AttachmentAssetID: textOf(asset)}
}

func (FinanceRepo) Insert(ctx context.Context, tx app.Tx, e app.NewExpense) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return writeFailure("insert expense", sqlcgen.New(t).InsertExpense(ctx, sqlcgen.InsertExpenseParams{ID: e.ID, TenantID: t.tenant, Month: e.Month,
		Category: e.Category, Amount: e.Amount, PaidOn: optDate(e.PaidOn), Note: optText(e.Note), Recurring: e.Recurring, Source: e.Source,
		AttachmentAssetID: optText(e.AttachmentAssetID), CreatedBy: optText(e.CreatedBy)}))
}

func (FinanceRepo) PostAuto(ctx context.Context, tx app.Tx, e app.AutoExpense) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	_, err = sqlcgen.New(t).InsertAutoExpense(ctx, sqlcgen.InsertAutoExpenseParams{ID: e.ID, TenantID: t.tenant, Month: e.Month, Category: e.Category,
		Amount: e.Amount, PaidOn: optDate(e.PaidOn), Note: optText(e.Note), Source: e.Source, RefID: optText(e.RefID), CreatedBy: optText(e.CreatedBy)})
	return writeFailure("post expense", err)
}

func (FinanceRepo) ByID(ctx context.Context, tx app.Tx, id string) (app.ExpenseRow, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.ExpenseRow{}, false, err
	}
	r, err := sqlcgen.New(t).GetExpense(ctx, sqlcgen.GetExpenseParams{TenantID: t.tenant, ExpenseID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ExpenseRow{}, false, nil
	}
	if err != nil {
		return app.ExpenseRow{}, false, wrap("select expense", err)
	}
	return toExpenseRow(r.ID, r.Month, r.Category, r.Amount, r.PaidOn, r.Note, r.Recurring, r.Source, r.AttachmentAssetID), true, nil
}

func (FinanceRepo) OfMonth(ctx context.Context, tx app.Tx, month string) ([]app.ExpenseRow, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListExpensesOfMonth(ctx, sqlcgen.ListExpensesOfMonthParams{TenantID: t.tenant, Month: month})
	if err != nil {
		return nil, wrap("list expenses", err)
	}
	out := make([]app.ExpenseRow, len(rows))
	for i, r := range rows {
		out[i] = toExpenseRow(r.ID, r.Month, r.Category, r.Amount, r.PaidOn, r.Note, r.Recurring, r.Source, r.AttachmentAssetID)
	}
	return out, nil
}

func (FinanceRepo) Update(ctx context.Context, tx app.Tx, e app.ExpenseRow) (bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return false, err
	}
	n, err := sqlcgen.New(t).UpdateExpense(ctx, sqlcgen.UpdateExpenseParams{TenantID: t.tenant, ExpenseID: e.ID, Month: e.Month, Category: e.Category,
		Amount: e.Amount, PaidOn: optDate(e.PaidOn), Note: optText(e.Note), Recurring: e.Recurring, AttachmentAssetID: optText(e.AttachmentAssetID)})
	return n == 1, wrap("update expense", err)
}

func (FinanceRepo) Delete(ctx context.Context, tx app.Tx, id string) (bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return false, err
	}
	n, err := sqlcgen.New(t).DeleteExpense(ctx, sqlcgen.DeleteExpenseParams{TenantID: t.tenant, ExpenseID: id})
	return n == 1, wrap("delete expense", err)
}

func (FinanceRepo) Summary(ctx context.Context, tx app.Tx, month string) ([]app.Share, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ExpenseSummaryOfMonth(ctx, sqlcgen.ExpenseSummaryOfMonthParams{TenantID: t.tenant, Month: month})
	if err != nil {
		return nil, wrap("expense summary", err)
	}
	out := make([]app.Share, len(rows))
	for i, r := range rows {
		out[i] = app.Share{Key: r.Category, Source: r.Source, Amount: r.Total}
	}
	return out, nil
}

func (FinanceRepo) EarliestRecurring(ctx context.Context, tx app.Tx) (string, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return "", false, err
	}
	m, err := sqlcgen.New(t).EarliestRecurringMonth(ctx, t.tenant)
	return m, m != "", wrap("earliest recurring month", err)
}

func (FinanceRepo) RecurringRan(ctx context.Context, tx app.Tx, month string) (bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return false, err
	}
	n, err := sqlcgen.New(t).RecurringRan(ctx, sqlcgen.RecurringRanParams{TenantID: t.tenant, Month: month})
	return n > 0, wrap("recurring ran", err)
}

func (FinanceRepo) CopyRecurring(ctx context.Context, tx app.Tx, fromMonth, toMonth string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	_, err = sqlcgen.New(t).CopyRecurringExpenses(ctx, sqlcgen.CopyRecurringExpensesParams{TenantID: t.tenant, FromMonth: fromMonth, ToMonth: toMonth})
	return wrap("copy recurring expenses", err)
}

func (FinanceRepo) MarkRecurringRan(ctx context.Context, tx app.Tx, month string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("mark recurring ran", sqlcgen.New(t).MarkRecurringRan(ctx, sqlcgen.MarkRecurringRanParams{TenantID: t.tenant, Month: month}))
}

func (FinanceRepo) Revenue(ctx context.Context, tx app.Tx, from, to time.Time) (int64, error) {
	t, err := pgTx(tx)
	if err != nil {
		return 0, err
	}
	n, err := sqlcgen.New(t).PaidInvoiceTotals(ctx, sqlcgen.PaidInvoiceTotalsParams{TenantID: t.tenant, FromAt: ts(from), ToAt: ts(to)})
	return n, wrap("paid invoice totals", err)
}

func (FinanceRepo) RevenueByMonth(ctx context.Context, tx app.Tx, from, to time.Time, zone string) (map[string]int64, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).RevenueByMonth(ctx, sqlcgen.RevenueByMonthParams{TenantID: t.tenant, FromAt: ts(from), ToAt: ts(to), Zone: zone})
	if err != nil {
		return nil, wrap("revenue by month", err)
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.Month] = r.Revenue
	}
	return out, nil
}

func (FinanceRepo) RevenueByRentalType(ctx context.Context, tx app.Tx, from, to time.Time) ([]app.Share, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).RevenueByRentalType(ctx, sqlcgen.RevenueByRentalTypeParams{TenantID: t.tenant, FromAt: ts(from), ToAt: ts(to)})
	if err != nil {
		return nil, wrap("revenue by rental type", err)
	}
	out := make([]app.Share, len(rows))
	for i, r := range rows {
		out[i] = app.Share{Key: r.Key, Amount: r.Revenue}
	}
	return out, nil
}

func (FinanceRepo) RevenueByBuilding(ctx context.Context, tx app.Tx, from, to time.Time) ([]app.Share, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).RevenueByBuilding(ctx, sqlcgen.RevenueByBuildingParams{TenantID: t.tenant, FromAt: ts(from), ToAt: ts(to)})
	if err != nil {
		return nil, wrap("revenue by building", err)
	}
	out := make([]app.Share, len(rows))
	for i, r := range rows {
		out[i] = app.Share{Key: r.Key, Amount: r.Revenue}
	}
	return out, nil
}

func (FinanceRepo) RevenueDeposit(ctx context.Context, tx app.Tx, from, to time.Time) (int64, error) {
	t, err := pgTx(tx)
	if err != nil {
		return 0, err
	}
	n, err := sqlcgen.New(t).RevenueDepositPart(ctx, sqlcgen.RevenueDepositPartParams{TenantID: t.tenant, FromAt: ts(from), ToAt: ts(to)})
	return n, wrap("revenue deposit part", err)
}

func (FinanceRepo) RevenueBalanceByMethod(ctx context.Context, tx app.Tx, from, to time.Time) ([]app.Share, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).RevenueBalanceByMethod(ctx, sqlcgen.RevenueBalanceByMethodParams{TenantID: t.tenant, FromAt: ts(from), ToAt: ts(to)})
	if err != nil {
		return nil, wrap("revenue by method", err)
	}
	out := make([]app.Share, len(rows))
	for i, r := range rows {
		out[i] = app.Share{Key: r.Key, Amount: r.Revenue}
	}
	return out, nil
}

func (FinanceRepo) ExpenseByMonth(ctx context.Context, tx app.Tx, fromMonth, toMonth string) (map[string]int64, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ExpenseTotalsByMonth(ctx, sqlcgen.ExpenseTotalsByMonthParams{TenantID: t.tenant, FromMonth: fromMonth, ToMonth: toMonth})
	if err != nil {
		return nil, wrap("expense totals by month", err)
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.Month] = r.Total
	}
	return out, nil
}

func (FinanceRepo) ExpenseByCategory(ctx context.Context, tx app.Tx, fromMonth, toMonth string) ([]app.Share, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ExpenseTotalsByCategory(ctx, sqlcgen.ExpenseTotalsByCategoryParams{TenantID: t.tenant, FromMonth: fromMonth, ToMonth: toMonth})
	if err != nil {
		return nil, wrap("expense totals by category", err)
	}
	out := make([]app.Share, len(rows))
	for i, r := range rows {
		out[i] = app.Share{Key: r.Category, Amount: r.Total}
	}
	return out, nil
}

func (FinanceRepo) OccupiedRoomDays(ctx context.Context, tx app.Tx, zone string, fromDay, toDay time.Time) (int64, error) {
	t, err := pgTx(tx)
	if err != nil {
		return 0, err
	}
	n, err := sqlcgen.New(t).OccupiedRoomDays(ctx, sqlcgen.OccupiedRoomDaysParams{TenantID: t.tenant, Zone: zone, FromDay: dateOf(fromDay), ToDay: dateOf(toDay)})
	return n, wrap("occupied room days", err)
}

func (FinanceRepo) RoomCount(ctx context.Context, tx app.Tx) (int64, error) {
	t, err := pgTx(tx)
	if err != nil {
		return 0, err
	}
	n, err := sqlcgen.New(t).CountRooms(ctx, t.tenant)
	return n, wrap("count rooms", err)
}

func (FinanceRepo) Staff(ctx context.Context, tx app.Tx, month string, first, last time.Time) ([]app.PayrollStaff, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListPayrollStaff(ctx, sqlcgen.ListPayrollStaffParams{TenantID: t.tenant, Month: month, MonthFirst: dateOf(first), MonthLast: dateOf(last)})
	if err != nil {
		return nil, wrap("list payroll staff", err)
	}
	out := make([]app.PayrollStaff, len(rows))
	for i, r := range rows {
		out[i] = app.PayrollStaff{UserID: r.ID, Name: r.Name, Position: r.Position, Removed: r.Status == "REMOVED", PayType: r.PayType,
			Rate: r.Rate, Allowance: r.FixedAllowance, StandardShifts: int(r.StandardShifts)}
	}
	return out, nil
}

func (FinanceRepo) Lines(ctx context.Context, tx app.Tx, month string) (map[string]app.PayrollStored, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListPayrollLines(ctx, sqlcgen.ListPayrollLinesParams{TenantID: t.tenant, Month: month})
	if err != nil {
		return nil, wrap("list payroll lines", err)
	}
	out := make(map[string]app.PayrollStored, len(rows))
	for _, r := range rows {
		out[r.UserID] = app.PayrollStored{Bonus: r.Bonus, Deduction: r.Deduction, Note: r.Note, Status: r.Status, Frozen: r.Frozen, PaidAt: timePtr(r.PaidAt)}
	}
	return out, nil
}

func (FinanceRepo) Save(ctx context.Context, tx app.Tx, month, userID string, bonus, deduction int64, note string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return writeFailure("save payroll line", sqlcgen.New(t).UpsertPayrollLine(ctx, sqlcgen.UpsertPayrollLineParams{TenantID: t.tenant, UserID: userID,
		Month: month, Bonus: bonus, Deduction: deduction, Note: optText(note)}))
}

func (FinanceRepo) Lock(ctx context.Context, tx app.Tx, month, userID string) (string, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return "", false, err
	}
	status, err := sqlcgen.New(t).LockPayrollLine(ctx, sqlcgen.LockPayrollLineParams{TenantID: t.tenant, UserID: userID, Month: month})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	return status, err == nil, wrap("lock payroll line", err)
}

func (FinanceRepo) Freeze(ctx context.Context, tx app.Tx, month, userID string, frozen []byte, at time.Time, by string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	n, err := sqlcgen.New(t).FreezePayrollLine(ctx, sqlcgen.FreezePayrollLineParams{TenantID: t.tenant, UserID: userID, Month: month, Frozen: frozen,
		PaidAt: ts(at), PaidBy: optText(by)})
	if err != nil {
		return writeFailure("freeze payroll line", err)
	}
	if n != 1 {
		return app.ErrPayrollPaid
	}
	return nil
}
