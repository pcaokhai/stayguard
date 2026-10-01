package app

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/pricing"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

func create(e *stayEnv, key string, in CreateStayInput) (StayDetail, bool, error) {
	return e.s.CreateStay(context.Background(), e.caller, roomA, key, in)
}

func TestCreateStay_SG203_AC1(t *testing.T) {
	e := newStayEnv(t)
	v, replayed, err := create(e, "k1", goodInput())
	if err != nil || replayed {
		t.Fatalf("err=%v replayed=%v", err, replayed)
	}
	if len(e.repo.inserted) != 1 || e.repo.rooms[tenantA][roomA].StoredStatus != string(room.StatusOccupied) {
		t.Fatal("expected one stay and an OCCUPIED room")
	}
	ns := e.repo.inserted[0]
	if !ns.CheckInAt.Equal(t0) || v.CheckInAt != t0 {
		t.Fatalf("time must come from the clock: %v %v", ns.CheckInAt, v.CheckInAt)
	}
	plan := stayPlan()
	if string(ns.RatePlanSnapshot) != string(plan.Snapshot()) || ns.RatePlanSchema != 1 {
		t.Fatalf("snapshot not canonical: %s", ns.RatePlanSnapshot)
	}
	if back, err := pricing.ParseRatePlan(ns.RatePlanSnapshot); err != nil || back != plan {
		t.Fatalf("snapshot does not decode to the plan: %v", err)
	}
	if v.Status != "ACTIVE" || v.PricingVersion != 3 || v.RoomCode != "101" || v.ID != ns.ID {
		t.Fatalf("view: %+v", v)
	}
	if len(e.audit.entries) != 1 {
		t.Fatalf("audit entries: %d", len(e.audit.entries))
	}
	a := e.audit.entries[0]
	raw, _ := json.Marshal(a)
	for _, banned := range []string{"Marker Guest", "900 000", idMarker} {
		if strings.Contains(string(raw), banned) {
			t.Fatalf("audit holds personal data %q: %s", banned, raw)
		}
	}
	if a.Action != "stay.check_in" || a.EntityType != "stay" || a.EntityID != ns.ID || a.ActorID != "u1" ||
		!strings.Contains(string(a.After), `"deposit":200000`) {
		t.Fatalf("audit: %+v %s", a, a.After)
	}
}

func TestCreateStayRoomNotVacant_SG203_AC2(t *testing.T) {
	for _, st := range []room.Status{room.StatusOccupied, room.StatusToClean, room.StatusMaintenance} {
		e := newStayEnv(t)
		e.setStatus(st)
		_, _, err := create(e, "k1", goodInput())
		if !errors.Is(err, room.ErrNotVacant) || len(e.repo.inserted) != 0 || e.idem.completes != 0 {
			t.Fatalf("%s: err=%v inserted=%d", st, err, len(e.repo.inserted))
		}
	}
	e := newStayEnv(t)
	e.repo.markErr = room.ErrNotVacant // the backstop: a concurrent winner took the room
	if _, _, err := create(e, "k1", goodInput()); !errors.Is(err, room.ErrNotVacant) || e.idem.completes != 0 {
		t.Fatalf("backstop: %v", err)
	}
}

func TestCreateStayIdempotent_SG203_AC3(t *testing.T) {
	e := newStayEnv(t)
	first, _, err := create(e, "k1", goodInput())
	if err != nil {
		t.Fatal(err)
	}
	again, replayed, err := create(e, "k1", goodInput())
	if err != nil || !replayed || !reflect.DeepEqual(first, again) {
		t.Fatalf("replay: err=%v replayed=%v equal=%v", err, replayed, reflect.DeepEqual(first, again))
	}
	if len(e.repo.inserted) != 1 || len(e.audit.entries) != 1 {
		t.Fatalf("replay had effects: %d stays, %d audits", len(e.repo.inserted), len(e.audit.entries))
	}
	other := "OTHERID999"
	for name, mod := range map[string]func(*CreateStayInput){
		"deposit": func(in *CreateStayInput) { in.Deposit++ },
		"name":    func(in *CreateStayInput) { in.GuestName += " " }, // raw, untrimmed body is compared
		"id":      func(in *CreateStayInput) { in.IDNumber = &other },
		"no id":   func(in *CreateStayInput) { in.IDNumber = nil },
		"rental":  func(in *CreateStayInput) { in.RentalType = "DAILY" },
		"phone":   func(in *CreateStayInput) { in.GuestPhone = "123456" },
	} {
		in := goodInput()
		mod(&in)
		if _, _, err := create(e, "k1", in); !errors.Is(err, ErrIdempotencyKeyReused) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if len(e.repo.inserted) != 1 {
		t.Fatal("a reused key must not insert")
	}
}

func TestCreateStayIdempotentScope_SG203_AC3(t *testing.T) {
	e := newStayEnv(t)
	e.repo.rooms[tenantA]["rm2"] = CheckInRoom{ID: "rm2", Code: "102", BuildingID: bldgA, StoredStatus: "VACANT", RatePlan: stayPlan().Snapshot()}
	if _, _, err := create(e, "k1", goodInput()); err != nil {
		t.Fatal(err)
	}
	_, replayed, err := e.s.CreateStay(context.Background(), e.caller, "rm2", "k1", goodInput())
	if err != nil || replayed || len(e.repo.inserted) != 2 {
		t.Fatalf("same key on another room must be independent: %v %v", err, replayed)
	}

	// A failed attempt leaves no key: the retry after the room is free succeeds.
	f := newStayEnv(t)
	f.setStatus(room.StatusOccupied)
	if _, _, err := create(f, "k9", goodInput()); !errors.Is(err, room.ErrNotVacant) {
		t.Fatal(err)
	}
	f.setStatus(room.StatusVacant)
	if _, replayed, err := create(f, "k9", goodInput()); err != nil || replayed {
		t.Fatalf("retry: %v %v", err, replayed)
	}

	// Replay passes the building check first.
	g := newStayEnv(t)
	if _, _, err := create(g, "k1", goodInput()); err != nil {
		t.Fatal(err)
	}
	g.levels.levels[bldgA] = access.NONE
	if _, _, err := create(g, "k1", goodInput()); !errors.Is(err, access.ErrBuildingForbidden) {
		t.Fatalf("replay without access: %v", err)
	}
}

func TestCreateStayValidation_SG203_AC1(t *testing.T) {
	bad := map[string]func(*CreateStayInput){
		"name":    func(in *CreateStayInput) { in.GuestName = " " },
		"phone":   func(in *CreateStayInput) { in.GuestPhone = "12" },
		"id":      func(in *CreateStayInput) { id := "ab"; in.IDNumber = &id },
		"deposit": func(in *CreateStayInput) { in.Deposit = -1 },
		"max":     func(in *CreateStayInput) { in.Deposit = stay.MaxDeposit + 1 },
	}
	for name, mod := range bad {
		e := newStayEnv(t)
		in := goodInput()
		mod(&in)
		_, _, err := create(e, "k1", in)
		var ve *stay.ValidationError
		if !errors.As(err, &ve) || e.repo.calls != 0 || len(e.idem.hashes) != 0 {
			t.Fatalf("%s: err=%v calls=%d", name, err, e.repo.calls)
		}
	}
	e := newStayEnv(t)
	in := goodInput()
	in.RentalType = "WEEKLY"
	if _, _, err := create(e, "k1", in); !errors.Is(err, pricing.ErrUnknownRentalType) || e.repo.calls != 0 {
		t.Fatalf("rental: %v", err)
	}
	// An unreadable stored plan is a server error, not a client validation error.
	e = newStayEnv(t)
	rm := e.repo.rooms[tenantA][roomA]
	rm.RatePlan = []byte(`{"version":0}`)
	e.repo.rooms[tenantA][roomA] = rm
	_, _, err := create(e, "k1", goodInput())
	var ve *stay.ValidationError
	if err == nil || errors.As(err, &ve) || errors.Is(err, ErrNotFound) || len(e.repo.inserted) != 0 {
		t.Fatalf("bad stored plan: %v", err)
	}
}

func TestCreateStayAccess_SG203_AC1(t *testing.T) {
	e := newStayEnv(t)
	e.caller.Role = access.RoleHousekeeping
	if _, _, err := create(e, "k1", goodInput()); !errors.Is(err, access.ErrRoleForbidden) || e.repo.calls != 0 {
		t.Fatalf("role: %v calls=%d", err, e.repo.calls)
	}
	for _, lv := range []access.Level{access.NONE, access.VIEW} {
		e = newStayEnv(t)
		e.levels.levels[bldgA] = lv
		if _, _, err := create(e, "k1", goodInput()); !errors.Is(err, access.ErrBuildingForbidden) || len(e.repo.inserted) != 0 {
			t.Fatalf("level %v: %v", lv, err)
		}
	}
	e = newStayEnv(t)
	_, _, foreign := e.s.CreateStay(context.Background(), e.caller, "rmB", "k1", goodInput())
	_, _, unknown := e.s.CreateStay(context.Background(), e.caller, "nope", "k1", goodInput())
	if !errors.Is(foreign, ErrNotFound) || foreign.Error() != unknown.Error() || !errors.Is(unknown, ErrNotFound) {
		t.Fatalf("foreign=%v unknown=%v", foreign, unknown)
	}
}

func TestCreateStayIdempotencyKey_SG203_AC3(t *testing.T) {
	for name, key := range map[string]string{"empty": "", "too long": strings.Repeat("k", maxIdempotencyKeyBytes+1)} {
		e := newStayEnv(t)
		_, _, err := create(e, key, goodInput())
		if !errors.Is(err, ErrInvalidIdempotencyKey) || e.repo.calls != 0 || len(e.idem.hashes) != 0 || len(e.uow.tenants) != 0 {
			t.Fatalf("%s: err=%v", name, err)
		}
		if strings.Contains(err.Error(), key) && key != "" {
			t.Fatal("error echoes the key")
		}
	}
	e := newStayEnv(t)
	if _, _, err := create(e, strings.Repeat("k", maxIdempotencyKeyBytes), goodInput()); err != nil {
		t.Fatalf("a key at the limit is valid: %v", err)
	}
}

func TestCreateStayHashEmptyId_SG203_AC3(t *testing.T) {
	hashOf := func(id *string) string {
		e := newStayEnv(t)
		in := goodInput()
		in.IDNumber = id
		if _, _, err := create(e, "k1", in); err != nil {
			t.Fatal(err)
		}
		return e.idem.hashes[0]
	}
	empty, blank, real := "", "   ", "REALID12345"
	none := hashOf(nil)
	if hashOf(&empty) != none || hashOf(&blank) != none {
		t.Fatal("null, empty and whitespace-only id numbers must hash alike")
	}
	if hashOf(&real) == none {
		t.Fatal("a real id number must change the hash")
	}
}
