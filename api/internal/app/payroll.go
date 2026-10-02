package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pcaokhai/stayguard/api/internal/domain/finance"
	"github.com/pcaokhai/stayguard/api/internal/domain/roster"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

const (
	payrollUnpaid       = "UNPAID"
	payrollPaid         = "PAID"
	auditPayrollLine    = "payroll.line_updated"
	auditPayrollPaid    = "payroll.paid"
	entityPayroll       = "payroll"
	maxPayrollNoteRunes = 300
)

// PayrollLineView is one person's pay for a month. A paid line is stored in this form and read back as it was paid.
type PayrollLineView struct {
	UserID         string  `json:"userId"`
	Name           string  `json:"name"`
	Position       string  `json:"position"`
	PayType        string  `json:"payType"`
	Rate           int64   `json:"rate"`
	ShiftsWorked   int     `json:"shiftsWorked"`
	StandardShifts int     `json:"standardShifts"`
	LeaveDays      int     `json:"leaveDays"`
	EarnedPay      int64   `json:"earnedPay"`
	Allowance      int64   `json:"allowance"`
	Bonus          int64   `json:"bonus"`
	Deduction      int64   `json:"deduction"`
	Net            int64   `json:"net"`
	Note           *string `json:"note,omitempty"`
	Status         string  `json:"status"`
}

type PayrollView struct {
	Month    string
	Lines    []PayrollLineView
	TotalNet int64
}

type PayrollLineInput struct {
	Bonus, Deduction *int64
	Note             *string
}

// Payroll computes a month's pay from the contracts and the roster and records what the owner adds and pays (SG-1104).
type Payroll struct {
	uow    UnitOfWork
	repo   PayrollRepo
	roster RosterRepo
	ledger ExpenseLedger
	idem   IdempotencyStore
	audit  AuditWriter
	ids    IDGenerator
	clock  Clock
	guard
}

func NewPayroll(uow UnitOfWork, repo PayrollRepo, rosterRepo RosterRepo, ledger ExpenseLedger, levels BuildingLevels,
	idem IdempotencyStore, audit AuditWriter, ids IDGenerator, clock Clock) *Payroll {
	return &Payroll{uow: uow, repo: repo, roster: rosterRepo, ledger: ledger, idem: idem, audit: audit, ids: ids, clock: clock, guard: guard{levels: levels}}
}

func monthError(path string) error {
	return stay.NewValidationError([]stay.FieldError{{Path: path, Code: stay.CodePattern}})
}

// Month returns the payroll of a month (owner only).
func (p *Payroll) Month(ctx context.Context, c Caller, month string) (PayrollView, error) {
	if err := p.checkRole("getPayroll", c); err != nil {
		return PayrollView{}, err
	}
	if _, _, err := finance.MonthDays(month); err != nil {
		return PayrollView{}, monthError("month")
	}
	var out PayrollView
	err := p.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		lines, err := p.compute(ctx, tx, month)
		out = payrollOf(month, lines)
		return err
	})
	return out, err
}

func payrollOf(month string, lines []PayrollLineView) PayrollView {
	v := PayrollView{Month: month, Lines: lines}
	for _, l := range lines {
		v.TotalNet += l.Net
	}
	return v
}

// compute builds every line of a month: paid lines as they were paid, the rest from contract, roster and leave.
func (p *Payroll) compute(ctx context.Context, tx Tx, month string) ([]PayrollLineView, error) {
	first, last, err := finance.MonthDays(month)
	if err != nil {
		return nil, monthError("month")
	}
	staff, err := p.repo.Staff(ctx, tx, month, first, last)
	if err != nil {
		return nil, fmt.Errorf("payroll staff: %w", err)
	}
	stored, err := p.repo.Lines(ctx, tx, month)
	if err != nil {
		return nil, fmt.Errorf("payroll lines: %w", err)
	}
	cells, err := p.roster.Assignments(ctx, tx, first, last)
	if err != nil {
		return nil, fmt.Errorf("assignments: %w", err)
	}
	leave, err := p.roster.Leave(ctx, tx, LeaveFilter{From: &first, To: &last, Standing: true})
	if err != nil {
		return nil, fmt.Errorf("leave: %w", err)
	}
	out := make([]PayrollLineView, 0, len(staff))
	for _, s := range staff {
		st := stored[s.UserID]
		if st.Status == payrollPaid {
			var frozen PayrollLineView
			if err := json.Unmarshal(st.Frozen, &frozen); err != nil {
				return nil, fmt.Errorf("frozen payroll line of %s: %w", s.UserID, err)
			}
			frozen.Status = payrollPaid
			out = append(out, frozen)
			continue
		}
		line, err := lineFor(s, st, first, last, cells, leave)
		if err != nil {
			return nil, err
		}
		out = append(out, line)
	}
	return out, nil
}

func lineFor(s PayrollStaff, st PayrollStored, first, last time.Time, cells []RosterCell, leave []LeaveRow) (PayrollLineView, error) {
	var mine []roster.Leave
	for _, l := range leave {
		if l.UserID == s.UserID && (l.Status == roster.Approved || l.Status == roster.CancelRequested) { // a pending request is not leave yet
			mine = append(mine, roster.Leave{UserID: l.UserID, Shift: l.Shift, Status: l.Status, From: l.From, To: l.To, Cover: l.CoverUserID})
		}
	}
	var w finance.Work
	for _, c := range cells {
		if c.UserID != s.UserID {
			continue
		}
		cell := roster.Cell{Date: c.Date, Shift: c.Shift}
		switch covered, paid := coverage(s.UserID, cell, leave); {
		case !covered:
			w.Shifts++
		case paid:
			w.PaidLeaveShifts++
		}
	}
	earned, err := finance.Earned(finance.Contract{PayType: s.PayType, Rate: s.Rate, Allowance: s.Allowance, StandardShifts: s.StandardShifts}, w)
	if err != nil {
		return PayrollLineView{}, fmt.Errorf("pay of %s: %w", s.UserID, err)
	}
	line := PayrollLineView{UserID: s.UserID, Name: s.Name, Position: s.Position, PayType: s.PayType, Rate: s.Rate, ShiftsWorked: w.Shifts,
		StandardShifts: s.StandardShifts, LeaveDays: leaveDays(mine, first, last), EarnedPay: earned, Allowance: s.Allowance,
		Bonus: st.Bonus, Deduction: st.Deduction, Status: payrollUnpaid}
	line.Net = finance.Net(earned, s.Allowance, st.Bonus, st.Deduction)
	if st.Note != "" {
		n := st.Note
		line.Note = &n
	}
	return line, nil
}

// coverage says whether standing leave of the person covers the shift and whether that leave is paid.
func coverage(userID string, cell roster.Cell, leave []LeaveRow) (covered, paid bool) {
	for _, l := range leave {
		rl := roster.Leave{UserID: l.UserID, Shift: l.Shift, Status: l.Status, From: l.From, To: l.To}
		if l.UserID == userID && rl.Covers(cell) {
			covered = true
			paid = paid || l.Kind == roster.KindPaid
		}
	}
	return covered, paid
}

// leaveDays counts the distinct days of the month a person is on standing leave, whatever its kind.
func leaveDays(leave []roster.Leave, first, last time.Time) int {
	n := 0
	for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
		for _, l := range leave {
			if !d.Before(l.From) && !d.After(l.To) {
				n++
				break
			}
		}
	}
	return n
}

// UpdateLine sets a person's bonus, deduction and note for a month. A paid line cannot change, and a deduction cannot
// exceed the gross pay.
func (p *Payroll) UpdateLine(ctx context.Context, c Caller, month, userID string, in PayrollLineInput) (PayrollLineView, error) {
	if err := p.checkRole("updatePayrollLine", c); err != nil {
		return PayrollLineView{}, err
	}
	if _, _, err := finance.MonthDays(month); err != nil {
		return PayrollLineView{}, monthError("month")
	}
	if err := checkPayrollInput(in); err != nil {
		return PayrollLineView{}, err
	}
	var out PayrollLineView
	err := p.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		status, found, err := p.repo.Lock(ctx, tx, month, userID)
		if err != nil {
			return fmt.Errorf("lock payroll line: %w", err)
		}
		if found && status == payrollPaid {
			return ErrPayrollPaid
		}
		lines, err := p.compute(ctx, tx, month)
		if err != nil {
			return err
		}
		cur, ok := findLine(lines, userID)
		if !ok {
			return ErrNotFound
		}
		bonus, deduction, note := cur.Bonus, cur.Deduction, ""
		if cur.Note != nil {
			note = *cur.Note
		}
		if in.Bonus != nil {
			bonus = *in.Bonus
		}
		if in.Deduction != nil {
			deduction = *in.Deduction
		}
		if in.Note != nil {
			note = strings.TrimSpace(*in.Note)
		}
		if deduction > finance.MaxDeduction(cur.EarnedPay, cur.Allowance, bonus) {
			return stay.NewValidationError([]stay.FieldError{{Path: "deduction", Code: stay.CodeMax}})
		}
		if err := p.repo.Save(ctx, tx, month, userID, bonus, deduction, note); err != nil {
			return fmt.Errorf("save payroll line: %w", err)
		}
		if err := p.auditPayroll(ctx, tx, c, auditPayrollLine, month+"/"+userID, map[string]any{"month": month, "userId": userID, "bonus": bonus, "deduction": deduction}); err != nil {
			return err
		}
		lines, err = p.compute(ctx, tx, month)
		if err != nil {
			return err
		}
		out, _ = findLine(lines, userID)
		return nil
	})
	return out, err
}

func checkPayrollInput(in PayrollLineInput) error {
	var errs []stay.FieldError
	if in.Bonus != nil && (*in.Bonus < 0 || *in.Bonus > finance.MaxExpense) {
		errs = append(errs, stay.FieldError{Path: "bonus", Code: stay.CodeMax})
	}
	if in.Deduction != nil && (*in.Deduction < 0 || *in.Deduction > finance.MaxExpense) {
		errs = append(errs, stay.FieldError{Path: "deduction", Code: stay.CodeMax})
	}
	if in.Note != nil && utf8.RuneCountInString(*in.Note) > maxPayrollNoteRunes {
		errs = append(errs, stay.FieldError{Path: "note", Code: stay.CodeTooLong})
	}
	if len(errs) > 0 {
		return stay.NewValidationError(errs)
	}
	return nil
}

func findLine(lines []PayrollLineView, userID string) (PayrollLineView, bool) {
	for _, l := range lines {
		if l.UserID == userID {
			return l, true
		}
	}
	return PayrollLineView{}, false
}

func (p *Payroll) auditPayroll(ctx context.Context, tx Tx, c Caller, action, id string, after any) error {
	raw, err := json.Marshal(after)
	if err != nil {
		return fmt.Errorf("encode audit: %w", err)
	}
	e := AuditEntry{ID: p.ids.New(auditIDPrefix), ActorID: c.UserID, Action: action, EntityType: entityPayroll, EntityID: id, After: raw}
	if err := p.audit.Append(ctx, tx, e); err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	return nil
}

// MarkPaid pays the chosen lines (all unpaid lines when none are named): each is frozen as paid and posts a STAFF_PAY
// expense for its net pay. A month that has not started cannot be paid, and there must be something to pay.
func (p *Payroll) MarkPaid(ctx context.Context, c Caller, key, month string, userIDs []string) (PayrollView, error) {
	if err := p.checkRole("markPayrollPaid", c); err != nil {
		return PayrollView{}, err
	}
	if err := checkKey(key); err != nil {
		return PayrollView{}, err
	}
	if _, _, err := finance.MonthDays(month); err != nil {
		return PayrollView{}, monthError("month")
	}
	body, _ := json.Marshal(userIDs) // strings: cannot fail
	route := "POST /v1/owner/payroll/" + month + "/mark-paid"
	var out PayrollView
	err := p.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		oc, err := p.idem.Begin(ctx, tx, route, key, RequestHash(body))
		if err != nil {
			return err
		}
		if oc.Replay {
			return json.Unmarshal(oc.Body, &out)
		}
		if out, err = p.pay(ctx, tx, c, month, userIDs); err != nil {
			return err
		}
		raw, err := json.Marshal(out)
		if err != nil {
			return fmt.Errorf("encode response: %w", err)
		}
		return p.idem.Complete(ctx, tx, route, key, statusOK, raw)
	})
	return out, err
}

func (p *Payroll) pay(ctx context.Context, tx Tx, c Caller, month string, userIDs []string) (PayrollView, error) {
	loc, err := loadZone(ctx, tx, p.repo)
	if err != nil {
		return PayrollView{}, err
	}
	now := storedTime(p.clock.Now())
	if month > finance.MonthOf(now.In(loc)) {
		return PayrollView{}, ErrPayrollFuture
	}
	lines, err := p.compute(ctx, tx, month)
	if err != nil {
		return PayrollView{}, err
	}
	want := map[string]bool{}
	for _, id := range userIDs {
		if _, ok := findLine(lines, id); !ok {
			return PayrollView{}, ErrNotFound
		}
		want[id] = true
	}
	paidOn := time.Date(now.In(loc).Year(), now.In(loc).Month(), now.In(loc).Day(), 0, 0, 0, 0, time.UTC)
	paid := 0
	for _, l := range lines {
		if l.Status != payrollUnpaid || (len(want) > 0 && !want[l.UserID]) {
			continue
		}
		if err := p.payLine(ctx, tx, c, month, l, now, paidOn); err != nil {
			return PayrollView{}, err
		}
		paid++
	}
	if paid == 0 {
		return PayrollView{}, ErrPayrollNothing
	}
	if err := p.auditPayroll(ctx, tx, c, auditPayrollPaid, month, map[string]any{"month": month, "lines": paid}); err != nil {
		return PayrollView{}, err
	}
	lines, err = p.compute(ctx, tx, month)
	return payrollOf(month, lines), err
}

func (p *Payroll) payLine(ctx context.Context, tx Tx, c Caller, month string, l PayrollLineView, now, paidOn time.Time) error {
	l.Status = payrollPaid
	frozen, err := json.Marshal(l)
	if err != nil {
		return fmt.Errorf("encode payroll line: %w", err)
	}
	if err := p.repo.Freeze(ctx, tx, month, l.UserID, frozen, now, c.UserID); err != nil {
		return fmt.Errorf("freeze payroll line: %w", err)
	}
	if l.Net == 0 {
		return nil
	}
	e := AutoExpense{ID: p.ids.New(expenseIDPrefix), Source: finance.Payroll, RefID: l.UserID + ":" + month, Category: "STAFF_PAY", Month: month,
		Amount: l.Net, PaidOn: &paidOn, CreatedBy: c.UserID}
	if err := p.ledger.PostAuto(ctx, tx, e); err != nil {
		return fmt.Errorf("post staff pay: %w", err)
	}
	return nil
}
