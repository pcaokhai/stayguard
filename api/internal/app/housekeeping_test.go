package app

import (
	"context"
	"errors"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
)

type fakeCleanRepo struct{ rooms map[string]*CleanRoom }

func (r *fakeCleanRepo) ToClean(context.Context, Tx) ([]CleanRoom, error) {
	var out []CleanRoom
	for _, c := range r.rooms {
		if c.Status == "TO_CLEAN" {
			out = append(out, *c)
		}
	}
	return out, nil
}

func (r *fakeCleanRepo) LockRoom(_ context.Context, _ Tx, id string) (CleanRoom, bool, error) {
	c, ok := r.rooms[id]
	if !ok {
		return CleanRoom{}, false, nil
	}
	return *c, true, nil
}

func (r *fakeCleanRepo) MarkClean(_ context.Context, _ Tx, id string) error {
	if r.rooms[id].Status != "TO_CLEAN" {
		return ErrRoomNotToClean
	}
	r.rooms[id].Status = "VACANT"
	return nil
}

func TestCompleteHousekeepingTask_A3(t *testing.T) {
	repo := &fakeCleanRepo{rooms: map[string]*CleanRoom{
		"r1": {ID: "r1", Code: "A103", BuildingID: "b1", Status: "TO_CLEAN", CleaningSince: t0},
		"r2": {ID: "r2", Code: "A104", BuildingID: "b1", Status: "OCCUPIED"},
	}}
	audit := &fakeAudit{}
	h := NewHousekeeping(&fakeUoW{}, repo, &fakeLevels{levels: map[string]access.Level{"b1": access.EDIT}}, audit, &seqIDs{}, fixedClock{t0})
	ctx := context.Background()
	hk := Caller{TenantID: "tn", UserID: "u", Role: access.RoleHousekeeping}

	h.levels = &fakeLevels{levels: map[string]access.Level{"b1": access.VIEW}}
	if _, err := h.Complete(ctx, Caller{TenantID: "tn", Role: access.RoleReceptionist}, "r1", "k"); !errors.Is(err, access.ErrBuildingForbidden) {
		t.Fatalf("receptionist with VIEW: %v, want building forbidden", err)
	}
	h.levels = &fakeLevels{levels: map[string]access.Level{"b1": access.EDIT}}
	if tasks, err := h.ListTasks(ctx, hk); err != nil || len(tasks) != 1 || tasks[0].RoomCode != "A103" || tasks[0].Done {
		t.Fatalf("list: %+v %v", tasks, err)
	}
	for i := 0; i < 2; i++ { // the second call is the retry
		got, err := h.Complete(ctx, hk, "r1", "k")
		if err != nil || !got.Done || got.CompletedAt == nil || repo.rooms["r1"].Status != "VACANT" {
			t.Fatalf("complete #%d: %+v %v status=%s", i, got, err, repo.rooms["r1"].Status)
		}
	}
	if len(audit.entries) != 1 {
		t.Errorf("audit entries = %d, want 1 (the retry changes nothing)", len(audit.entries))
	}
	if _, err := h.Complete(ctx, hk, "r2", "k"); !errors.Is(err, room.ErrNotToClean) {
		t.Errorf("occupied room: %v, want not-to-clean", err)
	}
	if _, err := h.Complete(ctx, hk, "nope", "k"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown: %v, want not found", err)
	}
	if tasks, _ := h.ListTasks(ctx, hk); len(tasks) != 0 {
		t.Errorf("tasks after completing = %d", len(tasks))
	}
}

// SG-1201: owner, manager, receptionist and housekeeping can all mark a room clean when they have EDIT on its building.
func TestCompleteHousekeepingTask_EveryRoleWithEdit_SG1201(t *testing.T) {
	for _, role := range []access.Role{access.RoleOwner, access.RoleManager, access.RoleReceptionist, access.RoleHousekeeping} {
		repo := &fakeCleanRepo{rooms: map[string]*CleanRoom{"r1": {ID: "r1", Code: "A103", BuildingID: "b1", Status: "TO_CLEAN", CleaningSince: t0}}}
		audit := &fakeAudit{}
		h := NewHousekeeping(&fakeUoW{}, repo, &fakeLevels{levels: map[string]access.Level{"b1": access.EDIT}}, audit, &seqIDs{}, fixedClock{t0})
		got, err := h.Complete(context.Background(), Caller{TenantID: "tn", UserID: "u_" + string(role), Role: role}, "r1", "k")
		if err != nil || !got.Done || repo.rooms["r1"].Status != "VACANT" || len(audit.entries) != 1 || audit.entries[0].ActorID != "u_"+string(role) {
			t.Errorf("%s: %+v %v audit=%+v", role, got, err, audit.entries)
		}
	}
}
