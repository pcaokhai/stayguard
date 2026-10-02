package httpadapter

import (
	"context"
	"encoding/json"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/pcaokhai/stayguard/api/internal/adapter/http/gen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// FinanceService is what the payroll, expense and report handlers need from the application layer (SG-1104 to SG-1203).
type FinanceService interface {
	Month(ctx context.Context, c app.Caller, month string) (app.PayrollView, error)
	UpdateLine(ctx context.Context, c app.Caller, month, userID string, in app.PayrollLineInput) (app.PayrollLineView, error)
	MarkPaid(ctx context.Context, c app.Caller, key, month string, userIDs []string) (app.PayrollView, error)
	ExpenseMonth(ctx context.Context, c app.Caller, month string) (app.ExpenseMonthView, error)
	CreateExpense(ctx context.Context, c app.Caller, key string, in app.ExpenseInput) (app.ExpenseView, error)
	UpdateExpense(ctx context.Context, c app.Caller, id string, in app.ExpenseInput) (app.ExpenseView, error)
	DeleteExpense(ctx context.Context, c app.Caller, id string) error
	IncomeCost(ctx context.Context, c app.Caller, from, to string) (app.IncomeCostReport, error)
}

// WithFinance adds the payroll, expense and report use cases.
func (s Server) WithFinance(f FinanceService) Server { s.finance = f; return s }

func (s Server) GetPayroll(ctx context.Context, req gen.GetPayrollRequestObject) (gen.GetPayrollResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	v, err := s.finance.Month(ctx, c, req.Month)
	if err != nil {
		return nil, err
	}
	return gen.GetPayroll200JSONResponse(toPayroll(v)), nil
}

func toPayroll(v app.PayrollView) gen.Payroll {
	lines := make([]gen.PayrollLine, len(v.Lines))
	for i, l := range v.Lines {
		lines[i] = toPayrollLine(l)
	}
	return gen.Payroll{Month: v.Month, Lines: lines, TotalNet: v.TotalNet}
}

func toPayrollLine(l app.PayrollLineView) gen.PayrollLine {
	std := l.StandardShifts
	return gen.PayrollLine{UserId: l.UserID, Name: l.Name, Position: gen.Position(l.Position), PayType: gen.PayType(l.PayType), Rate: l.Rate,
		ShiftsWorked: l.ShiftsWorked, StandardShifts: &std, LeaveDays: l.LeaveDays, EarnedPay: l.EarnedPay, Allowance: l.Allowance,
		Bonus: l.Bonus, Deduction: l.Deduction, Net: l.Net, Note: l.Note, Status: gen.PayrollLineStatus(l.Status)}
}

func (s Server) UpdatePayrollLine(ctx context.Context, req gen.UpdatePayrollLineRequestObject) (gen.UpdatePayrollLineResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	b := req.Body
	l, err := s.finance.UpdateLine(ctx, c, req.Month, req.UserId, app.PayrollLineInput{Bonus: b.Bonus, Deduction: b.Deduction, Note: b.Note})
	if err != nil {
		return nil, err
	}
	return gen.UpdatePayrollLine200JSONResponse(toPayrollLine(l)), nil
}

func (s Server) MarkPayrollPaid(ctx context.Context, req gen.MarkPayrollPaidRequestObject) (gen.MarkPayrollPaidResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	var ids []string
	if req.Body != nil && req.Body.UserIds != nil {
		ids = *req.Body.UserIds
	}
	v, err := s.finance.MarkPaid(ctx, c, req.Params.IdempotencyKey.String(), req.Month, ids)
	if err != nil {
		return nil, err
	}
	return gen.MarkPayrollPaid200JSONResponse(toPayroll(v)), nil
}

func toExpense(e app.ExpenseView) gen.Expense {
	rec := e.Recurring
	out := gen.Expense{Id: e.ID, Category: gen.ExpenseCategory(e.Category), Amount: e.Amount, Month: e.Month, Source: gen.ExpenseSource(e.Source),
		Note: nilIfEmpty(e.Note), Recurring: &rec, AttachmentAssetId: nilIfEmpty(e.AttachmentAssetID)}
	if e.PaidOn != nil {
		out.PaidOn = &openapi_types.Date{Time: *e.PaidOn}
	}
	return out
}

func expenseInput(b *gen.CreateExpenseRequest) app.ExpenseInput {
	in := app.ExpenseInput{Category: string(b.Category), Month: b.Month, Amount: b.Amount, Note: deref(b.Note), AttachmentAssetID: deref(b.AttachmentAssetId)}
	if b.Recurring != nil {
		in.Recurring = *b.Recurring
	}
	if b.PaidOn != nil {
		in.PaidOn = &b.PaidOn.Time
	}
	return in
}

func (s Server) GetExpenseMonth(ctx context.Context, req gen.GetExpenseMonthRequestObject) (gen.GetExpenseMonthResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	v, err := s.finance.ExpenseMonth(ctx, c, req.Params.Month)
	if err != nil {
		return nil, err
	}
	// The generated categories element is an anonymous struct, so the answer goes through JSON.
	type category struct {
		Category string `json:"category"`
		Amount   int64  `json:"amount"`
		Source   string `json:"source"`
	}
	cats := make([]category, len(v.Categories))
	for i, s := range v.Categories {
		cats[i] = category{s.Key, s.Amount, s.Source}
	}
	items := make([]gen.Expense, len(v.Items))
	for i, e := range v.Items {
		items[i] = toExpense(e)
	}
	raw, err := json.Marshal(map[string]any{"month": v.Month, "categories": cats, "items": items, "total": v.Total, "revenue": v.Revenue})
	if err != nil {
		return nil, err
	}
	var out gen.ExpenseMonth
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return gen.GetExpenseMonth200JSONResponse(out), nil
}

func (s Server) CreateExpense(ctx context.Context, req gen.CreateExpenseRequestObject) (gen.CreateExpenseResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	e, err := s.finance.CreateExpense(ctx, c, req.Params.IdempotencyKey.String(), expenseInput(req.Body))
	if err != nil {
		return nil, err
	}
	return gen.CreateExpense201JSONResponse(toExpense(e)), nil
}

func (s Server) UpdateExpense(ctx context.Context, req gen.UpdateExpenseRequestObject) (gen.UpdateExpenseResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	e, err := s.finance.UpdateExpense(ctx, c, req.ExpenseId, expenseInput(req.Body))
	if err != nil {
		return nil, err
	}
	return gen.UpdateExpense200JSONResponse(toExpense(e)), nil
}

func (s Server) DeleteExpense(ctx context.Context, req gen.DeleteExpenseRequestObject) (gen.DeleteExpenseResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	if err := s.finance.DeleteExpense(ctx, c, req.ExpenseId); err != nil {
		return nil, err
	}
	return gen.DeleteExpense204Response{}, nil
}

func (s Server) GetIncomeCostReport(ctx context.Context, req gen.GetIncomeCostReportRequestObject) (gen.GetIncomeCostReportResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	r, err := s.finance.IncomeCost(ctx, c, req.Params.From, req.Params.To)
	if err != nil {
		return nil, err
	}
	shares := func(in []app.Share) []gen.AmountShare {
		out := make([]gen.AmountShare, len(in))
		for i, s := range in {
			out[i] = gen.AmountShare{Key: s.Key, Amount: s.Amount}
		}
		return out
	}
	type month struct {
		Month    string `json:"month"`
		Revenue  int64  `json:"revenue"`
		Expenses int64  `json:"expenses"`
	}
	months := make([]month, len(r.Months))
	for i, m := range r.Months {
		months[i] = month{m.Month, m.Revenue, m.Expenses}
	}
	raw, err := json.Marshal(map[string]any{"from": r.From, "to": r.To, "revenue": r.Revenue, "expenses": r.Expenses, "profit": r.Profit,
		"marginPct": r.MarginPct, "occupancyPct": r.OccupancyPct, "months": months, "expensesByCategory": shares(r.ExpensesByCategory),
		"revenueByRentalType": shares(r.RevenueByRentalType), "revenueByBuilding": shares(r.RevenueByBuilding), "revenueByMethod": shares(r.RevenueByMethod)})
	if err != nil {
		return nil, err
	}
	var out gen.IncomeCostReport
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return gen.GetIncomeCostReport200JSONResponse(out), nil
}
