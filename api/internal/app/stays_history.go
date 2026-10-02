package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

const (
	// defaultFrontDeskHistoryDays is how far back a receptionist may look (docs/15 rule 5).
	// ponytail: a constant until the property settings (L-A3) hold `frontDeskHistoryDays`.
	defaultFrontDeskHistoryDays = 7
	maxHistoryRangeDays         = 366
	historyPageSize             = 50
	cursorSeparator             = "|"
)

// StayListPage is one page of the history; NextCursor is empty on the last page.
type StayListPage struct {
	Items      []StayListRow
	NextCursor string
}

// StayListQuery is the request: either Date alone or From and To (days in the tenant zone), plus filters.
type StayListQuery struct {
	Date, From, To                   *time.Time // only the calendar day is used
	Query, BuildingID, State, Cursor string
}

// StayHistory holds the read side of stays: history list, owner timeline and receipt (SG-802, SG-904).
type StayHistory struct {
	uow   UnitOfWork
	repo  StayHistoryRepo
	clock Clock
	guard
}

func NewStayHistory(uow UnitOfWork, repo StayHistoryRepo, levels BuildingLevels, clock Clock) *StayHistory {
	return &StayHistory{uow: uow, repo: repo, clock: clock, guard: guard{levels: levels}}
}

// ListStays returns the stays checked in on the chosen days, newest first, from buildings the caller can view.
// A receptionist is limited to the last defaultFrontDeskHistoryDays days; owners and managers have no limit.
func (h *StayHistory) ListStays(ctx context.Context, c Caller, q StayListQuery) (StayListPage, error) {
	const op = "listStays"
	if err := h.checkRole(op, c); err != nil {
		return StayListPage{}, err
	}
	var out StayListPage
	err := h.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		loc, err := loadZone(ctx, tx, h.repo)
		if err != nil {
			return err
		}
		f, err := h.filter(ctx, tx, c, q, loc)
		if err != nil {
			return err
		}
		rows, err := h.repo.Stays(ctx, tx, f)
		if err != nil {
			return fmt.Errorf("stays: %w", err)
		}
		out = pageOf(rows)
		return nil
	})
	return out, err
}

func (h *StayHistory) filter(ctx context.Context, tx Tx, c Caller, q StayListQuery, loc *time.Location) (StayFilter, error) {
	from, to, err := h.window(c, q, loc)
	if err != nil {
		return StayFilter{}, err
	}
	allowed, err := h.viewable(ctx, tx, c)
	if err != nil {
		return StayFilter{}, err
	}
	f := StayFilter{From: from, To: to, BuildingIDs: allowed, BuildingID: q.BuildingID, State: q.State,
		Query: strings.TrimSpace(q.Query), Limit: historyPageSize + 1}
	if q.BuildingID != "" && !contains(allowed, q.BuildingID) {
		return StayFilter{}, access.ErrBuildingForbidden
	}
	if q.State != "" && !historyStates[q.State] {
		return StayFilter{}, stay.NewValidationError([]stay.FieldError{{Path: "state", Code: stay.CodePattern}})
	}
	if q.Cursor != "" {
		if f.CursorAt, f.CursorID, err = decodeCursor(q.Cursor); err != nil {
			return StayFilter{}, err
		}
	}
	return f, nil
}

var historyStates = map[string]bool{"IN_STAY": true, "PAID": true, "UNPAID": true, "MISMATCH": true, "TIME_EDITED": true}

// window turns the requested days into instants: [start of first day, start of the day after the last).
func (h *StayHistory) window(c Caller, q StayListQuery, loc *time.Location) (time.Time, time.Time, error) {
	today := dayOf(h.clock.Now().In(loc), loc)
	first, last := today, today
	switch {
	case q.Date != nil:
		first = dayOf(q.Date.In(time.UTC), loc)
		last = first
	case q.From != nil || q.To != nil:
		if q.From == nil || q.To == nil {
			return time.Time{}, time.Time{}, stay.NewValidationError([]stay.FieldError{{Path: "to", Code: stay.CodeRequired}})
		}
		first, last = dayOf(q.From.In(time.UTC), loc), dayOf(q.To.In(time.UTC), loc)
	}
	if last.Before(first) {
		return time.Time{}, time.Time{}, stay.NewValidationError([]stay.FieldError{{Path: "to", Code: stay.CodeMin}})
	}
	if last.Sub(first) > maxHistoryRangeDays*24*time.Hour {
		return time.Time{}, time.Time{}, stay.NewValidationError([]stay.FieldError{{Path: "to", Code: stay.CodeMax}})
	}
	if c.Role == access.RoleReceptionist && first.Before(today.AddDate(0, 0, -defaultFrontDeskHistoryDays)) {
		return time.Time{}, time.Time{}, stay.NewValidationError([]stay.FieldError{{Path: historyDayField(q), Code: stay.CodeMin}})
	}
	return first, last.AddDate(0, 0, 1), nil
}

func historyDayField(q StayListQuery) string {
	if q.Date != nil {
		return "date"
	}
	return "from"
}

// dayOf is the start of t's calendar day in loc for a date given as a calendar date (its y/m/d fields).
func dayOf(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

// viewable lists the buildings the caller has VIEW or EDIT on.
func (h *StayHistory) viewable(ctx context.Context, tx Tx, c Caller) ([]string, error) {
	ids, err := h.repo.BuildingIDs(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("buildings: %w", err)
	}
	lv, err := h.levels.Levels(ctx, c, ids)
	if err != nil {
		return nil, fmt.Errorf("building levels: %w", err)
	}
	out := []string{}
	for _, id := range ids {
		if lv[id] >= access.VIEW {
			out = append(out, id)
		}
	}
	return out, nil
}

func contains(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// pageOf cuts the extra look-ahead row and turns it into the next cursor.
func pageOf(rows []StayListRow) StayListPage {
	if len(rows) <= historyPageSize {
		return StayListPage{Items: rows}
	}
	last := rows[historyPageSize-1]
	return StayListPage{Items: rows[:historyPageSize], NextCursor: encodeCursor(last.CheckInAt, last.ID)}
}

func encodeCursor(at time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + cursorSeparator + id))
}

func decodeCursor(s string) (*time.Time, string, error) {
	bad := stay.NewValidationError([]stay.FieldError{{Path: "cursor", Code: stay.CodePattern}})
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, "", bad
	}
	at, id, ok := strings.Cut(string(raw), cursorSeparator)
	if !ok || id == "" {
		return nil, "", bad
	}
	t, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return nil, "", bad
	}
	return &t, id, nil
}

// Timeline returns everything that happened to a stay, oldest first (owner and manager).
func (h *StayHistory) Timeline(ctx context.Context, c Caller, stayID string) ([]TimelineRow, error) {
	if err := h.checkRole("getStayTimeline", c); err != nil {
		return nil, err
	}
	var out []TimelineRow
	err := h.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		bid, ok, err := h.repo.StayBuilding(ctx, tx, stayID)
		if err != nil {
			return fmt.Errorf("stay: %w", err)
		}
		if !ok {
			return ErrNotFound
		}
		if err := h.checkBuilding(ctx, "getStayTimeline", c, bid); err != nil {
			return err
		}
		if out, err = h.repo.Timeline(ctx, tx, stayID); err != nil {
			return fmt.Errorf("timeline: %w", err)
		}
		return nil
	})
	return out, err
}

// ReceiptView is the printable receipt. Lines come from the frozen invoice quote, never a re-pricing.
type ReceiptView struct {
	PropertyName, BillCode, RoomCode string
	CheckInAt, CheckOutAt            time.Time
	Lines                            []LineView
	Extras                           []ExtraView
	Total, Deposit                   int64
	Payments                         []ReceiptPayment
}

type ReceiptPayment struct {
	ID, Method string
	Amount     int64
	At         time.Time
}

// Receipt returns the receipt data of an invoice.
func (h *StayHistory) Receipt(ctx context.Context, c Caller, invoiceID string) (ReceiptView, error) {
	const op = "getReceipt"
	if err := h.checkRole(op, c); err != nil {
		return ReceiptView{}, err
	}
	var out ReceiptView
	err := h.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		rec, ok, err := h.repo.Receipt(ctx, tx, invoiceID)
		if err != nil {
			return fmt.Errorf("receipt: %w", err)
		}
		if !ok {
			return ErrNotFound
		}
		if err := h.checkBuilding(ctx, op, c, rec.BuildingID); err != nil {
			return err
		}
		out, err = receiptOf(rec)
		return err
	})
	return out, err
}

func receiptOf(rec ReceiptRecord) (ReceiptView, error) {
	var q QuoteView
	if err := json.Unmarshal(rec.Quote, &q); err != nil {
		return ReceiptView{}, fmt.Errorf("stored invoice quote: %w", err)
	}
	_, extras, err := extrasFor(rec.Extras)
	if err != nil {
		return ReceiptView{}, err
	}
	v := ReceiptView{PropertyName: rec.PropertyName, BillCode: rec.BillCode, RoomCode: rec.RoomCode,
		CheckInAt: rec.CheckInAt.UTC(), CheckOutAt: rec.CheckOutAt.UTC(), Lines: q.Lines, Extras: extras,
		Total: q.Total, Deposit: q.DepositPaid, Payments: make([]ReceiptPayment, len(rec.Payments))}
	for i, p := range rec.Payments {
		v.Payments[i] = ReceiptPayment{ID: p.ID, Method: p.Method, Amount: p.Amount, At: p.At.UTC()}
	}
	return v, nil
}
