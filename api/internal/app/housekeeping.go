package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
)

const auditRoomCleaned = "room.cleaned"

// ErrRoomNotToClean: the room is occupied or under maintenance, so there is nothing to complete.
var ErrRoomNotToClean = room.ErrNotToClean

// CleanRoom is a room as the housekeeping use cases see it; CleaningSince is the last check-out.
type CleanRoom struct {
	ID, Code, BuildingID, Status string
	CleaningSince                time.Time
}

// HousekeepingRepo filters by the tenant of the Tx.
type HousekeepingRepo interface {
	ToClean(ctx context.Context, tx Tx) ([]CleanRoom, error)
	// LockRoom takes the room row lock so two completions serialize.
	LockRoom(ctx context.Context, tx Tx, roomID string) (CleanRoom, bool, error)
	// MarkClean returns ErrRoomNotToClean when the room was not TO_CLEAN: the backstop.
	MarkClean(ctx context.Context, tx Tx, roomID string) error
}

// HousekeepingTask is a room waiting to be cleaned; its id is the room id (no task table in FAST MODE).
type HousekeepingTask struct {
	ID, RoomCode, BuildingID string
	Done                     bool
	CreatedAt                time.Time
	CompletedAt              *time.Time
}

// Housekeeping holds the housekeeping use cases (SG-401, FAST MODE).
type Housekeeping struct {
	uow    UnitOfWork
	repo   HousekeepingRepo
	levels BuildingLevels
	audit  AuditWriter
	ids    IDGenerator
	clock  Clock
	authz  access.Authorizer
}

func NewHousekeeping(uow UnitOfWork, repo HousekeepingRepo, levels BuildingLevels, audit AuditWriter, ids IDGenerator, clock Clock) *Housekeeping {
	return &Housekeeping{uow: uow, repo: repo, levels: levels, audit: audit, ids: ids, clock: clock}
}

// ListTasks returns the TO_CLEAN rooms in buildings the caller may see.
func (h *Housekeeping) ListTasks(ctx context.Context, c Caller) ([]HousekeepingTask, error) {
	if err := h.authz.Check("listHousekeepingTasks", c.Role, access.EDIT); err != nil {
		return nil, err
	}
	var out []HousekeepingTask
	err := h.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		rooms, err := h.repo.ToClean(ctx, tx)
		if err != nil {
			return fmt.Errorf("rooms to clean: %w", err)
		}
		ids := make([]string, len(rooms))
		for i, r := range rooms {
			ids[i] = r.BuildingID
		}
		lv, err := h.levels.Levels(ctx, c, ids)
		if err != nil {
			return fmt.Errorf("building levels: %w", err)
		}
		out = []HousekeepingTask{}
		for _, r := range rooms {
			if lv[r.BuildingID] >= access.VIEW {
				out = append(out, taskOf(r, false, nil))
			}
		}
		return nil
	})
	return out, err
}

// Complete marks a room clean: TO_CLEAN becomes VACANT. Repeating it on a VACANT room answers DONE again.
func (h *Housekeeping) Complete(ctx context.Context, c Caller, taskID, retryID string) (HousekeepingTask, error) {
	const op = "completeHousekeepingTask"
	if err := h.authz.Check(op, c.Role, access.EDIT); err != nil {
		return HousekeepingTask{}, err
	}
	if err := checkKey(retryID); err != nil {
		return HousekeepingTask{}, err
	}
	var out HousekeepingTask
	err := h.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		r, ok, err := h.repo.LockRoom(ctx, tx, taskID)
		if err != nil {
			return fmt.Errorf("lock room: %w", err)
		}
		if !ok {
			return ErrNotFound
		}
		lv, err := h.levels.Levels(ctx, c, []string{r.BuildingID})
		if err != nil {
			return fmt.Errorf("building levels: %w", err)
		}
		if err := h.authz.Check(op, c.Role, lv[r.BuildingID]); err != nil {
			return err
		}
		now := storedTime(h.clock.Now())
		if room.Status(r.Status) == room.StatusToClean {
			if err := h.repo.MarkClean(ctx, tx, r.ID); err != nil {
				return err
			}
			if err := h.auditClean(ctx, tx, c, r); err != nil {
				return err
			}
		} else if _, err := room.Clean(room.Status(r.Status)); err != nil {
			return err
		}
		out = taskOf(r, true, &now)
		return nil
	})
	return out, err
}

func (h *Housekeeping) auditClean(ctx context.Context, tx Tx, c Caller, r CleanRoom) error {
	raw, err := json.Marshal(map[string]string{"roomId": r.ID, "status": string(room.StatusVacant)})
	if err != nil {
		return fmt.Errorf("encode audit: %w", err)
	}
	e := AuditEntry{ID: h.ids.New(auditIDPrefix), ActorID: c.UserID, Action: auditRoomCleaned, EntityType: "room", EntityID: r.ID, After: raw}
	if err := h.audit.Append(ctx, tx, e); err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	return nil
}

func taskOf(r CleanRoom, done bool, at *time.Time) HousekeepingTask {
	return HousekeepingTask{ID: r.ID, RoomCode: r.Code, BuildingID: r.BuildingID, Done: done,
		CreatedAt: r.CleaningSince.UTC(), CompletedAt: at}
}
