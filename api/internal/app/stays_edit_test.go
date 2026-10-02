package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/pricing"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

const (
	editStay = "st_1"
	bldgB    = "b2"
	roomB    = "rm2"
)

// fakeEditRepo keeps one tenant's stays and rooms in memory and enforces the same backstops as the adapter.
type fakeEditRepo struct {
	stays map[string]StayRecord
	rooms map[string]CheckInRoom
	edits []StayEdit
	locks []string
}

func (r *fakeEditRepo) LockStay(_ context.Context, _ Tx, id string) (StayRecord, bool, error) {
	rec, ok := r.stays[id]
	return rec, ok, nil
}

func (r *fakeEditRepo) Room(_ context.Context, _ Tx, id string, forUpdate bool) (CheckInRoom, bool, error) {
	if forUpdate {
		r.locks = append(r.locks, id)
	}
	rm, ok := r.rooms[id]
	return rm, ok, nil
}

func (r *fakeEditRepo) Timezone(context.Context, Tx) (string, error) { return zoneName, nil }

func (r *fakeEditRepo) FirstRecordedCheckIn(context.Context, Tx, string) (time.Time, bool, error) {
	for _, e := range r.edits {
		if e.Kind == editKindCheckIn {
			return *e.OldCheckInAt, true, nil
		}
	}
	return time.Time{}, false, nil
}

func (r *fakeEditRepo) UpdateCheckIn(_ context.Context, _ Tx, id string, at time.Time) error {
	rec := r.stays[id]
	rec.CheckInAt = at
	r.stays[id] = rec
	return nil
}

func (r *fakeEditRepo) MoveStay(_ context.Context, _ Tx, m MovedStay) error {
	rec := r.stays[m.StayID]
	to := r.rooms[m.ToRoomID]
	rec.RoomID, rec.RoomCode, rec.BuildingID, rec.RentalType, rec.RatePlanSnapshot = to.ID, to.Code, to.BuildingID, m.RentalType, m.RatePlanSnapshot
	r.stays[m.StayID] = rec
	return nil
}

func (r *fakeEditRepo) setStatus(id string, s room.Status) {
	rm := r.rooms[id]
	rm.StoredStatus = string(s)
	r.rooms[id] = rm
}

func (r *fakeEditRepo) MarkRoomOccupied(_ context.Context, _ Tx, id string) error {
	r.setStatus(id, room.StatusOccupied)
	return nil
}

func (r *fakeEditRepo) MarkRoomToClean(_ context.Context, _ Tx, id string) error {
	r.setStatus(id, room.StatusToClean)
	return nil
}

func (r *fakeEditRepo) InsertEdit(_ context.Context, _ Tx, e StayEdit) error {
	r.edits = append(r.edits, e)
	return nil
}

type fakeAlerts struct{ raised []AlertDraft }

func (a *fakeAlerts) Raise(_ context.Context, _ Tx, d AlertDraft) error {
	a.raised = append(a.raised, d)
	return nil
}

type editEnv struct {
	s      *StayEdits
	repo   *fakeEditRepo
	alerts *fakeAlerts
	audit  *fakeAudit
	levels *fakeLevels
	idem   *fakeIdem
	caller Caller
}

// the stay was checked in 90 minutes before t0 (the fixed clock), HOURLY, in room 101.
func newEditEnv(t *testing.T) *editEnv {
	t.Helper()
	plan := stayPlan()
	checkIn := t0.Add(-90 * time.Minute)
	e := &editEnv{
		repo: &fakeEditRepo{
			stays: map[string]StayRecord{editStay: {ID: editStay, RoomID: roomA, RoomCode: "101", BuildingID: bldgA, RentalType: "HOURLY",
				Status: "ACTIVE", GuestName: "Guest", GuestPhone: "0900000000", Deposit: 100_000, CheckInAt: checkIn, RatePlanSnapshot: plan.Snapshot()}},
			rooms: map[string]CheckInRoom{
				roomA: {ID: roomA, Code: "101", BuildingID: bldgA, StoredStatus: string(room.StatusOccupied), RatePlan: plan.Snapshot()},
				roomB: {ID: roomB, Code: "201", BuildingID: bldgB, StoredStatus: string(room.StatusVacant), RatePlan: premiumPlan().Snapshot()},
			}},
		alerts: &fakeAlerts{}, audit: &fakeAudit{},
		levels: &fakeLevels{levels: map[string]access.Level{bldgA: access.EDIT, bldgB: access.EDIT}},
		idem:   &fakeIdem{m: map[string]idemState{}},
		caller: Caller{TenantID: tenantA, UserID: "u1", Role: access.RoleReceptionist},
	}
	uow := &rollbackUoW{idem: e.idem}
	e.s = NewStayEdits(uow, e.repo, e.levels, fakeEnc{}, e.idem, e.audit, e.alerts, &seqIDs{}, fixedClock{t0})
	return e
}

// premiumPlan prices every rental type higher than stayPlan.
func premiumPlan() pricing.RatePlan {
	p := stayPlan()
	p.Hourly.FirstHour, p.Hourly.ExtraHour = 150_000, 50_000
	return p
}

func (e *editEnv) wantQuote(t *testing.T, plan pricing.RatePlan, rental pricing.RentalType, checkIn time.Time) int64 {
	t.Helper()
	q, err := pricing.QuoteRunning(plan, rental, checkIn, t0, mustLoc(t))
	if err != nil {
		t.Fatal(err)
	}
	return q.Total.Int64()
}

func validEdit(newAt time.Time) EditCheckInInput {
	return EditCheckInInput{NewCheckInAt: newAt, ReasonCode: "WRONG_TIME", Note: "typed the wrong time"}
}

// SG-801 AC1: the edit changes the price, and the price comes from pricing.
func TestEditCheckIn_ChangesPriceThroughPricing_SG801_AC1(t *testing.T) {
	e := newEditEnv(t)
	before := e.repo.stays[editStay].CheckInAt
	earlier := before.Add(-2 * time.Hour)
	d, err := e.s.EditCheckIn(context.Background(), e.caller, editStay, "k1", validEdit(earlier))
	if err != nil {
		t.Fatal(err)
	}
	if want := e.wantQuote(t, stayPlan(), pricing.RentalHourly, earlier); d.Quote.StayAmount != want || !d.CheckInAt.Equal(earlier) {
		t.Fatalf("stay amount %d checkIn %v, want %d at %v", d.Quote.StayAmount, d.CheckInAt, want, earlier)
	}
	if d.Quote.StayAmount == e.wantQuote(t, stayPlan(), pricing.RentalHourly, before) {
		t.Fatal("the edit did not change the price")
	}
}

// SG-801 AC2: later than 60 minutes after the recorded time, or in the future, is a 422 and changes nothing.
func TestEditCheckIn_OutOfRangeIs422_SG801_AC2(t *testing.T) {
	e := newEditEnv(t)
	before := e.repo.stays[editStay].CheckInAt
	for name, at := range map[string]time.Time{"61 minutes later": before.Add(61 * time.Minute), "in the future": t0.Add(time.Minute)} {
		_, err := e.s.EditCheckIn(context.Background(), e.caller, editStay, "k-"+name, validEdit(at))
		var ve *stay.ValidationError
		if !errors.As(err, &ve) || ve.Errors[0].Path != "newCheckInAt" {
			t.Errorf("%s: got %v", name, err)
		}
	}
	if !e.repo.stays[editStay].CheckInAt.Equal(before) || len(e.repo.edits) != 0 || len(e.alerts.raised) != 0 {
		t.Fatal("a rejected edit changed state")
	}
}

// SG-801 AC3: every accepted edit raises STAY_TIME_EDITED with the old and new time, and is audited without the note.
func TestEditCheckIn_RaisesAlertAndAudit_SG801_AC3(t *testing.T) {
	e := newEditEnv(t)
	before := e.repo.stays[editStay].CheckInAt
	if _, err := e.s.EditCheckIn(context.Background(), e.caller, editStay, "k1", validEdit(before.Add(30*time.Minute))); err != nil {
		t.Fatal(err)
	}
	if len(e.alerts.raised) != 1 {
		t.Fatalf("alerts: %+v", e.alerts.raised)
	}
	a := e.alerts.raised[0]
	if a.Kind != AlertStayTimeEdited || a.StayID != editStay || a.RoomCode != "101" || a.By != "u1" ||
		a.Details["oldTime"] == "" || a.Details["newTime"] == "" || a.Details["oldTime"] == a.Details["newTime"] {
		t.Fatalf("alert: %+v", a)
	}
	if len(e.audit.entries) != 1 || e.audit.entries[0].Action != auditCheckInEdited || containsText(e.audit.entries[0].After, "typed the wrong") {
		t.Fatalf("audit: %+v", e.audit.entries)
	}
}

// Repeated corrections cannot creep later than 60 minutes after the time first recorded.
func TestEditCheckIn_LimitFollowsFirstRecordedTime_SG801_AC2(t *testing.T) {
	e := newEditEnv(t)
	first := e.repo.stays[editStay].CheckInAt
	ctx := context.Background()
	if _, err := e.s.EditCheckIn(ctx, e.caller, editStay, "k1", validEdit(first.Add(50*time.Minute))); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.EditCheckIn(ctx, e.caller, editStay, "k2", validEdit(first.Add(70*time.Minute))); err == nil {
		t.Fatal("a second edit moved check-in past 60 minutes after the first recorded time")
	}
}

func TestEditCheckIn_ReplayAndGuards_SG801(t *testing.T) {
	e := newEditEnv(t)
	ctx := context.Background()
	in := validEdit(e.repo.stays[editStay].CheckInAt.Add(-time.Hour))
	first, err := e.s.EditCheckIn(ctx, e.caller, editStay, "k1", in)
	if err != nil {
		t.Fatal(err)
	}
	again, err := e.s.EditCheckIn(ctx, e.caller, editStay, "k1", in)
	if err != nil || again.CheckInAt != first.CheckInAt || len(e.alerts.raised) != 1 {
		t.Fatalf("replay: %v %+v alerts=%d", err, again, len(e.alerts.raised))
	}
	if _, err := e.s.EditCheckIn(ctx, e.caller, editStay, "k1", validEdit(in.NewCheckInAt.Add(time.Minute))); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Errorf("same key, other body: %v", err)
	}
	if _, err := e.s.EditCheckIn(ctx, e.caller, "nope", "k2", in); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown stay: %v", err)
	}
	hk := e.caller
	hk.Role = access.RoleHousekeeping
	if _, err := e.s.EditCheckIn(ctx, hk, editStay, "k3", in); !errors.Is(err, access.ErrRoleForbidden) {
		t.Errorf("housekeeping: %v", err)
	}
	e.levels.levels[bldgA] = access.VIEW
	if _, err := e.s.EditCheckIn(ctx, e.caller, editStay, "k4", in); !errors.Is(err, access.ErrBuildingForbidden) {
		t.Errorf("view only: %v", err)
	}
	e.levels.levels[bldgA] = access.EDIT
	rec := e.repo.stays[editStay]
	rec.Status = "CHECKED_OUT"
	e.repo.stays[editStay] = rec
	if _, err := e.s.EditCheckIn(ctx, e.caller, editStay, "k5", in); !errors.Is(err, stay.ErrNotActive) {
		t.Errorf("checked out: %v", err)
	}
}

// SG-801 AC4: a move keeps check-in time and extras, prices the whole stay with the new room type and cleans up the old room.
func TestMoveStay_RepricesAndReleasesOldRoom_SG801_AC4(t *testing.T) {
	e := newEditEnv(t)
	rec := e.repo.stays[editStay]
	rec.Extras = []ExtraRecord{{ServiceCode: "WATER", Quantity: 2, UnitAmount: 10_000}}
	e.repo.stays[editStay] = rec
	d, err := e.s.MoveStay(context.Background(), e.caller, editStay, "m1", MoveStayInput{ToRoomID: roomB, RentalType: "HOURLY"})
	if err != nil {
		t.Fatal(err)
	}
	if want := e.wantQuote(t, premiumPlan(), pricing.RentalHourly, rec.CheckInAt); d.Quote.StayAmount != want {
		t.Fatalf("stay amount %d, want %d (new room type)", d.Quote.StayAmount, want)
	}
	if !d.CheckInAt.Equal(rec.CheckInAt) || d.RoomCode != "201" || d.RoomID != roomB || len(d.Extras) != 1 || d.Quote.ExtrasAmount != 20_000 {
		t.Fatalf("moved stay: %+v", d)
	}
	if e.repo.rooms[roomA].StoredStatus != string(room.StatusToClean) || e.repo.rooms[roomB].StoredStatus != string(room.StatusOccupied) {
		t.Fatalf("rooms: %s %s", e.repo.rooms[roomA].StoredStatus, e.repo.rooms[roomB].StoredStatus)
	}
	if len(e.repo.locks) != 2 || e.repo.locks[0] > e.repo.locks[1] {
		t.Fatalf("rooms must be locked in id order: %v", e.repo.locks)
	}
}

func TestMoveStay_Refusals_SG801_AC4(t *testing.T) {
	e := newEditEnv(t)
	ctx := context.Background()
	check := func(name string, want error, to string, rental string) {
		t.Helper()
		_, err := e.s.MoveStay(ctx, e.caller, editStay, "k-"+name, MoveStayInput{ToRoomID: to, RentalType: rental})
		if !errors.Is(err, want) {
			t.Errorf("%s: got %v want %v", name, err, want)
		}
		if e.repo.stays[editStay].RoomID != roomA {
			t.Fatalf("%s: the stay moved", name)
		}
	}
	e.repo.setStatus(roomB, room.StatusOccupied)
	check("occupied target", room.ErrNotVacant, roomB, "HOURLY")
	e.repo.setStatus(roomB, room.StatusVacant)
	check("unknown target", ErrNotFound, "nope", "HOURLY")
	check("bad rental type", pricing.ErrUnknownRentalType, roomB, "WEEKLY")
	var ve *stay.ValidationError
	if _, err := e.s.MoveStay(ctx, e.caller, editStay, "k-same", MoveStayInput{ToRoomID: roomA, RentalType: "HOURLY"}); !errors.As(err, &ve) {
		t.Errorf("same room: %v", err)
	}
	e.levels.levels[bldgB] = access.VIEW
	check("no edit on target building", access.ErrBuildingForbidden, roomB, "HOURLY")
}

func containsText(raw []byte, s string) bool { return strings.Contains(string(raw), s) }
