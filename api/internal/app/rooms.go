package app

import (
	"context"
	"fmt"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
)

type BuildingView struct {
	ID, Code, Name string
	Level          access.Level
	Counts         room.Counts
}

type StayView struct {
	ID             string
	RentalType     room.RentalType
	CheckInAt      time.Time
	GuestName      string
	ElapsedMinutes int
	RunningTotal   int64
}

type RoomView struct {
	ID, Code, BuildingID string
	Floor                int
	UnitTypeCode         string
	UnitTypeName         LocalizedName
	Status               room.Status
	Note                 *string
	ActiveStay           *StayView
}

// Rooms holds the read use cases of the room map (SG-201).
type Rooms struct {
	uow    UnitOfWork
	repo   RoomRepo
	levels BuildingLevels
	quoter StayQuoter
	clock  Clock
	authz  access.Authorizer
}

func NewRooms(uow UnitOfWork, repo RoomRepo, levels BuildingLevels, quoter StayQuoter, clock Clock) *Rooms {
	return &Rooms{uow: uow, repo: repo, levels: levels, quoter: quoter, clock: clock}
}

// checkRole applies only the role half of the rule: EDIT satisfies any building level.
func (s *Rooms) checkRole(op string, c Caller) error { return s.authz.Check(op, c.Role, access.EDIT) }

// checkBuilding runs after the existence check, so a foreign id is a 404 before it can be a 403.
func (s *Rooms) checkBuilding(ctx context.Context, op string, c Caller, buildingID string) error {
	lv, err := s.levels.Levels(ctx, c, []string{buildingID})
	if err != nil {
		return fmt.Errorf("building levels: %w", err)
	}
	return s.authz.Check(op, c.Role, lv[buildingID]) // missing id is NONE
}

func (s *Rooms) zone(ctx context.Context, tx Tx) (*time.Location, error) {
	name, err := s.repo.Timezone(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("tenant timezone: %w", err)
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("tenant timezone %q: %w", name, err)
	}
	return loc, nil
}

func (s *Rooms) ListBuildings(ctx context.Context, c Caller) ([]BuildingView, error) {
	const op = "listBuildings"
	if err := s.checkRole(op, c); err != nil {
		return nil, err
	}
	var out []BuildingView
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		buildings, err := s.repo.Buildings(ctx, tx)
		if err != nil {
			return fmt.Errorf("buildings: %w", err)
		}
		rows, err := s.repo.Rooms(ctx, tx, RoomFilter{})
		if err != nil {
			return fmt.Errorf("rooms: %w", err)
		}
		loc, err := s.zone(ctx, tx)
		if err != nil {
			return err
		}
		byBuilding, err := s.statusesByBuilding(rows, loc)
		if err != nil {
			return err
		}
		ids := make([]string, len(buildings))
		for i, b := range buildings {
			ids[i] = b.ID
		}
		lv, err := s.levels.Levels(ctx, c, ids)
		if err != nil {
			return fmt.Errorf("building levels: %w", err)
		}
		for _, b := range buildings {
			if l := lv[b.ID]; l != access.VIEW && l != access.EDIT {
				continue
			}
			out = append(out, BuildingView{b.ID, b.Code, b.Name, lv[b.ID], room.Count(byBuilding[b.ID])})
		}
		return nil
	})
	return out, err
}

func (s *Rooms) statusesByBuilding(rows []RoomRow, loc *time.Location) (map[string][]room.Status, error) {
	now := s.clock.Now()
	out := map[string][]room.Status{}
	for _, r := range rows {
		st, _, err := deriveRoom(r, now, loc)
		if err != nil {
			return nil, err
		}
		out[r.BuildingID] = append(out[r.BuildingID], st)
	}
	return out, nil
}

func (s *Rooms) ListRooms(ctx context.Context, c Caller, buildingID string, status *room.Status) ([]RoomView, error) {
	const op = "listRooms"
	if err := s.checkRole(op, c); err != nil {
		return nil, err
	}
	var out []RoomView
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		if err := s.requireBuilding(ctx, op, c, tx, buildingID); err != nil {
			return err
		}
		rows, err := s.repo.Rooms(ctx, tx, RoomFilter{BuildingID: buildingID})
		if err != nil {
			return fmt.Errorf("rooms: %w", err)
		}
		loc, err := s.zone(ctx, tx)
		if err != nil {
			return err
		}
		out, err = s.views(ctx, inBuilding(rows, buildingID), status, loc)
		return err
	})
	return out, err
}

func (s *Rooms) requireBuilding(ctx context.Context, op string, c Caller, tx Tx, buildingID string) error {
	buildings, err := s.repo.Buildings(ctx, tx)
	if err != nil {
		return fmt.Errorf("buildings: %w", err)
	}
	for _, b := range buildings {
		if b.ID == buildingID {
			return s.checkBuilding(ctx, op, c, buildingID)
		}
	}
	return ErrNotFound
}

func (s *Rooms) GetRoom(ctx context.Context, c Caller, roomID string) (RoomView, error) {
	const op = "getRoom"
	if err := s.checkRole(op, c); err != nil {
		return RoomView{}, err
	}
	var out RoomView
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		rows, err := s.repo.Rooms(ctx, tx, RoomFilter{RoomID: roomID})
		if err != nil {
			return fmt.Errorf("rooms: %w", err)
		}
		if roomID == "" || len(rows) == 0 || rows[0].ID != roomID {
			return ErrNotFound
		}
		if err := s.checkBuilding(ctx, op, c, rows[0].BuildingID); err != nil {
			return err
		}
		loc, err := s.zone(ctx, tx)
		if err != nil {
			return err
		}
		views, err := s.views(ctx, rows[:1], nil, loc)
		if err != nil {
			return err
		}
		out = views[0]
		return nil
	})
	return out, err
}

// inBuilding drops rows of other buildings, so an adapter that ignores the filter cannot leak them.
func inBuilding(rows []RoomRow, buildingID string) []RoomRow {
	out := make([]RoomRow, 0, len(rows))
	for _, r := range rows {
		if r.BuildingID == buildingID {
			out = append(out, r)
		}
	}
	return out
}

// views derives every row, applies the optional status filter, then quotes only the rooms kept.
func (s *Rooms) views(ctx context.Context, rows []RoomRow, filter *room.Status, loc *time.Location) ([]RoomView, error) {
	now := s.clock.Now()
	out := make([]RoomView, 0, len(rows))
	for _, r := range rows {
		st, timing, err := deriveRoom(r, now, loc)
		if err != nil {
			return nil, err
		}
		if filter != nil && st != *filter {
			continue
		}
		v := RoomView{ID: r.ID, Code: r.Code, BuildingID: r.BuildingID, Floor: r.Floor,
			UnitTypeCode: r.UnitTypeCode, UnitTypeName: r.UnitTypeName, Status: st, Note: r.Note}
		if timing != nil {
			if v.ActiveStay, err = s.stayView(ctx, r.Stay, *timing, now, loc); err != nil {
				return nil, err
			}
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *Rooms) stayView(ctx context.Context, st *StayRow, t room.StayTiming, now time.Time, loc *time.Location) (*StayView, error) {
	total, err := s.quoter.RunningTotal(ctx, st.RatePlanSnapshot, t.RentalType, t.CheckInAt, now, loc)
	if err != nil {
		return nil, fmt.Errorf("running total: %w", err)
	}
	return &StayView{ID: st.ID, RentalType: t.RentalType, CheckInAt: st.CheckInAt, GuestName: st.GuestName,
		ElapsedMinutes: room.ElapsedMinutes(st.CheckInAt, now), RunningTotal: total}, nil
}

// deriveRoom fails closed on an unreadable stored status, rental type or snapshot.
func deriveRoom(r RoomRow, now time.Time, loc *time.Location) (room.Status, *room.StayTiming, error) {
	stored, err := room.ParseStored(r.StoredStatus)
	if err != nil {
		return "", nil, err
	}
	var timing *room.StayTiming
	if r.Stay != nil && stored == room.StatusOccupied { // a stay on any other status is stale
		rt, err := room.ParseRentalType(r.Stay.RentalType)
		if err != nil {
			return "", nil, err
		}
		t, err := room.ParseTiming(rt, r.Stay.CheckInAt, r.Stay.RatePlanSnapshot)
		if err != nil {
			return "", nil, err
		}
		timing = &t
	}
	st, err := room.Derive(stored, timing, now, loc)
	return st, timing, err
}
