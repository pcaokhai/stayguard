package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/finance"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

const (
	expenseIDPrefix = "ex"
	auditExpenseNew = "expense.created"
	auditExpenseUpd = "expense.updated"
	auditExpenseDel = "expense.deleted"
	entityExpense   = "expense"
	// maxRecurringChain bounds how many months one catch-up copies.
	maxRecurringChain = 60
)

// ExpenseView is an expense line as the contract shows it.
type ExpenseView struct {
	ID, Month, Category, Source, Note, AttachmentAssetID string
	Amount                                               int64
	PaidOn                                               *time.Time
	Recurring                                            bool
}

type ExpenseInput struct {
	Category, Month, Note, AttachmentAssetID string
	Amount                                   int64
	PaidOn                                   *time.Time // a calendar day
	Recurring                                bool
}

type ExpenseMonthView struct {
	Month      string
	Categories []Share // key is the category, Source the source
	Items      []ExpenseView
	Total      int64
	Revenue    int64
}

// Expenses holds the expense use cases (SG-1202) and the monthly copy of recurring expenses.
type Expenses struct {
	uow   UnitOfWork
	repo  ExpenseRepo
	idem  IdempotencyStore
	audit AuditWriter
	ids   IDGenerator
	clock Clock
	guard
}

func NewExpenses(uow UnitOfWork, repo ExpenseRepo, levels BuildingLevels, idem IdempotencyStore, audit AuditWriter, ids IDGenerator, clock Clock) *Expenses {
	return &Expenses{uow: uow, repo: repo, idem: idem, audit: audit, ids: ids, clock: clock, guard: guard{levels: levels}}
}

func expenseViewOf(r ExpenseRow) ExpenseView { return ExpenseView(r) }

// currentMonth is the tenant-local month of the clock.
func currentMonth(ctx context.Context, tx Tx, z zoneSource, clk Clock) (string, error) {
	loc, err := loadZone(ctx, tx, z)
	if err != nil {
		return "", err
	}
	return finance.MonthOf(clk.Now().In(loc)), nil
}

// EnsureRecurring copies the recurring lines of each month into the next, up to the current month, once per month. It
// runs the first time a month is read, so a month nobody opens is copied when somebody finally does; a line deleted
// from a copied month stays deleted because the month is marked as done.
func EnsureRecurring(ctx context.Context, tx Tx, repo ExpenseRepo, upTo string) error {
	earliest, ok, err := repo.EarliestRecurring(ctx, tx)
	if err != nil {
		return fmt.Errorf("earliest recurring: %w", err)
	}
	if !ok || earliest >= upTo {
		return nil
	}
	steps := 0
	for m := finance.NextMonth(earliest); m <= upTo; m = finance.NextMonth(m) {
		if steps++; steps > maxRecurringChain {
			return fmt.Errorf("recurring expenses start %s: more than %d months to copy", earliest, maxRecurringChain)
		}
		ran, err := repo.RecurringRan(ctx, tx, m)
		if err != nil {
			return fmt.Errorf("recurring ran: %w", err)
		}
		if ran {
			continue
		}
		if err := repo.CopyRecurring(ctx, tx, finance.PrevMonth(m), m); err != nil {
			return fmt.Errorf("copy recurring: %w", err)
		}
		if err := repo.MarkRecurringRan(ctx, tx, m); err != nil {
			return fmt.Errorf("mark recurring ran: %w", err)
		}
	}
	return nil
}

// ExpenseMonth returns the expenses of a month by category with every line, and the month's revenue.
func (e *Expenses) ExpenseMonth(ctx context.Context, c Caller, month string) (ExpenseMonthView, error) {
	if err := e.checkRole("getExpenseMonth", c); err != nil {
		return ExpenseMonthView{}, err
	}
	first, _, err := finance.MonthDays(month)
	if err != nil {
		return ExpenseMonthView{}, monthError("month")
	}
	var out ExpenseMonthView
	err = e.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		cur, err := currentMonth(ctx, tx, e.repo, e.clock)
		if err != nil {
			return err
		}
		if err := EnsureRecurring(ctx, tx, e.repo, min(month, cur)); err != nil {
			return err
		}
		loc, err := loadZone(ctx, tx, e.repo)
		if err != nil {
			return err
		}
		rows, err := e.repo.OfMonth(ctx, tx, month)
		if err != nil {
			return fmt.Errorf("expenses: %w", err)
		}
		shares, err := e.repo.Summary(ctx, tx, month)
		if err != nil {
			return fmt.Errorf("expense summary: %w", err)
		}
		from := time.Date(first.Year(), first.Month(), 1, 0, 0, 0, 0, loc)
		rev, err := e.repo.Revenue(ctx, tx, from, from.AddDate(0, 1, 0))
		if err != nil {
			return fmt.Errorf("revenue: %w", err)
		}
		out = ExpenseMonthView{Month: month, Categories: shares, Revenue: rev, Items: make([]ExpenseView, len(rows))}
		for i, r := range rows {
			out.Items[i] = expenseViewOf(r)
			out.Total += r.Amount
		}
		return nil
	})
	return out, err
}

func checkExpense(in ExpenseInput) error {
	paidOn := ""
	if in.PaidOn != nil {
		paidOn = calendarDay(*in.PaidOn).Format(time.DateOnly)
	}
	errs := finance.CheckExpense(in.Category, in.Amount, in.Month, paidOn, in.Note)
	if len(errs) == 0 {
		return nil
	}
	out := make([]stay.FieldError, len(errs))
	for i, f := range errs {
		out[i] = stay.FieldError{Path: f.Path, Code: f.Code}
	}
	return stay.NewValidationError(out)
}

func (in ExpenseInput) row(id, source string) ExpenseRow {
	r := ExpenseRow{ID: id, Month: in.Month, Category: in.Category, Amount: in.Amount, Note: strings.TrimSpace(in.Note), Recurring: in.Recurring,
		Source: source, AttachmentAssetID: in.AttachmentAssetID}
	if in.PaidOn != nil {
		d := calendarDay(*in.PaidOn)
		r.PaidOn = &d
	}
	return r
}

// CreateExpense adds an expense a person wrote; it is RECURRING when it repeats every month.
func (e *Expenses) CreateExpense(ctx context.Context, c Caller, key string, in ExpenseInput) (ExpenseView, error) {
	if err := e.checkRole("createExpense", c); err != nil {
		return ExpenseView{}, err
	}
	if err := checkKey(key); err != nil {
		return ExpenseView{}, err
	}
	if err := checkExpense(in); err != nil {
		return ExpenseView{}, err
	}
	body, _ := json.Marshal(in) // plain values: cannot fail
	var out ExpenseView
	err := e.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		const route = "POST /v1/owner/expenses"
		oc, err := e.idem.Begin(ctx, tx, route, key, RequestHash(body))
		if err != nil {
			return err
		}
		if oc.Replay {
			return json.Unmarshal(oc.Body, &out)
		}
		source := finance.Manual
		if in.Recurring {
			source = finance.Recurring
		}
		row := in.row(e.ids.New(expenseIDPrefix), source)
		if err := e.repo.Insert(ctx, tx, NewExpense{ExpenseRow: row, CreatedBy: c.UserID}); err != nil {
			return fmt.Errorf("insert expense: %w", err)
		}
		if err := e.auditExpense(ctx, tx, c, auditExpenseNew, row.ID, map[string]any{"expenseId": row.ID, "category": row.Category, "month": row.Month, "amount": row.Amount}); err != nil {
			return err
		}
		out = expenseViewOf(row)
		raw, err := json.Marshal(out)
		if err != nil {
			return fmt.Errorf("encode response: %w", err)
		}
		return e.idem.Complete(ctx, tx, route, key, statusCreated, raw)
	})
	return out, err
}

// UpdateExpense replaces an expense a person wrote. The system's own lines (payroll, maintenance, stock) answer 409.
func (e *Expenses) UpdateExpense(ctx context.Context, c Caller, id string, in ExpenseInput) (ExpenseView, error) {
	if err := e.checkRole("updateExpense", c); err != nil {
		return ExpenseView{}, err
	}
	if err := checkExpense(in); err != nil {
		return ExpenseView{}, err
	}
	var out ExpenseView
	err := e.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		cur, err := e.editable(ctx, tx, id)
		if err != nil {
			return err
		}
		row := in.row(id, cur.Source)
		if ok, err := e.repo.Update(ctx, tx, row); err != nil {
			return fmt.Errorf("update expense: %w", err)
		} else if !ok {
			return ErrExpenseAutomatic
		}
		if err := e.auditExpense(ctx, tx, c, auditExpenseUpd, id, map[string]any{"expenseId": id, "category": row.Category, "month": row.Month, "amount": row.Amount}); err != nil {
			return err
		}
		out = expenseViewOf(row)
		return nil
	})
	return out, err
}

// DeleteExpense removes an expense a person wrote.
func (e *Expenses) DeleteExpense(ctx context.Context, c Caller, id string) error {
	if err := e.checkRole("deleteExpense", c); err != nil {
		return err
	}
	return e.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		if _, err := e.editable(ctx, tx, id); err != nil {
			return err
		}
		if ok, err := e.repo.Delete(ctx, tx, id); err != nil {
			return fmt.Errorf("delete expense: %w", err)
		} else if !ok {
			return ErrExpenseAutomatic
		}
		return e.auditExpense(ctx, tx, c, auditExpenseDel, id, map[string]any{"expenseId": id})
	})
}

// editable finds a line (404 when unknown) and refuses the system's own (409).
func (e *Expenses) editable(ctx context.Context, tx Tx, id string) (ExpenseRow, error) {
	cur, ok, err := e.repo.ByID(ctx, tx, id)
	if err != nil {
		return ExpenseRow{}, fmt.Errorf("expense: %w", err)
	}
	if !ok {
		return ExpenseRow{}, ErrNotFound
	}
	if finance.IsAutomatic(cur.Source) {
		return ExpenseRow{}, ErrExpenseAutomatic
	}
	return cur, nil
}

func (e *Expenses) auditExpense(ctx context.Context, tx Tx, c Caller, action, id string, after any) error {
	raw, err := json.Marshal(after)
	if err != nil {
		return fmt.Errorf("encode audit: %w", err)
	}
	a := AuditEntry{ID: e.ids.New(auditIDPrefix), ActorID: c.UserID, Action: action, EntityType: entityExpense, EntityID: id, After: raw}
	if err := e.audit.Append(ctx, tx, a); err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	return nil
}
