package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/shift"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

const (
	shiftIDPrefix   = "sh"
	cashIDPrefix    = "ce"
	shiftOpen       = "OPEN"
	shiftClosed     = "CLOSED"
	routePayout     = "POST /v1/shifts/current/payouts/"
	routeClose      = "POST /v1/shifts/current/close/"
	auditPayout     = "shift.payout"
	auditShiftClose = "shift.closed"
	entityShift     = "shift"
	maxPayoutNote   = 200
	maxCloseReason  = 500
	closedPageSize  = 50
)

// ShiftView is a shift with the cash the system expects. CashIn and CashOut come from the ledger.
type ShiftView struct {
	ID, UserID, UserName, Status string
	OpenedAt                     time.Time
	ClosedAt                     *time.Time
	OpeningFloat, CashIn         int64
	CashOut, ExpectedCash        int64
	TransfersReceived            int64
	BuildingIDs                  []string
}

type CashPaymentRow struct {
	RoomCode, RentalType string
	At                   time.Time
	Amount               int64
}

type ShiftReview struct {
	Shift            ShiftView
	CountedCash      int64
	Difference       int64
	Reason           *string
	ReasonRecordedAt *time.Time
	CashPayments     []CashPaymentRow
	ShiftsWithDiff   int64
	TotalShort       int64
}

type PayoutInput struct {
	Amount      int64
	Description string
}

type CloseShiftInput struct {
	Counts           []shift.Count
	FloatLeft        int64
	Reason           string
	HandoverToUserID string
}

type ClosedShiftPage struct {
	Items      []ClosedShiftRow
	NextCursor string
}

type ClosedShiftsQuery struct {
	Month, UserID, Cursor string
	OnlyDifferences       bool
}

// Shifts holds the front-desk shift use cases (SG-503) and is the cash ledger the money commands write to.
type Shifts struct {
	uow    UnitOfWork
	repo   ShiftRepo
	idem   IdempotencyStore
	audit  AuditWriter
	alerts AlertWriter
	ids    IDGenerator
	clock  Clock
	guard
}

var _ CashLedger = (*Shifts)(nil)

func NewShifts(uow UnitOfWork, repo ShiftRepo, levels BuildingLevels, idem IdempotencyStore, audit AuditWriter,
	alerts AlertWriter, ids IDGenerator, clock Clock) *Shifts {
	return &Shifts{uow: uow, repo: repo, idem: idem, audit: audit, alerts: alerts, ids: ids, clock: clock, guard: guard{levels: levels}}
}

func hasDrawer(c Caller) bool { return c.Role == access.RoleReceptionist }

// Record puts a cash movement on the drawer of the caller's open shift, opening the shift first when this is the
// caller's first cash action. It runs in the transaction of the command that moved the cash.
//
// Cash taken by the owner or a manager has no drawer, so it opens no shift and has no ledger line. It is not lost: the
// payment is stored, the transactions list shows it as CASH with no shift id, and revenue by method counts it
// (TestOwnerCash_NoShiftButNeverDropped_FU1).
func (s *Shifts) Record(ctx context.Context, tx Tx, c Caller, e CashRecord) error {
	if !hasDrawer(c) || e.Amount <= 0 {
		return nil
	}
	now := storedTime(s.clock.Now())
	sh, err := s.ensureOpen(ctx, tx, c, now)
	if err != nil {
		return err
	}
	return s.addEntry(ctx, tx, c, sh.ID, e.Kind, e.Amount, e.StayID, e.PaymentID, "", now)
}

func (s *Shifts) addEntry(ctx context.Context, tx Tx, c Caller, shiftID, kind string, amount int64, stayID, paymentID, desc string, at time.Time) error {
	e := CashEntry{ID: s.ids.New(cashIDPrefix), ShiftID: shiftID, Kind: kind, Amount: amount, StayID: stayID, PaymentID: paymentID,
		Description: desc, CreatedBy: c.UserID, CreatedAt: at}
	if err := s.repo.AddEntry(ctx, tx, e); err != nil {
		return fmt.Errorf("cash entry: %w", err)
	}
	return nil
}

// ensureOpen returns the caller's open shift, locked; it opens one with the float the last shift left.
func (s *Shifts) ensureOpen(ctx context.Context, tx Tx, c Caller, now time.Time) (ShiftRecord, error) {
	sh, ok, err := s.repo.LockOpen(ctx, tx, c.UserID)
	if err != nil {
		return ShiftRecord{}, fmt.Errorf("open shift: %w", err)
	}
	if ok {
		return sh, nil
	}
	loc, err := loadZone(ctx, tx, s.repo)
	if err != nil {
		return ShiftRecord{}, err
	}
	float, err := s.repo.LastFloatLeft(ctx, tx)
	if err != nil {
		return ShiftRecord{}, fmt.Errorf("last float: %w", err)
	}
	code, err := s.shiftCode(ctx, tx, c.UserID, now.In(loc))
	if err != nil {
		return ShiftRecord{}, err
	}
	n := NewShift{ID: s.ids.New(shiftIDPrefix), UserID: c.UserID, Code: code, OpenedAt: now, OpeningFloat: float}
	if _, err := s.repo.Open(ctx, tx, n); err != nil {
		return ShiftRecord{}, fmt.Errorf("open shift: %w", err)
	}
	if sh, ok, err = s.repo.LockOpen(ctx, tx, c.UserID); err != nil || !ok {
		return ShiftRecord{}, fmt.Errorf("open shift after insert (found %v): %w", ok, err)
	}
	return sh, nil
}

// shiftCode is the roster shift the person is on now: the one matching the hour when they are rostered for it, else
// the first they are rostered for that day, else the one the hour says (a person nobody rostered still opens a shift).
func (s *Shifts) shiftCode(ctx context.Context, tx Tx, userID string, at time.Time) (string, error) {
	hour := shift.CodeFor(at)
	day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
	rostered, err := s.repo.Scheduled(ctx, tx, userID, day)
	if err != nil {
		return "", fmt.Errorf("scheduled shifts: %w", err)
	}
	for _, code := range rostered {
		if code == hour {
			return hour, nil
		}
	}
	if len(rostered) > 0 {
		return rostered[0], nil
	}
	return hour, nil
}

// maxEditable is the maximum building level of the caller (the rule for shift operations).
func (s *Shifts) maxEditable(ctx context.Context, tx Tx, c Caller) (access.Level, []string, error) {
	ids, err := s.repo.BuildingIDs(ctx, tx)
	if err != nil {
		return access.NONE, nil, fmt.Errorf("buildings: %w", err)
	}
	lv, err := s.levels.Levels(ctx, c, ids)
	if err != nil {
		return access.NONE, nil, fmt.Errorf("building levels: %w", err)
	}
	max, editable := access.NONE, []string{}
	for _, id := range ids {
		if lv[id] > max {
			max = lv[id]
		}
		if lv[id] >= access.EDIT {
			editable = append(editable, id)
		}
	}
	return max, editable, nil
}

func (s *Shifts) check(ctx context.Context, tx Tx, op string, c Caller) ([]string, error) {
	max, buildings, err := s.maxEditable(ctx, tx, c)
	if err != nil {
		return nil, err
	}
	if err := s.authz.Check(op, c.Role, max); err != nil {
		return nil, err
	}
	return buildings, nil
}

// view totals the ledger of sh; a closed shift reads the same ledger, which is locked.
func (s *Shifts) view(ctx context.Context, tx Tx, sh ShiftRecord, now time.Time, buildings []string) (ShiftView, error) {
	in, out, err := s.repo.Cash(ctx, tx, sh.ID)
	if err != nil {
		return ShiftView{}, fmt.Errorf("shift cash: %w", err)
	}
	expected, err := shift.ExpectedCash(sh.OpeningFloat, in, out)
	if err != nil {
		return ShiftView{}, fmt.Errorf("expected cash: %w", err)
	}
	until := now
	if sh.ClosedAt != nil {
		until = *sh.ClosedAt
	}
	transfers, err := s.repo.Transfers(ctx, tx, sh.OpenedAt, until)
	if err != nil {
		return ShiftView{}, fmt.Errorf("transfers: %w", err)
	}
	if buildings == nil {
		buildings = []string{}
	}
	return ShiftView{ID: sh.ID, UserID: sh.UserID, UserName: sh.UserName, Status: sh.Status, OpenedAt: sh.OpenedAt.UTC(),
		ClosedAt: utcPtr(sh.ClosedAt), OpeningFloat: sh.OpeningFloat, CashIn: in, CashOut: out, ExpectedCash: expected,
		TransfersReceived: transfers, BuildingIDs: buildings}, nil
}

// Current returns the caller's open shift, or ErrNotFound when none was opened yet (no cash action so far).
func (s *Shifts) Current(ctx context.Context, c Caller) (ShiftView, error) {
	const op = "getCurrentShift"
	if err := s.checkRole(op, c); err != nil {
		return ShiftView{}, err
	}
	var out ShiftView
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		buildings, err := s.check(ctx, tx, op, c)
		if err != nil {
			return err
		}
		sh, ok, err := s.repo.LockOpen(ctx, tx, c.UserID)
		if err != nil {
			return fmt.Errorf("open shift: %w", err)
		}
		if !ok || !hasDrawer(c) {
			return ErrNotFound
		}
		out, err = s.view(ctx, tx, sh, storedTime(s.clock.Now()), buildings)
		return err
	})
	return out, err
}

// Payout records cash paid out of the drawer (docs/15 rule 17). It opens the shift when this is the first cash action,
// and never takes out more than the drawer should hold.
func (s *Shifts) Payout(ctx context.Context, c Caller, retryID string, in PayoutInput) (ShiftView, error) {
	const op = "recordCashPayout"
	if err := s.checkRole(op, c); err != nil {
		return ShiftView{}, err
	}
	if err := checkKey(retryID); err != nil {
		return ShiftView{}, err
	}
	desc := strings.TrimSpace(in.Description)
	if errs := checkPayout(in.Amount, desc); len(errs) > 0 {
		return ShiftView{}, stay.NewValidationError(errs)
	}
	if !hasDrawer(c) {
		return ShiftView{}, ErrNotFound
	}
	body, _ := json.Marshal(map[string]any{"amount": in.Amount, "description": in.Description}) // plain values: cannot fail
	return s.idempotent(ctx, c, op, routePayout+c.UserID, retryID, RequestHash(body), func(ctx context.Context, tx Tx, buildings []string) (ShiftView, error) {
		now := storedTime(s.clock.Now())
		sh, err := s.ensureOpen(ctx, tx, c, now)
		if err != nil {
			return ShiftView{}, err
		}
		v, err := s.view(ctx, tx, sh, now, buildings)
		if err != nil {
			return ShiftView{}, err
		}
		if in.Amount > v.ExpectedCash {
			return ShiftView{}, stay.NewValidationError([]stay.FieldError{{Path: "amount", Code: stay.CodeMax}})
		}
		if err := s.addEntry(ctx, tx, c, sh.ID, shift.Payout, in.Amount, "", "", desc, now); err != nil {
			return ShiftView{}, err
		}
		if err := s.auditShift(ctx, tx, c, auditPayout, sh.ID, map[string]any{"shiftId": sh.ID, "amount": in.Amount}); err != nil {
			return ShiftView{}, err
		}
		return s.view(ctx, tx, sh, now, buildings)
	})
}

func checkPayout(amount int64, desc string) []stay.FieldError {
	var errs []stay.FieldError
	if amount < 1 {
		errs = append(errs, stay.FieldError{Path: "amount", Code: stay.CodeMin})
	}
	switch n := utf8.RuneCountInString(desc); {
	case n == 0:
		errs = append(errs, stay.FieldError{Path: "description", Code: stay.CodeRequired})
	case n > maxPayoutNote:
		errs = append(errs, stay.FieldError{Path: "description", Code: stay.CodeTooLong})
	}
	return errs
}

// idempotent runs fn under the key: a repeated call answers with the stored shift.
func (s *Shifts) idempotent(ctx context.Context, c Caller, op, route, key, hash string,
	fn func(context.Context, Tx, []string) (ShiftView, error)) (ShiftView, error) {
	var out ShiftView
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		buildings, err := s.check(ctx, tx, op, c)
		if err != nil {
			return err
		}
		oc, err := s.idem.Begin(ctx, tx, route, key, hash)
		if err != nil {
			return err
		}
		if oc.Replay {
			return json.Unmarshal(oc.Body, &out)
		}
		if out, err = fn(ctx, tx, buildings); err != nil {
			return err
		}
		body, err := json.Marshal(out)
		if err != nil {
			return fmt.Errorf("encode response: %w", err)
		}
		return s.idem.Complete(ctx, tx, route, key, statusOK, body)
	})
	return out, err
}

// Close locks the figures of the caller's shift with a counted total. A difference needs a reason and raises an alert.
func (s *Shifts) Close(ctx context.Context, c Caller, retryID string, in CloseShiftInput) (ShiftReview, error) {
	const op = "closeShift"
	if err := s.checkRole(op, c); err != nil {
		return ShiftReview{}, err
	}
	if err := checkKey(retryID); err != nil {
		return ShiftReview{}, err
	}
	counted, err := shift.CountedCash(in.Counts)
	if err != nil {
		return ShiftReview{}, stay.NewValidationError([]stay.FieldError{{Path: "counts", Code: stay.CodePattern}})
	}
	if utf8.RuneCountInString(in.Reason) > maxCloseReason {
		return ShiftReview{}, stay.NewValidationError([]stay.FieldError{{Path: "reason", Code: stay.CodeTooLong}})
	}
	if !hasDrawer(c) {
		return ShiftReview{}, ErrNotFound
	}
	hash := RequestHash(closeBody(in))
	var out ShiftReview
	err = s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		buildings, err := s.check(ctx, tx, op, c)
		if err != nil {
			return err
		}
		route := routeClose + c.UserID
		oc, err := s.idem.Begin(ctx, tx, route, retryID, hash)
		if err != nil {
			return err
		}
		if oc.Replay {
			return json.Unmarshal(oc.Body, &out)
		}
		if out, err = s.closeLocked(ctx, tx, c, in, counted, buildings); err != nil {
			return err
		}
		body, err := json.Marshal(out)
		if err != nil {
			return fmt.Errorf("encode response: %w", err)
		}
		return s.idem.Complete(ctx, tx, route, retryID, statusOK, body)
	})
	return out, err
}

func closeBody(in CloseShiftInput) []byte {
	b, _ := json.Marshal(struct {
		Counts    []shift.Count `json:"counts"`
		FloatLeft int64         `json:"floatLeft"`
		Reason    string        `json:"reason"`
		Handover  string        `json:"handoverToUserId"`
	}{in.Counts, in.FloatLeft, in.Reason, in.HandoverToUserID}) // plain values: cannot fail
	return b
}

func (s *Shifts) closeLocked(ctx context.Context, tx Tx, c Caller, in CloseShiftInput, counted int64, buildings []string) (ShiftReview, error) {
	now := storedTime(s.clock.Now())
	sh, ok, err := s.repo.LockOpen(ctx, tx, c.UserID)
	if err != nil {
		return ShiftReview{}, fmt.Errorf("open shift: %w", err)
	}
	if !ok {
		return ShiftReview{}, ErrNotFound
	}
	v, err := s.view(ctx, tx, sh, now, buildings)
	if err != nil {
		return ShiftReview{}, err
	}
	if errs := shift.CheckClose(counted, v.ExpectedCash, in.FloatLeft, in.Reason); len(errs) > 0 {
		return ShiftReview{}, shiftErrors(errs)
	}
	diff := counted - v.ExpectedCash
	counts, _ := json.Marshal(in.Counts) // plain values: cannot fail
	cl := ShiftClose{ShiftID: sh.ID, At: now, Expected: v.ExpectedCash, Counted: counted, Diff: diff, FloatLeft: in.FloatLeft,
		Reason: strings.TrimSpace(in.Reason), HandoverTo: in.HandoverToUserID, CountsJSON: counts}
	if err := s.repo.Close(ctx, tx, cl); err != nil {
		return ShiftReview{}, fmt.Errorf("close shift: %w", err) // keeps ErrShiftNotOpen visible
	}
	if err := s.alertDifference(ctx, tx, c, sh.ID, v.ExpectedCash, counted, diff); err != nil {
		return ShiftReview{}, err
	}
	after := map[string]any{"shiftId": sh.ID, "expected": v.ExpectedCash, "counted": counted, "difference": diff, "floatLeft": in.FloatLeft}
	if err := s.auditShift(ctx, tx, c, auditShiftClose, sh.ID, after); err != nil {
		return ShiftReview{}, err
	}
	closed, ok, err := s.repo.ByID(ctx, tx, sh.ID)
	if err != nil || !ok {
		return ShiftReview{}, fmt.Errorf("closed shift (found %v): %w", ok, err)
	}
	return s.review(ctx, tx, closed, buildings)
}

func shiftErrors(errs []shift.FieldError) error {
	out := make([]stay.FieldError, len(errs))
	for i, e := range errs {
		out[i] = stay.FieldError{Path: e.Path, Code: e.Code}
	}
	return stay.NewValidationError(out)
}

func (s *Shifts) alertDifference(ctx context.Context, tx Tx, c Caller, shiftID string, expected, counted, diff int64) error {
	if diff == 0 {
		return nil
	}
	kind, amount := AlertCashOver, diff
	if diff < 0 {
		kind, amount = AlertCashShort, -diff
	}
	a := AlertDraft{ID: s.ids.New(alertIDPrefix), Kind: kind, ShiftID: shiftID, By: c.UserID, Amount: &amount,
		Details: map[string]string{"expected": fmt.Sprint(expected), "counted": fmt.Sprint(counted)}}
	if err := s.alerts.Raise(ctx, tx, a); err != nil {
		return fmt.Errorf("raise alert: %w", err)
	}
	return nil
}

func (s *Shifts) auditShift(ctx context.Context, tx Tx, c Caller, action, shiftID string, after any) error {
	raw, err := json.Marshal(after)
	if err != nil {
		return fmt.Errorf("encode audit: %w", err)
	}
	e := AuditEntry{ID: s.ids.New(auditIDPrefix), ActorID: c.UserID, Action: action, EntityType: entityShift, EntityID: shiftID, After: raw}
	if err := s.audit.Append(ctx, tx, e); err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	return nil
}

// Review is the owner's reconciliation detail of a closed shift.
func (s *Shifts) Review(ctx context.Context, c Caller, shiftID string) (ShiftReview, error) {
	const op = "getShiftReview"
	if err := s.checkRole(op, c); err != nil {
		return ShiftReview{}, err
	}
	var out ShiftReview
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		sh, ok, err := s.repo.ByID(ctx, tx, shiftID)
		if err != nil {
			return fmt.Errorf("shift: %w", err)
		}
		if !ok || sh.Status != shiftClosed {
			return ErrNotFound
		}
		_, buildings, err := s.maxEditable(ctx, tx, c)
		if err != nil {
			return err
		}
		out, err = s.review(ctx, tx, sh, buildings)
		return err
	})
	return out, err
}

func (s *Shifts) review(ctx context.Context, tx Tx, sh ShiftRecord, buildings []string) (ShiftReview, error) {
	if sh.Counted == nil || sh.Difference == nil || sh.ClosedAt == nil {
		return ShiftReview{}, fmt.Errorf("shift %s is closed without figures", sh.ID)
	}
	v, err := s.view(ctx, tx, sh, *sh.ClosedAt, buildings)
	if err != nil {
		return ShiftReview{}, err
	}
	rows, err := s.repo.CashIn(ctx, tx, sh.ID)
	if err != nil {
		return ShiftReview{}, fmt.Errorf("cash received: %w", err)
	}
	loc, err := loadZone(ctx, tx, s.repo)
	if err != nil {
		return ShiftReview{}, err
	}
	y, m, _ := sh.ClosedAt.In(loc).Date()
	from := time.Date(y, m, 1, 0, 0, 0, 0, loc)
	withDiff, short, err := s.repo.MonthStats(ctx, tx, sh.UserID, from, from.AddDate(0, 1, 0))
	if err != nil {
		return ShiftReview{}, fmt.Errorf("staff history: %w", err)
	}
	r := ShiftReview{Shift: v, CountedCash: *sh.Counted, Difference: *sh.Difference, Reason: sh.Reason,
		ReasonRecordedAt: utcPtr(sh.ReasonAt), CashPayments: make([]CashPaymentRow, len(rows)), ShiftsWithDiff: withDiff, TotalShort: short}
	for i, row := range rows {
		r.CashPayments[i] = CashPaymentRow{RoomCode: row.RoomCode, RentalType: row.RentalType, At: row.At.UTC(), Amount: row.Amount}
	}
	return r, nil
}

// ListClosed lists closed shifts, newest first, for the owner and the manager.
func (s *Shifts) ListClosed(ctx context.Context, c Caller, q ClosedShiftsQuery) (ClosedShiftPage, error) {
	if err := s.checkRole("listClosedShifts", c); err != nil {
		return ClosedShiftPage{}, err
	}
	var out ClosedShiftPage
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		loc, err := loadZone(ctx, tx, s.repo)
		if err != nil {
			return err
		}
		f := ClosedFilter{UserID: q.UserID, OnlyDifferences: q.OnlyDifferences, Limit: closedPageSize + 1}
		if q.Month != "" {
			start, perr := time.ParseInLocation("2006-01", q.Month, loc)
			if perr != nil {
				return stay.NewValidationError([]stay.FieldError{{Path: "month", Code: stay.CodePattern}})
			}
			f.From, f.To = start, start.AddDate(0, 1, 0)
		}
		if q.Cursor != "" {
			if f.CursorAt, f.CursorID, err = decodeCursor(q.Cursor); err != nil {
				return err
			}
		}
		rows, err := s.repo.ListClosed(ctx, tx, f)
		if err != nil {
			return fmt.Errorf("closed shifts: %w", err)
		}
		out.Items = rows
		if len(rows) > closedPageSize {
			last := rows[closedPageSize-1]
			out.Items, out.NextCursor = rows[:closedPageSize], encodeCursor(last.ClosedAt, last.ID)
		}
		return nil
	})
	return out, err
}
