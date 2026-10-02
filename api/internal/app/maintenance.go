package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
	"github.com/pcaokhai/stayguard/api/internal/domain/ticket"
)

const (
	ticketIDPrefix    = "mt"
	auditTicketNew    = "ticket.created"
	auditTicketUpdate = "ticket.updated"
	auditTicketDone   = "ticket.done"
	auditTicketCost   = "ticket.cost_updated"
	entityTicket      = "ticket"
	// maxTicketCost guards the cost fields against a typo and keeps their sum far from overflow.
	maxTicketCost    = 100_000_000_000
	maxRepairerRunes = 100
	maxTicketNote    = 500
	maxUsageNote     = 500
)

// DamageInput is the raw damage report.
// DamageInput is the raw damage report. PhotoAssetIDs must be empty: photos are not supported yet.
type DamageInput struct {
	Category, Description, Severity string
	PhotoAssetIDs                   []string
}

// UpdateTicketInput holds the fields a request sets; nil means "not in the request".
type UpdateTicketInput struct {
	Status                *string
	RoomLocked            *bool
	ExpectedDoneOn        *time.Time // calendar day
	PartsCost, LabourCost *int64
	Repairer, Note        *string
}

// TicketView is a ticket as the contract shows it.
type TicketView struct {
	ID, Code, RoomID, RoomCode, Category, Description, Status, ReportedBy string
	RoomLocked                                                            bool
	ReportedAt                                                            time.Time
	ExpectedDoneOn                                                        *time.Time
	PartsCost, LabourCost, TotalCost                                      *int64
	Repairer                                                              *string
	CompletedAt                                                           *time.Time
}

// Maintenance holds the damage-report and ticket use cases (SG-1201) and the unused-room report (SG-401).
type Maintenance struct {
	uow    UnitOfWork
	repo   TicketRepo
	idem   IdempotencyStore
	audit  AuditWriter
	alerts AlertWriter
	ids    IDGenerator
	clock  Clock
	// expenses is optional (nil posts nothing): a ticket that is DONE posts its cost as a MAINTENANCE expense (docs/15 rule 8).
	expenses ExpenseLedger
	guard
}

// WithExpenses posts the cost of a finished ticket to the expense ledger.
func (m *Maintenance) WithExpenses(l ExpenseLedger) *Maintenance { m.expenses = l; return m }

func NewMaintenance(uow UnitOfWork, repo TicketRepo, levels BuildingLevels, idem IdempotencyStore, audit AuditWriter,
	alerts AlertWriter, ids IDGenerator, clock Clock) *Maintenance {
	return &Maintenance{uow: uow, repo: repo, idem: idem, audit: audit, alerts: alerts, ids: ids, clock: clock, guard: guard{levels: levels}}
}

func ticketViewOf(r TicketRecord) TicketView {
	return TicketView{ID: r.ID, Code: r.Code, RoomID: r.RoomID, RoomCode: r.RoomCode, Category: r.Category, Description: r.Description,
		Status: r.Status, ReportedBy: r.ReportedBy, RoomLocked: r.RoomLocked, ReportedAt: r.ReportedAt.UTC(), ExpectedDoneOn: r.ExpectedDoneOn,
		PartsCost: r.PartsCost, LabourCost: r.LabourCost, TotalCost: ticket.Total(r.PartsCost, r.LabourCost), Repairer: r.Repairer,
		CompletedAt: utcPtr(r.CompletedAt)}
}

func ticketErrors(errs []ticket.FieldError) error {
	out := make([]stay.FieldError, len(errs))
	for i, e := range errs {
		out[i] = stay.FieldError{Path: e.Path, Code: e.Code}
	}
	return stay.NewValidationError(out)
}

// ReportDamage records damage or missing items as a NEW ticket and tells the owner. With severity LOCK_ROOM the room is
// taken out of service at once, unless a guest is in it (nothing is written then).
func (m *Maintenance) ReportDamage(ctx context.Context, c Caller, roomID, retryID string, in DamageInput) (TicketView, error) {
	const op = "reportDamage"
	if err := m.checkRole(op, c); err != nil {
		return TicketView{}, err
	}
	if err := checkKey(retryID); err != nil {
		return TicketView{}, err
	}
	errs := ticket.CheckReport(in.Category, in.Description, in.Severity)
	if len(in.PhotoAssetIDs) > 0 { // refused, not dropped: there is no asset store yet
		errs = append(errs, ticket.FieldError{Path: "photoAssetIds", Code: "NOT_SUPPORTED"})
	}
	if len(errs) > 0 {
		return TicketView{}, ticketErrors(errs)
	}
	desc := strings.TrimSpace(in.Description)
	hash := RequestHash([]byte(fmt.Sprintf(`{"c":%q,"d":%q,"s":%q}`, in.Category, in.Description, in.Severity)))
	var out TicketView
	err := m.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		rm, err := m.room(ctx, tx, op, c, roomID)
		if err != nil {
			return err
		}
		route := "POST /v1/rooms/" + roomID + "/damage-reports"
		oc, err := m.idem.Begin(ctx, tx, route, retryID, hash)
		if err != nil {
			return err
		}
		if oc.Replay {
			return json.Unmarshal(oc.Body, &out)
		}
		if out, err = m.report(ctx, tx, c, rm, in, desc); err != nil {
			return err
		}
		body, err := json.Marshal(out)
		if err != nil {
			return fmt.Errorf("encode response: %w", err)
		}
		return m.idem.Complete(ctx, tx, route, retryID, statusCreated, body)
	})
	return out, err
}

// room finds and locks a room (a foreign id is a 404 before a 403) and checks EDIT on its building.
func (m *Maintenance) room(ctx context.Context, tx Tx, op string, c Caller, roomID string) (RoomLock, error) {
	rm, ok, err := m.repo.LockRoom(ctx, tx, roomID)
	if err != nil {
		return RoomLock{}, fmt.Errorf("lock room: %w", err)
	}
	if !ok {
		return RoomLock{}, ErrNotFound
	}
	return rm, m.checkBuilding(ctx, op, c, rm.BuildingID)
}

func (m *Maintenance) report(ctx context.Context, tx Tx, c Caller, rm RoomLock, in DamageInput, desc string) (TicketView, error) {
	locked := in.Severity == ticket.LockRoom
	if locked {
		if err := m.lockRoom(ctx, tx, rm); err != nil {
			return TicketView{}, err
		}
	}
	seq, err := m.repo.NextSeq(ctx, tx)
	if err != nil {
		return TicketView{}, fmt.Errorf("ticket number: %w", err)
	}
	now := storedTime(m.clock.Now())
	t := NewTicket{ID: m.ids.New(ticketIDPrefix), Code: fmt.Sprintf("BT-%03d", seq), RoomID: rm.ID, Category: in.Category,
		Description: desc, ReportedBy: c.UserID, RoomLocked: locked, ReportedAt: now}
	if err := m.repo.Insert(ctx, tx, t); err != nil {
		return TicketView{}, fmt.Errorf("insert ticket: %w", err)
	}
	alert := AlertDraft{ID: m.ids.New(alertIDPrefix), Kind: AlertDamageReported, RoomCode: rm.Code, By: c.UserID,
		Details: map[string]string{"ticket": t.Code, "category": in.Category, "severity": in.Severity}}
	if err := m.alerts.Raise(ctx, tx, alert); err != nil {
		return TicketView{}, fmt.Errorf("raise alert: %w", err)
	}
	if err := m.auditTicket(ctx, tx, c, auditTicketNew, t.ID, map[string]any{"ticketId": t.ID, "roomId": rm.ID, "category": in.Category, "roomLocked": locked}); err != nil {
		return TicketView{}, err
	}
	return m.view(ctx, tx, t.ID)
}

func (m *Maintenance) lockRoom(ctx context.Context, tx Tx, rm RoomLock) error {
	if _, err := room.LockForMaintenance(room.Status(rm.Status)); err != nil {
		return ErrRoomOccupied
	}
	return m.repo.SetMaintenance(ctx, tx, rm.ID)
}

func (m *Maintenance) view(ctx context.Context, tx Tx, id string) (TicketView, error) {
	r, ok, err := m.repo.ByID(ctx, tx, id)
	if err != nil || !ok {
		return TicketView{}, fmt.Errorf("ticket %s (found %v): %w", id, ok, err)
	}
	return ticketViewOf(r), nil
}

func (m *Maintenance) auditTicket(ctx context.Context, tx Tx, c Caller, action, ticketID string, after any) error {
	raw, err := json.Marshal(after)
	if err != nil {
		return fmt.Errorf("encode audit: %w", err)
	}
	e := AuditEntry{ID: m.ids.New(auditIDPrefix), ActorID: c.UserID, Action: action, EntityType: entityTicket, EntityID: ticketID, After: raw}
	if err := m.audit.Append(ctx, tx, e); err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	return nil
}

// ListTickets returns tickets newest first, optionally of one status (owner and manager).
func (m *Maintenance) ListTickets(ctx context.Context, c Caller, status string) ([]TicketView, error) {
	if err := m.checkRole("listTickets", c); err != nil {
		return nil, err
	}
	if status != "" && !ticket.ValidStatus(status) {
		return nil, stay.NewValidationError([]stay.FieldError{{Path: "status", Code: stay.CodePattern}})
	}
	var out []TicketView
	err := m.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		rows, err := m.repo.List(ctx, tx, status)
		if err != nil {
			return fmt.Errorf("tickets: %w", err)
		}
		out = make([]TicketView, len(rows))
		for i, r := range rows {
			out[i] = ticketViewOf(r)
		}
		return nil
	})
	return out, err
}

func (m *Maintenance) GetTicket(ctx context.Context, c Caller, id string) (TicketView, error) {
	if err := m.checkRole("getTicket", c); err != nil {
		return TicketView{}, err
	}
	var out TicketView
	err := m.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		r, ok, err := m.repo.ByID(ctx, tx, id)
		if err != nil {
			return fmt.Errorf("ticket: %w", err)
		}
		if !ok {
			return ErrNotFound
		}
		out = ticketViewOf(r)
		return nil
	})
	return out, err
}

// UpdateTicket changes status, room lock, expected date, costs, repairer and note. Costs belong to the owner. DONE
// stamps who and when and puts the room back in service (docs/15 rule 8); the expense posting comes with the finance task.
func (m *Maintenance) UpdateTicket(ctx context.Context, c Caller, id string, in UpdateTicketInput) (TicketView, error) {
	const op = "updateTicket"
	if err := m.checkRole(op, c); err != nil {
		return TicketView{}, err
	}
	if (in.PartsCost != nil || in.LabourCost != nil) && c.Role != access.RoleOwner {
		return TicketView{}, fmt.Errorf("%w: costs are the owner's", access.ErrRoleForbidden)
	}
	if err := checkTicketUpdate(in); err != nil {
		return TicketView{}, err
	}
	var out TicketView
	err := m.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		found, err := m.repo.Lock(ctx, tx, id)
		if err != nil {
			return fmt.Errorf("lock ticket: %w", err)
		}
		rec, ok, err := m.repo.ByID(ctx, tx, id)
		if err != nil {
			return fmt.Errorf("ticket: %w", err)
		}
		if !found || !ok {
			return ErrNotFound
		}
		ch, err := m.change(ctx, tx, c, rec, in)
		if err != nil {
			return err
		}
		if err := m.repo.Update(ctx, tx, ch); err != nil {
			return fmt.Errorf("update ticket: %w", err)
		}
		if ch.Status == ticket.Done {
			if err := m.syncCost(ctx, tx, c, rec, ch); err != nil {
				return err
			}
		}
		action := auditTicketUpdate
		switch {
		case rec.Status == ticket.Done:
			action = auditTicketCost
		case ch.Status == ticket.Done:
			action = auditTicketDone
		}
		after := map[string]any{"ticketId": id, "status": ch.Status, "roomLocked": ch.RoomLocked}
		if action == auditTicketCost {
			after["total"] = ticket.Total(ch.PartsCost, ch.LabourCost)
		}
		if err := m.auditTicket(ctx, tx, c, action, id, after); err != nil {
			return err
		}
		out, err = m.view(ctx, tx, id)
		return err
	})
	return out, err
}

// syncCost keeps the MAINTENANCE expense of a finished ticket equal to its cost, in the month it was completed in: it is
// posted when the ticket is finished with a cost, changed when the owner corrects the cost, and removed when the cost is cleared.
func (m *Maintenance) syncCost(ctx context.Context, tx Tx, c Caller, rec TicketRecord, ch TicketChange) error {
	if m.expenses == nil || ch.CompletedAt == nil {
		return nil
	}
	total := ticket.Total(ch.PartsCost, ch.LabourCost)
	if total == nil || *total == 0 {
		if err := m.expenses.RemoveAuto(ctx, tx, "MAINTENANCE", rec.ID); err != nil {
			return fmt.Errorf("remove maintenance cost: %w", err)
		}
		return nil
	}
	loc, err := loadZone(ctx, tx, m.repo)
	if err != nil {
		return err
	}
	at := ch.CompletedAt.In(loc)
	day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
	e := AutoExpense{ID: m.ids.New(expenseIDPrefix), Source: "MAINTENANCE", RefID: rec.ID, Category: "MAINTENANCE", Month: at.Format("2006-01"),
		Amount: *total, PaidOn: &day, Note: rec.Code, CreatedBy: c.UserID}
	if err := m.expenses.UpsertAuto(ctx, tx, e); err != nil {
		return fmt.Errorf("post maintenance cost: %w", err)
	}
	return nil
}

func checkTicketUpdate(in UpdateTicketInput) error {
	var errs []stay.FieldError
	if in.Status != nil && !ticket.ValidStatus(*in.Status) {
		errs = append(errs, stay.FieldError{Path: "status", Code: stay.CodePattern})
	}
	for path, v := range map[string]*int64{"partsCost": in.PartsCost, "labourCost": in.LabourCost} {
		if v != nil && (*v < 0 || *v > maxTicketCost) {
			errs = append(errs, stay.FieldError{Path: path, Code: stay.CodeMax})
		}
	}
	if in.Repairer != nil && utf8.RuneCountInString(*in.Repairer) > maxRepairerRunes {
		errs = append(errs, stay.FieldError{Path: "repairer", Code: stay.CodeTooLong})
	}
	if in.Note != nil && utf8.RuneCountInString(*in.Note) > maxTicketNote {
		errs = append(errs, stay.FieldError{Path: "note", Code: stay.CodeTooLong})
	}
	if len(errs) > 0 {
		return stay.NewValidationError(errs)
	}
	return nil
}

// change applies a request to a ticket and moves the room as needed; it returns the columns to store.
func (m *Maintenance) change(ctx context.Context, tx Tx, c Caller, rec TicketRecord, in UpdateTicketInput) (TicketChange, error) {
	if rec.Status == ticket.Done {
		return doneCostChange(rec, in)
	}
	status := rec.Status
	if in.Status != nil {
		if !ticket.CanMove(rec.Status, *in.Status) {
			return TicketChange{}, ErrTicketStatus
		}
		status = *in.Status
	}
	ch := TicketChange{ID: rec.ID, Status: status, RoomLocked: rec.RoomLocked, ExpectedDoneOn: rec.ExpectedDoneOn,
		PartsCost: rec.PartsCost, LabourCost: rec.LabourCost, Repairer: rec.Repairer, Note: rec.Note}
	if in.ExpectedDoneOn != nil {
		d := dayOf(*in.ExpectedDoneOn, time.UTC)
		ch.ExpectedDoneOn = &d
	}
	ch.PartsCost, ch.LabourCost = pick(in.PartsCost, ch.PartsCost), pick(in.LabourCost, ch.LabourCost)
	ch.Repairer, ch.Note = pickStr(in.Repairer, ch.Repairer), pickStr(in.Note, ch.Note)
	wantLocked := rec.RoomLocked
	if in.RoomLocked != nil {
		wantLocked = *in.RoomLocked
	}
	if status == ticket.Done {
		wantLocked = false
		now := storedTime(m.clock.Now())
		ch.CompletedAt, ch.CompletedBy = &now, c.UserID
	}
	if err := m.moveRoom(ctx, tx, rec, wantLocked); err != nil {
		return TicketChange{}, err
	}
	ch.RoomLocked = wantLocked
	return ch, nil
}

// moveRoom locks or reopens the ticket's room when its lock changes. The room reopens only when no other open ticket keeps it locked.
func (m *Maintenance) moveRoom(ctx context.Context, tx Tx, rec TicketRecord, wantLocked bool) error {
	if wantLocked == rec.RoomLocked {
		return nil
	}
	rm, ok, err := m.repo.LockRoom(ctx, tx, rec.RoomID)
	if err != nil || !ok {
		return fmt.Errorf("lock room (found %v): %w", ok, err)
	}
	if wantLocked {
		return m.lockRoom(ctx, tx, rm)
	}
	others, err := m.repo.OtherLocks(ctx, tx, rec.RoomID, rec.ID)
	if err != nil {
		return fmt.Errorf("other locks: %w", err)
	}
	if others > 0 {
		return nil
	}
	if err := m.repo.Reopen(ctx, tx, rec.RoomID); err != nil {
		return fmt.Errorf("reopen room: %w", err)
	}
	return nil
}

// doneCostChange is what a finished ticket still allows: its costs (the owner may price it late or correct it) and nothing else.
func doneCostChange(rec TicketRecord, in UpdateTicketInput) (TicketChange, error) {
	if in.Status != nil || in.RoomLocked != nil || in.ExpectedDoneOn != nil || in.Repairer != nil || in.Note != nil {
		return TicketChange{}, ErrTicketDone
	}
	return TicketChange{ID: rec.ID, Status: ticket.Done, RoomLocked: false, ExpectedDoneOn: rec.ExpectedDoneOn, Repairer: rec.Repairer, Note: rec.Note,
		CompletedAt: rec.CompletedAt, PartsCost: pick(in.PartsCost, rec.PartsCost), LabourCost: pick(in.LabourCost, rec.LabourCost)}, nil
}

func pick(in, cur *int64) *int64 {
	if in != nil {
		return in
	}
	return cur
}

func pickStr(in, cur *string) *string {
	if in != nil {
		return in
	}
	return cur
}

// ReportUsage tells the owner a room looks used although no stay is active (SG-401). It writes only the alert.
func (m *Maintenance) ReportUsage(ctx context.Context, c Caller, roomID, retryID, note string) (AlertRow, error) {
	const op = "reportRoomUsage"
	if err := m.checkRole(op, c); err != nil {
		return AlertRow{}, err
	}
	if err := checkKey(retryID); err != nil {
		return AlertRow{}, err
	}
	if utf8.RuneCountInString(note) > maxUsageNote {
		return AlertRow{}, stay.NewValidationError([]stay.FieldError{{Path: "note", Code: stay.CodeTooLong}})
	}
	var out AlertRow
	err := m.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		rm, err := m.room(ctx, tx, op, c, roomID)
		if err != nil {
			return err
		}
		route := "POST /v1/rooms/" + roomID + "/usage-reports"
		oc, err := m.idem.Begin(ctx, tx, route, retryID, RequestHash([]byte(fmt.Sprintf(`{"note":%q}`, note))))
		if err != nil {
			return err
		}
		if oc.Replay {
			return json.Unmarshal(oc.Body, &out)
		}
		details := map[string]string{}
		if n := strings.TrimSpace(note); n != "" {
			details["note"] = n
		}
		d := AlertDraft{ID: m.ids.New(alertIDPrefix), Kind: AlertUnusedRoomReport, RoomCode: rm.Code, By: c.UserID, Details: details}
		if err := m.alerts.Raise(ctx, tx, d); err != nil {
			return fmt.Errorf("raise alert: %w", err)
		}
		out = AlertRow{ID: d.ID, Kind: d.Kind, RoomCode: d.RoomCode, Details: details, CreatedAt: storedTime(m.clock.Now())}
		body, err := json.Marshal(out)
		if err != nil {
			return fmt.Errorf("encode response: %w", err)
		}
		return m.idem.Complete(ctx, tx, route, retryID, statusCreated, body)
	})
	return out, err
}
