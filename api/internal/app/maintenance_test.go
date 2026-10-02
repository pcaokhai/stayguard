package app

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

type fakeTicketRepo struct {
	seq     int
	tickets map[string]*TicketRecord
	rooms   map[string]*RoomLock
}

func (r *fakeTicketRepo) Timezone(context.Context, Tx) (string, error) { return zoneName, nil }
func (r *fakeTicketRepo) NextSeq(context.Context, Tx) (int, error)     { r.seq++; return r.seq, nil }
func (r *fakeTicketRepo) Insert(_ context.Context, _ Tx, n NewTicket) error {
	rm := r.rooms[n.RoomID]
	r.tickets[n.ID] = &TicketRecord{ID: n.ID, Code: n.Code, RoomID: n.RoomID, RoomCode: rm.Code, BuildingID: rm.BuildingID, Category: n.Category,
		Description: n.Description, Status: "NEW", RoomLocked: n.RoomLocked, ReportedAt: n.ReportedAt, ReportedBy: "Lan"}
	return nil
}
func (r *fakeTicketRepo) Lock(_ context.Context, _ Tx, id string) (bool, error) {
	_, ok := r.tickets[id]
	return ok, nil
}
func (r *fakeTicketRepo) ByID(_ context.Context, _ Tx, id string) (TicketRecord, bool, error) {
	t, ok := r.tickets[id]
	if !ok {
		return TicketRecord{}, false, nil
	}
	return *t, true, nil
}
func (r *fakeTicketRepo) List(_ context.Context, _ Tx, status string) ([]TicketRecord, error) {
	var out []TicketRecord
	for _, t := range r.tickets {
		if status == "" || t.Status == status {
			out = append(out, *t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out, nil
}
func (r *fakeTicketRepo) Update(_ context.Context, _ Tx, c TicketChange) error {
	t := r.tickets[c.ID]
	t.Status, t.RoomLocked, t.ExpectedDoneOn, t.PartsCost, t.LabourCost, t.Repairer, t.Note, t.CompletedAt =
		c.Status, c.RoomLocked, c.ExpectedDoneOn, c.PartsCost, c.LabourCost, c.Repairer, c.Note, c.CompletedAt
	return nil
}
func (r *fakeTicketRepo) LockRoom(_ context.Context, _ Tx, id string) (RoomLock, bool, error) {
	rm, ok := r.rooms[id]
	if !ok {
		return RoomLock{}, false, nil
	}
	return *rm, true, nil
}
func (r *fakeTicketRepo) SetMaintenance(_ context.Context, _ Tx, id string) error {
	r.rooms[id].Status = string(room.StatusMaintenance)
	return nil
}
func (r *fakeTicketRepo) Reopen(_ context.Context, _ Tx, id string) error {
	r.rooms[id].Status = string(room.StatusVacant)
	return nil
}
func (r *fakeTicketRepo) OtherLocks(_ context.Context, _ Tx, roomID, ticketID string) (int, error) {
	n := 0
	for id, t := range r.tickets {
		if id != ticketID && t.RoomID == roomID && t.RoomLocked && t.Status != "DONE" {
			n++
		}
	}
	return n, nil
}

type ticketEnv struct {
	m      *Maintenance
	repo   *fakeTicketRepo
	alerts *fakeAlerts
	levels *fakeLevels
	lan    Caller
}

func newTicketEnv() *ticketEnv {
	repo := &fakeTicketRepo{tickets: map[string]*TicketRecord{}, rooms: map[string]*RoomLock{
		"r_vac": {ID: "r_vac", Code: "A101", BuildingID: bldgA, Status: "VACANT"},
		"r_occ": {ID: "r_occ", Code: "A102", BuildingID: bldgA, Status: "OCCUPIED"},
	}}
	e := &ticketEnv{repo: repo, alerts: &fakeAlerts{}, levels: &fakeLevels{levels: map[string]access.Level{bldgA: access.EDIT}},
		lan: Caller{TenantID: tenantA, UserID: "u_lan", Role: access.RoleReceptionist}}
	idem := &fakeIdem{m: map[string]idemState{}}
	e.m = NewMaintenance(&rollbackUoW{idem: idem}, repo, e.levels, idem, &fakeAudit{}, e.alerts, &seqIDs{}, fixedClock{t0})
	return e
}

func damage(sev string) DamageInput {
	return DamageInput{Category: "TV", Description: "remote is missing", Severity: sev}
}

func (e *ticketEnv) report(t *testing.T, roomID, key, sev string) TicketView {
	t.Helper()
	v, err := e.m.ReportDamage(context.Background(), e.lan, roomID, key, damage(sev))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestReportDamage_LockRoomAndAlert_SG1201(t *testing.T) {
	e := newTicketEnv()
	v := e.report(t, "r_vac", "k1", "LOCK_ROOM")
	if v.Code != "BT-001" || v.Status != "NEW" || !v.RoomLocked || v.RoomCode != "A101" || e.repo.rooms["r_vac"].Status != "MAINTENANCE" {
		t.Fatalf("ticket %+v room %s", v, e.repo.rooms["r_vac"].Status)
	}
	if a := e.alerts.raised; len(a) != 1 || a[0].Kind != AlertDamageReported || a[0].RoomCode != "A101" || a[0].Details["ticket"] != "BT-001" {
		t.Fatalf("alerts: %+v", a)
	}
	if again := e.report(t, "r_vac", "k1", "LOCK_ROOM"); again.ID != v.ID || len(e.repo.tickets) != 1 || len(e.alerts.raised) != 1 {
		t.Fatalf("replay created more: %+v", again)
	}
	if next := e.report(t, "r_vac", "k2", "STILL_RENTABLE"); next.Code != "BT-002" || next.RoomLocked {
		t.Fatalf("second ticket: %+v", next)
	}
}

func TestReportDamage_CannotLockARoomWithAGuest_SG1201(t *testing.T) {
	e := newTicketEnv()
	if _, err := e.m.ReportDamage(context.Background(), e.lan, "r_occ", "k1", damage("LOCK_ROOM")); !errors.Is(err, ErrRoomOccupied) {
		t.Fatalf("lock with a guest: %v", err)
	}
	if len(e.repo.tickets) != 0 || len(e.alerts.raised) != 0 || e.repo.seq != 0 || e.repo.rooms["r_occ"].Status != "OCCUPIED" {
		t.Fatal("a refused lock wrote something")
	}
	if v := e.report(t, "r_occ", "k2", "STILL_RENTABLE"); v.RoomLocked || e.repo.rooms["r_occ"].Status != "OCCUPIED" {
		t.Fatalf("still rentable: %+v", v)
	}
}

func TestMaintenance_RolesAndBuildings_SG1201(t *testing.T) {
	e := newTicketEnv()
	ctx := context.Background()
	hk := e.lan
	hk.Role = access.RoleHousekeeping
	if _, err := e.m.ReportDamage(ctx, hk, "r_vac", "k1", damage("STILL_RENTABLE")); err != nil {
		t.Errorf("housekeeping may report damage: %v", err)
	}
	if _, err := e.m.ListTickets(ctx, hk, ""); !errors.Is(err, access.ErrRoleForbidden) {
		t.Errorf("housekeeping lists tickets: %v", err)
	}
	if _, err := e.m.ListTickets(ctx, e.lan, ""); !errors.Is(err, access.ErrRoleForbidden) {
		t.Errorf("receptionist lists tickets: %v", err)
	}
	e.levels.levels[bldgA] = access.VIEW
	if _, err := e.m.ReportDamage(ctx, e.lan, "r_vac", "k2", damage("STILL_RENTABLE")); !errors.Is(err, access.ErrBuildingForbidden) {
		t.Errorf("VIEW only: %v", err)
	}
	if _, err := e.m.ReportDamage(ctx, e.lan, "nope", "k3", damage("STILL_RENTABLE")); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown room: %v", err)
	}
	var ve *stay.ValidationError
	if _, err := e.m.ReportDamage(ctx, e.lan, "r_vac", "k4", DamageInput{Category: "SOFA", Severity: "LOCK_ROOM"}); !errors.As(err, &ve) {
		t.Errorf("invalid report: %v", err)
	}
}

func TestUpdateTicket_CostsDoneAndRoom_SG1201(t *testing.T) {
	e := newTicketEnv()
	ctx := context.Background()
	first := e.report(t, "r_vac", "k1", "LOCK_ROOM")
	second := e.report(t, "r_vac", "k2", "LOCK_ROOM")
	owner := Caller{TenantID: tenantA, UserID: "u_o", Role: access.RoleOwner}
	manager := Caller{TenantID: tenantA, UserID: "u_m", Role: access.RoleManager}
	n := func(v int64) *int64 { return &v }
	st := func(s string) *string { return &s }

	if _, err := e.m.UpdateTicket(ctx, manager, first.ID, UpdateTicketInput{PartsCost: n(100_000)}); !errors.Is(err, access.ErrRoleForbidden) {
		t.Fatalf("manager sets costs: %v", err)
	}
	if v, err := e.m.UpdateTicket(ctx, manager, first.ID, UpdateTicketInput{Status: st("IN_REPAIR"), Repairer: st("Mr Ba")}); err != nil || v.Status != "IN_REPAIR" || *v.Repairer != "Mr Ba" {
		t.Fatalf("manager moves it: %+v %v", v, err)
	}
	v, err := e.m.UpdateTicket(ctx, owner, first.ID, UpdateTicketInput{PartsCost: n(150_000), LabourCost: n(50_000)})
	if err != nil || v.TotalCost == nil || *v.TotalCost != 200_000 {
		t.Fatalf("owner costs: %+v %v", v, err)
	}
	if _, err := e.m.UpdateTicket(ctx, owner, first.ID, UpdateTicketInput{Status: st("NEW")}); !errors.Is(err, ErrTicketStatus) {
		t.Fatalf("back to NEW: %v", err)
	}
	// Done: stamped, unlocked, but the room stays locked while the second ticket still locks it.
	v, err = e.m.UpdateTicket(ctx, manager, first.ID, UpdateTicketInput{Status: st("DONE")})
	if err != nil || v.CompletedAt == nil || v.RoomLocked || e.repo.rooms["r_vac"].Status != "MAINTENANCE" {
		t.Fatalf("done: %+v %v room %s", v, err, e.repo.rooms["r_vac"].Status)
	}
	if _, err := e.m.UpdateTicket(ctx, owner, first.ID, UpdateTicketInput{Note: st("late")}); !errors.Is(err, ErrTicketDone) {
		t.Fatalf("edit a done ticket: %v", err)
	}
	if _, err := e.m.UpdateTicket(ctx, manager, second.ID, UpdateTicketInput{Status: st("DONE")}); err != nil || e.repo.rooms["r_vac"].Status != "VACANT" {
		t.Fatalf("last lock released: %v room %s", err, e.repo.rooms["r_vac"].Status)
	}
	if _, err := e.m.UpdateTicket(ctx, owner, "nope", UpdateTicketInput{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown ticket: %v", err)
	}
	if _, err := e.m.UpdateTicket(ctx, owner, first.ID, UpdateTicketInput{PartsCost: n(-1)}); err == nil {
		t.Fatal("negative cost accepted")
	}
}

func TestUpdateTicket_LockAndUnlockByHand_SG1201(t *testing.T) {
	e := newTicketEnv()
	ctx := context.Background()
	owner := Caller{TenantID: tenantA, UserID: "u_o", Role: access.RoleOwner}
	yes, no := true, false
	occ := e.report(t, "r_occ", "k1", "STILL_RENTABLE")
	if _, err := e.m.UpdateTicket(ctx, owner, occ.ID, UpdateTicketInput{RoomLocked: &yes}); !errors.Is(err, ErrRoomOccupied) {
		t.Fatalf("lock with a guest: %v", err)
	}
	vac := e.report(t, "r_vac", "k2", "STILL_RENTABLE")
	if v, err := e.m.UpdateTicket(ctx, owner, vac.ID, UpdateTicketInput{RoomLocked: &yes}); err != nil || !v.RoomLocked || e.repo.rooms["r_vac"].Status != "MAINTENANCE" {
		t.Fatalf("lock: %+v %v", v, err)
	}
	day := time.Date(2026, 10, 9, 15, 30, 0, 0, time.UTC)
	if v, err := e.m.UpdateTicket(ctx, owner, vac.ID, UpdateTicketInput{RoomLocked: &no, ExpectedDoneOn: &day}); err != nil || v.RoomLocked ||
		e.repo.rooms["r_vac"].Status != "VACANT" || v.ExpectedDoneOn == nil || v.ExpectedDoneOn.Hour() != 0 {
		t.Fatalf("unlock: %+v %v", v, err)
	}
}

type fakeLedger struct{ lines map[string]AutoExpense }

func (l *fakeLedger) PostAuto(_ context.Context, _ Tx, e AutoExpense) error {
	if _, ok := l.lines[e.Source+"/"+e.RefID]; !ok {
		l.lines[e.Source+"/"+e.RefID] = e
	}
	return nil
}
func (l *fakeLedger) UpsertAuto(_ context.Context, _ Tx, e AutoExpense) error {
	l.lines[e.Source+"/"+e.RefID] = e
	return nil
}
func (l *fakeLedger) RemoveAuto(_ context.Context, _ Tx, source, ref string) error {
	delete(l.lines, source+"/"+ref)
	return nil
}

// Follow-up 4: the owner can enter or change the cost of a DONE ticket; the MAINTENANCE expense follows and each change is audited.
func TestUpdateTicket_CostOnDoneTicketUpdatesExpense_FU4(t *testing.T) {
	e := newTicketEnv()
	ledger := &fakeLedger{lines: map[string]AutoExpense{}}
	audit := &fakeAudit{}
	idem := &fakeIdem{m: map[string]idemState{}}
	e.m = NewMaintenance(&rollbackUoW{idem: idem}, e.repo, e.levels, idem, audit, e.alerts, &seqIDs{}, fixedClock{t0}).WithExpenses(ledger)
	ctx := context.Background()
	owner := Caller{TenantID: tenantA, UserID: "u_o", Role: access.RoleOwner}
	manager := Caller{TenantID: tenantA, UserID: "u_m", Role: access.RoleManager}
	n := func(v int64) *int64 { return &v }
	st := func(s string) *string { return &s }

	v := e.report(t, "r_vac", "k1", "STILL_RENTABLE")
	if _, err := e.m.UpdateTicket(ctx, owner, v.ID, UpdateTicketInput{Status: st("DONE")}); err != nil || len(ledger.lines) != 0 {
		t.Fatalf("done without a cost: %v lines=%v", err, ledger.lines)
	}
	if _, err := e.m.UpdateTicket(ctx, manager, v.ID, UpdateTicketInput{PartsCost: n(100_000)}); !errors.Is(err, access.ErrRoleForbidden) {
		t.Fatalf("manager prices a done ticket: %v", err)
	}
	if _, err := e.m.UpdateTicket(ctx, owner, v.ID, UpdateTicketInput{Note: st("late")}); !errors.Is(err, ErrTicketDone) {
		t.Fatalf("other edits stay locked: %v", err)
	}
	got, err := e.m.UpdateTicket(ctx, owner, v.ID, UpdateTicketInput{PartsCost: n(300_000), LabourCost: n(100_000)})
	line := ledger.lines["MAINTENANCE/"+v.ID]
	if err != nil || got.TotalCost == nil || *got.TotalCost != 400_000 || line.Amount != 400_000 || line.Month != "2026-01" {
		t.Fatalf("priced after done: %+v %v line=%+v", got, err, line)
	}
	if _, err := e.m.UpdateTicket(ctx, owner, v.ID, UpdateTicketInput{LabourCost: n(250_000)}); err != nil || ledger.lines["MAINTENANCE/"+v.ID].Amount != 550_000 {
		t.Fatalf("changed: %v line=%+v", err, ledger.lines["MAINTENANCE/"+v.ID])
	}
	if _, err := e.m.UpdateTicket(ctx, owner, v.ID, UpdateTicketInput{PartsCost: n(0), LabourCost: n(0)}); err != nil || len(ledger.lines) != 0 {
		t.Fatalf("cleared: %v lines=%v", err, ledger.lines)
	}
	var costAudits int
	for _, a := range audit.entries {
		if a.Action == auditTicketCost && a.EntityID == v.ID {
			costAudits++
		}
	}
	if costAudits != 3 {
		t.Fatalf("cost audits: %d, want 3", costAudits)
	}
}

// Follow-up 5: photos are not supported yet, so a damage report that names some is refused, not quietly stored without them.
func TestReportDamage_RejectsPhotoAssetIDs_FU5(t *testing.T) {
	e := newTicketEnv()
	in := damage("STILL_RENTABLE")
	in.PhotoAssetIDs = []string{"as_1"}
	_, err := e.m.ReportDamage(context.Background(), e.lan, "r_vac", "k1", in)
	var ve *stay.ValidationError
	if !errors.As(err, &ve) || ve.Errors[0].Path != "photoAssetIds" || len(e.repo.tickets) != 0 || len(e.alerts.raised) != 0 {
		t.Fatalf("photos: %v tickets=%d alerts=%d", err, len(e.repo.tickets), len(e.alerts.raised))
	}
	in.PhotoAssetIDs = []string{}
	if _, err := e.m.ReportDamage(context.Background(), e.lan, "r_vac", "k2", in); err != nil {
		t.Fatalf("an empty list is no photos: %v", err)
	}
}
