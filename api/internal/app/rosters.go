package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/roster"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

const (
	leaveIDPrefix     = "lv"
	maxRosterRange    = 62 // days in one roster read
	maxRosterChanges  = 500
	maxRosterYears    = 1
	maxDeclineRunes   = 200
	minDeclineRunes   = 2
	auditRosterPut    = "roster.updated"
	auditRosterCopy   = "roster.copied"
	auditLeaveNew     = "leave.requested"
	auditLeaveDecided = "leave.decided"
	entityRoster      = "roster"
	entityLeave       = "leave"
)

// LeaveView is a leave request as the contract shows it; Status is TAKEN for approved leave that is over.
type LeaveView struct {
	ID, UserID, UserName, Shift, Kind, Reason, CoverUserID, Status, DeclineReason string
	From, To                                                                      time.Time
	CreatedAt                                                                     time.Time
	DecidedAt                                                                     *time.Time
}

// RosterView is a date range of the roster: who works, leave that touches it, and shifts with nobody on duty.
type RosterView struct {
	From, To    time.Time
	Assignments []RosterCell
	Leave       []LeaveView
	Gaps        []roster.Cell
}

type LeaveBalance struct{ Year, Annual, Used, Left int }

type MyLeave struct {
	Items   []LeaveView
	Balance LeaveBalance
}

type CreateLeaveInput struct {
	From, To                     time.Time
	Shift, Kind, Reason, CoverID string
}

// Rosters holds the roster and leave use cases (SG-1102, SG-1103).
type Rosters struct {
	uow    UnitOfWork
	repo   RosterRepo
	idem   IdempotencyStore
	audit  AuditWriter
	alerts AlertWriter
	ids    IDGenerator
	clock  Clock
	guard
}

func NewRosters(uow UnitOfWork, repo RosterRepo, levels BuildingLevels, idem IdempotencyStore, audit AuditWriter,
	alerts AlertWriter, ids IDGenerator, clock Clock) *Rosters {
	return &Rosters{uow: uow, repo: repo, idem: idem, audit: audit, alerts: alerts, ids: ids, clock: clock, guard: guard{levels: levels}}
}

// calendarDay reads a date given as a calendar date (its year, month and day) as midnight UTC.
func calendarDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func (r *Rosters) today(ctx context.Context, tx Tx) (time.Time, error) {
	loc, err := loadZone(ctx, tx, r.repo)
	if err != nil {
		return time.Time{}, err
	}
	return calendarDay(r.clock.Now().In(loc)), nil
}

func leaveViewOf(l LeaveRow, today time.Time) LeaveView {
	status := l.Status
	if status == roster.Approved && l.To.Before(today) {
		status = roster.Taken
	}
	return LeaveView{ID: l.ID, UserID: l.UserID, UserName: l.UserName, Shift: l.Shift, Kind: l.Kind, Reason: l.Reason,
		CoverUserID: l.CoverUserID, Status: status, DeclineReason: l.DeclineReason, From: l.From, To: l.To,
		CreatedAt: l.CreatedAt.UTC(), DecidedAt: utcPtr(l.DecidedAt)}
}

// runIdem runs fn once per key in one transaction; a repeated key answers with the stored body.
func runIdem[T any](ctx context.Context, r *Rosters, c Caller, route, key, hash string, status int, fn func(context.Context, Tx) (T, error)) (T, error) {
	var out T
	err := r.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		oc, err := r.idem.Begin(ctx, tx, route, key, hash)
		if err != nil {
			return err
		}
		if oc.Replay {
			return json.Unmarshal(oc.Body, &out)
		}
		if out, err = fn(ctx, tx); err != nil {
			return err
		}
		body, err := json.Marshal(out)
		if err != nil {
			return fmt.Errorf("encode response: %w", err)
		}
		return r.idem.Complete(ctx, tx, route, key, status, body)
	})
	return out, err
}

func rangeOf(from, to time.Time) (time.Time, time.Time, error) {
	f, t := calendarDay(from), calendarDay(to)
	switch {
	case t.Before(f):
		return f, t, stay.NewValidationError([]stay.FieldError{{Path: "to", Code: stay.CodeMin}})
	case roster.Days(f, t) > maxRosterRange:
		return f, t, stay.NewValidationError([]stay.FieldError{{Path: "to", Code: stay.CodeMax}})
	}
	return f, t, nil
}

// GetRoster returns the roster of a date range for the owner and the manager.
func (r *Rosters) GetRoster(ctx context.Context, c Caller, from, to time.Time) (RosterView, error) {
	if err := r.checkRole("getRoster", c); err != nil {
		return RosterView{}, err
	}
	f, t, err := rangeOf(from, to)
	if err != nil {
		return RosterView{}, err
	}
	var out RosterView
	err = r.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		out, err = r.view(ctx, tx, f, t, "", true)
		return err
	})
	return out, err
}

// MyRoster is the caller's own shifts and leave in a date range.
func (r *Rosters) MyRoster(ctx context.Context, c Caller, from, to time.Time) (RosterView, error) {
	if err := r.checkRole("getMyRoster", c); err != nil {
		return RosterView{}, err
	}
	f, t, err := rangeOf(from, to)
	if err != nil {
		return RosterView{}, err
	}
	var out RosterView
	err = r.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		out, err = r.view(ctx, tx, f, t, c.UserID, false)
		return err
	})
	return out, err
}

// view reads a range; userID limits it to one person, withGaps adds the uncovered shifts (whole roster only).
func (r *Rosters) view(ctx context.Context, tx Tx, from, to time.Time, userID string, withGaps bool) (RosterView, error) {
	today, err := r.today(ctx, tx)
	if err != nil {
		return RosterView{}, err
	}
	cells, err := r.repo.Assignments(ctx, tx, from, to)
	if err != nil {
		return RosterView{}, fmt.Errorf("assignments: %w", err)
	}
	rows, err := r.repo.Leave(ctx, tx, LeaveFilter{UserID: userID, From: &from, To: &to, Standing: true})
	if err != nil {
		return RosterView{}, fmt.Errorf("leave: %w", err)
	}
	out := RosterView{From: from, To: to, Assignments: []RosterCell{}, Leave: make([]LeaveView, len(rows))}
	for _, cell := range cells {
		if userID == "" || cell.UserID == userID {
			out.Assignments = append(out.Assignments, cell)
		}
	}
	covers := make([]roster.Leave, len(rows))
	for i, l := range rows {
		out.Leave[i] = leaveViewOf(l, today)
		covers[i] = roster.Leave{UserID: l.UserID, Shift: l.Shift, Status: l.Status, Cover: l.CoverUserID, From: l.From, To: l.To}
	}
	if withGaps {
		as := make([]roster.Assignment, len(cells))
		for i, cell := range cells {
			as[i] = roster.Assignment{UserID: cell.UserID, Date: cell.Date, Shift: cell.Shift}
		}
		out.Gaps = roster.Gaps(from, to, today, as, covers)
	}
	return out, nil
}

// PutRoster sets and removes assignments in one change. Only people who have not been removed can be rostered, and
// nobody can be set on a shift that their standing leave covers.
func (r *Rosters) PutRoster(ctx context.Context, c Caller, key string, set, remove []RosterCell) (RosterView, error) {
	const op = "putRoster"
	if err := r.checkRole(op, c); err != nil {
		return RosterView{}, err
	}
	if err := checkKey(key); err != nil {
		return RosterView{}, err
	}
	set, remove = normalise(set), normalise(remove)
	if len(set)+len(remove) > maxRosterChanges {
		return RosterView{}, stay.NewValidationError([]stay.FieldError{{Path: "set", Code: stay.CodeMax}})
	}
	if err := r.checkCells(set, remove); err != nil {
		return RosterView{}, err
	}
	if len(set)+len(remove) == 0 {
		return RosterView{}, stay.NewValidationError([]stay.FieldError{{Path: "set", Code: stay.CodeRequired}})
	}
	body, _ := json.Marshal(struct {
		Set    []RosterCell `json:"set"`
		Remove []RosterCell `json:"remove"`
	}{set, remove}) // plain values: cannot fail
	return runIdem(ctx, r, c, "PUT /v1/owner/roster", key, RequestHash(body), statusOK, func(ctx context.Context, tx Tx) (RosterView, error) {
		if err := r.checkPeople(ctx, tx, set); err != nil {
			return RosterView{}, err
		}
		if err := r.checkLeaveClash(ctx, tx, set); err != nil {
			return RosterView{}, err
		}
		if err := r.repo.Remove(ctx, tx, remove); err != nil {
			return RosterView{}, fmt.Errorf("remove assignments: %w", err)
		}
		if err := r.repo.Add(ctx, tx, set, c.UserID); err != nil {
			return RosterView{}, fmt.Errorf("add assignments: %w", err)
		}
		if err := r.auditRoster(ctx, tx, c, auditRosterPut, map[string]any{"set": len(set), "removed": len(remove)}); err != nil {
			return RosterView{}, err
		}
		from, to := span(append(append([]RosterCell{}, set...), remove...))
		return r.view(ctx, tx, from, to, "", true)
	})
}

func normalise(cells []RosterCell) []RosterCell {
	out := make([]RosterCell, len(cells))
	for i, c := range cells {
		out[i] = RosterCell{UserID: c.UserID, Date: calendarDay(c.Date), Shift: c.Shift}
	}
	return out
}

func (r *Rosters) checkCells(sets ...[]RosterCell) error {
	for _, cells := range sets {
		for _, c := range cells {
			if c.UserID == "" || !roster.ValidShift(c.Shift) || c.Date.Year() < 2000 || c.Date.Year() > r.clock.Now().Year()+maxRosterYears {
				return stay.NewValidationError([]stay.FieldError{{Path: "set", Code: stay.CodePattern}})
			}
		}
	}
	return nil
}

func span(cells []RosterCell) (time.Time, time.Time) {
	from, to := cells[0].Date, cells[0].Date
	for _, c := range cells {
		if c.Date.Before(from) {
			from = c.Date
		}
		if c.Date.After(to) {
			to = c.Date
		}
	}
	if roster.Days(from, to) > maxRosterRange { // the answer is capped; the change itself is not
		to = from.AddDate(0, 0, maxRosterRange-1)
	}
	return from, to
}

func (r *Rosters) checkPeople(ctx context.Context, tx Tx, set []RosterCell) error {
	if len(set) == 0 {
		return nil
	}
	seen, ids := map[string]bool{}, []string{}
	for _, c := range set {
		if !seen[c.UserID] {
			seen[c.UserID] = true
			ids = append(ids, c.UserID)
		}
	}
	ok, err := r.repo.ActiveUsers(ctx, tx, ids)
	if err != nil {
		return fmt.Errorf("active users: %w", err)
	}
	for _, id := range ids {
		if !ok[id] {
			return stay.NewValidationError([]stay.FieldError{{Path: "set", Code: "UNKNOWN_USER"}})
		}
	}
	return nil
}

func (r *Rosters) checkLeaveClash(ctx context.Context, tx Tx, set []RosterCell) error {
	if len(set) == 0 {
		return nil
	}
	from, to := span(set)
	rows, err := r.repo.Leave(ctx, tx, LeaveFilter{From: &from, To: &to, Standing: true})
	if err != nil {
		return fmt.Errorf("leave: %w", err)
	}
	for _, l := range rows {
		rl := roster.Leave{UserID: l.UserID, Shift: l.Shift, Status: l.Status, From: l.From, To: l.To}
		for _, c := range set {
			if c.UserID == l.UserID && rl.Covers(roster.Cell{Date: c.Date, Shift: c.Shift}) {
				return ErrLeaveConflict
			}
		}
	}
	return nil
}

// CopyWeek copies the week before weekStart into the empty week that starts on weekStart (a Monday).
func (r *Rosters) CopyWeek(ctx context.Context, c Caller, key string, weekStart time.Time) (RosterView, error) {
	if err := r.checkRole("copyRosterWeek", c); err != nil {
		return RosterView{}, err
	}
	if err := checkKey(key); err != nil {
		return RosterView{}, err
	}
	start := calendarDay(weekStart)
	if !roster.IsMonday(start) {
		return RosterView{}, stay.NewValidationError([]stay.FieldError{{Path: "weekStart", Code: stay.CodePattern}})
	}
	hash := RequestHash([]byte(`{"weekStart":"` + start.Format(time.DateOnly) + `"}`))
	return runIdem(ctx, r, c, "POST /v1/owner/roster/copy-week", key, hash, statusOK, func(ctx context.Context, tx Tx) (RosterView, error) {
		end := start.AddDate(0, 0, 6)
		n, err := r.repo.CountBetween(ctx, tx, start, end)
		if err != nil {
			return RosterView{}, fmt.Errorf("count assignments: %w", err)
		}
		if n > 0 {
			return RosterView{}, ErrRosterNotEmpty
		}
		if err := r.repo.CopyWeek(ctx, tx, start.AddDate(0, 0, -7), c.UserID); err != nil {
			return RosterView{}, fmt.Errorf("copy week: %w", err)
		}
		if err := r.auditRoster(ctx, tx, c, auditRosterCopy, map[string]any{"weekStart": start.Format(time.DateOnly)}); err != nil {
			return RosterView{}, err
		}
		return r.view(ctx, tx, start, end, "", true)
	})
}

func (r *Rosters) auditRoster(ctx context.Context, tx Tx, c Caller, action string, after any) error {
	return r.auditEntity(ctx, tx, c, action, entityRoster, c.TenantID, after)
}

func (r *Rosters) auditEntity(ctx context.Context, tx Tx, c Caller, action, entity, id string, after any) error {
	raw, err := json.Marshal(after)
	if err != nil {
		return fmt.Errorf("encode audit: %w", err)
	}
	e := AuditEntry{ID: r.ids.New(auditIDPrefix), ActorID: c.UserID, Action: action, EntityType: entity, EntityID: id, After: raw}
	if err := r.audit.Append(ctx, tx, e); err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	return nil
}

// ListLeave lists leave requests, newest first, optionally of one status (owner and manager).
func (r *Rosters) ListLeave(ctx context.Context, c Caller, status string) ([]LeaveView, error) {
	if err := r.checkRole("listLeaveRequests", c); err != nil {
		return nil, err
	}
	if status != "" && status != roster.Taken && !leaveStatuses[status] {
		return nil, stay.NewValidationError([]stay.FieldError{{Path: "status", Code: stay.CodePattern}})
	}
	var out []LeaveView
	err := r.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		today, err := r.today(ctx, tx)
		if err != nil {
			return err
		}
		stored := status
		if status == roster.Taken {
			stored = roster.Approved // TAKEN is approved leave that is over: filter after the view is derived
		}
		rows, err := r.repo.Leave(ctx, tx, LeaveFilter{Status: stored})
		if err != nil {
			return fmt.Errorf("leave: %w", err)
		}
		out = []LeaveView{}
		for _, l := range rows {
			if v := leaveViewOf(l, today); status == "" || v.Status == status {
				out = append(out, v)
			}
		}
		return nil
	})
	return out, err
}

var leaveStatuses = map[string]bool{roster.Pending: true, roster.Approved: true, roster.Declined: true, roster.Cancelled: true, roster.CancelRequested: true}

// Approve approves a request, or approves a request to cancel approved leave (which ends it).
func (r *Rosters) Approve(ctx context.Context, c Caller, key, id string) (LeaveView, error) {
	return r.decide(ctx, c, "approveLeave", key, id, roster.Approve, "")
}

// Decline declines a request with a reason; declining a request to cancel leaves the approved leave standing.
func (r *Rosters) Decline(ctx context.Context, c Caller, key, id, reason string) (LeaveView, error) {
	reason = strings.TrimSpace(reason)
	if n := len([]rune(reason)); n < minDeclineRunes || n > maxDeclineRunes {
		if err := r.checkRole("declineLeave", c); err != nil {
			return LeaveView{}, err
		}
		return LeaveView{}, stay.NewValidationError([]stay.FieldError{{Path: "reason", Code: map[bool]string{true: stay.CodeTooShort, false: stay.CodeTooLong}[n < minDeclineRunes]}})
	}
	return r.decide(ctx, c, "declineLeave", key, id, roster.Decline, reason)
}

// CancelMine cancels the caller's own pending request at once, or asks to cancel an approved one.
func (r *Rosters) CancelMine(ctx context.Context, c Caller, key, id string) (LeaveView, error) {
	return r.decide(ctx, c, "cancelMyLeave", key, id, roster.Cancel, "")
}

func (r *Rosters) decide(ctx context.Context, c Caller, op, key, id, action, reason string) (LeaveView, error) {
	if err := r.checkRole(op, c); err != nil {
		return LeaveView{}, err
	}
	if err := checkKey(key); err != nil {
		return LeaveView{}, err
	}
	hash := RequestHash([]byte(`{"action":"` + action + `","reason":` + quoteJSON(reason) + `}`))
	return runIdem(ctx, r, c, "POST leave/"+id+"/"+action, key, hash, statusOK, func(ctx context.Context, tx Tx) (LeaveView, error) {
		found, err := r.repo.LockLeave(ctx, tx, id)
		if err != nil {
			return LeaveView{}, fmt.Errorf("lock leave: %w", err)
		}
		l, ok, err := r.repo.LeaveByID(ctx, tx, id)
		if err != nil {
			return LeaveView{}, fmt.Errorf("leave: %w", err)
		}
		mine := ok && l.UserID == c.UserID
		// A person's own requests are theirs to cancel and nobody else's; a manager does not decide their own.
		if !found || !ok || (action == roster.Cancel && !mine) {
			return LeaveView{}, ErrNotFound
		}
		if action != roster.Cancel && mine && c.Role != access.RoleOwner {
			return LeaveView{}, fmt.Errorf("%w: not your own leave", access.ErrRoleForbidden)
		}
		return r.move(ctx, tx, c, l, action, reason)
	})
}

func (r *Rosters) move(ctx context.Context, tx Tx, c Caller, l LeaveRow, action, reason string) (LeaveView, error) {
	today, err := r.today(ctx, tx)
	if err != nil {
		return LeaveView{}, err
	}
	if leaveViewOf(l, today).Status == roster.Taken {
		return LeaveView{}, ErrLeaveState
	}
	to, ok := roster.Next(action, l.Status)
	if !ok {
		return LeaveView{}, ErrLeaveState
	}
	now := storedTime(r.clock.Now())
	decline := ""
	if action == roster.Decline {
		decline = reason
	}
	if err := r.repo.SetLeaveStatus(ctx, tx, l.ID, to, decline, now, c.UserID); err != nil {
		return LeaveView{}, fmt.Errorf("set leave status: %w", err)
	}
	if err := r.auditEntity(ctx, tx, c, auditLeaveDecided, entityLeave, l.ID, map[string]any{"leaveId": l.ID, "action": action, "status": to}); err != nil {
		return LeaveView{}, err
	}
	if to == roster.CancelRequested {
		if err := r.raiseLeave(ctx, tx, c, l, "cancel"); err != nil {
			return LeaveView{}, err
		}
	}
	l.Status, l.DeclineReason, l.DecidedAt = to, decline, &now
	return leaveViewOf(l, today), nil
}

func (r *Rosters) raiseLeave(ctx context.Context, tx Tx, c Caller, l LeaveRow, what string) error {
	a := AlertDraft{ID: r.ids.New(alertIDPrefix), Kind: AlertLeaveRequested, By: c.UserID,
		Details: map[string]string{"from": l.From.Format(time.DateOnly), "to": l.To.Format(time.DateOnly), "kind": l.Kind, "request": what}}
	if err := r.alerts.Raise(ctx, tx, a); err != nil {
		return fmt.Errorf("raise alert: %w", err)
	}
	return nil
}

// MyLeave lists the caller's leave requests with the annual balance of the current year.
func (r *Rosters) MyLeave(ctx context.Context, c Caller) (MyLeave, error) {
	if err := r.checkRole("listMyLeaveRequests", c); err != nil {
		return MyLeave{}, err
	}
	var out MyLeave
	err := r.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		today, err := r.today(ctx, tx)
		if err != nil {
			return err
		}
		rows, err := r.repo.Leave(ctx, tx, LeaveFilter{UserID: c.UserID})
		if err != nil {
			return fmt.Errorf("leave: %w", err)
		}
		out.Items = make([]LeaveView, len(rows))
		for i, l := range rows {
			out.Items[i] = leaveViewOf(l, today)
		}
		year := today.Year()
		annual, err := r.repo.AnnualLeaveDays(ctx, tx, c.UserID)
		if err != nil {
			return fmt.Errorf("annual leave: %w", err)
		}
		used, err := r.repo.PaidLeaveDays(ctx, tx, c.UserID, time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC))
		if err != nil {
			return fmt.Errorf("used leave: %w", err)
		}
		out.Balance = LeaveBalance{Year: year, Annual: annual, Used: used, Left: max(annual-used, 0)}
		return nil
	})
	return out, err
}

// CreateLeave records a leave request for the caller and tells the owner. It cannot overlap another request that is
// pending or stands.
func (r *Rosters) CreateLeave(ctx context.Context, c Caller, key string, in CreateLeaveInput) (LeaveView, error) {
	const op = "createLeaveRequest"
	if err := r.checkRole(op, c); err != nil {
		return LeaveView{}, err
	}
	if err := checkKey(key); err != nil {
		return LeaveView{}, err
	}
	from, to := calendarDay(in.From), calendarDay(in.To)
	body, _ := json.Marshal(struct {
		From, To, Shift, Kind, Reason, Cover string
	}{from.Format(time.DateOnly), to.Format(time.DateOnly), in.Shift, in.Kind, in.Reason, in.CoverID}) // plain strings: cannot fail
	return runIdem(ctx, r, c, "POST /v1/me/leave-requests", key, RequestHash(body), statusCreated, func(ctx context.Context, tx Tx) (LeaveView, error) {
		today, err := r.today(ctx, tx)
		if err != nil {
			return LeaveView{}, err
		}
		if errs := roster.CheckLeave(from, to, today, in.Kind, in.Shift, in.Reason, c.UserID, in.CoverID); len(errs) > 0 {
			return LeaveView{}, rosterErrors(errs)
		}
		if in.CoverID != "" {
			if ok, err := r.repo.ActiveUsers(ctx, tx, []string{in.CoverID}); err != nil || !ok[in.CoverID] {
				return LeaveView{}, errOr(err, stay.NewValidationError([]stay.FieldError{{Path: "coverUserId", Code: "UNKNOWN_USER"}}))
			}
		}
		if n, err := r.repo.OverlappingLeave(ctx, tx, c.UserID, from, to); err != nil {
			return LeaveView{}, fmt.Errorf("overlapping leave: %w", err)
		} else if n > 0 {
			return LeaveView{}, ErrLeaveConflict
		}
		now := storedTime(r.clock.Now())
		l := LeaveRow{ID: r.ids.New(leaveIDPrefix), UserID: c.UserID, Shift: in.Shift, Kind: in.Kind, Reason: strings.TrimSpace(in.Reason),
			CoverUserID: in.CoverID, Status: roster.Pending, From: from, To: to, CreatedAt: now}
		if err := r.repo.InsertLeave(ctx, tx, l); err != nil {
			return LeaveView{}, fmt.Errorf("insert leave: %w", err)
		}
		if err := r.raiseLeave(ctx, tx, c, l, "leave"); err != nil {
			return LeaveView{}, err
		}
		if err := r.auditEntity(ctx, tx, c, auditLeaveNew, entityLeave, l.ID, map[string]any{"leaveId": l.ID, "kind": l.Kind}); err != nil {
			return LeaveView{}, err
		}
		stored, ok, err := r.repo.LeaveByID(ctx, tx, l.ID)
		if err != nil || !ok {
			return LeaveView{}, fmt.Errorf("leave %s (found %v): %w", l.ID, ok, err)
		}
		return leaveViewOf(stored, today), nil
	})
}

func rosterErrors(errs []roster.FieldError) error {
	out := make([]stay.FieldError, len(errs))
	for i, e := range errs {
		out[i] = stay.FieldError{Path: e.Path, Code: e.Code}
	}
	return stay.NewValidationError(out)
}
