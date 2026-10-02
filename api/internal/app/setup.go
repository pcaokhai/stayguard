package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/money"
	"github.com/pcaokhai/stayguard/api/internal/domain/pricing"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

const (
	maxRoomsPerRequest = 200
	maxItemNameLen     = 80
	maxRoomCodeLen     = 20
	maxServiceCodeLen  = 64
	maxUnitLen         = 20
	statusVacant       = "VACANT"
	statusToClean      = "TO_CLEAN"
	statusMaintenance  = "MAINTENANCE"
	kindOpening        = "OPENING"
	kindIn             = "IN"
	movementPrefix     = "sm"
	floorPrefix        = "fl"
	roomPrefix         = "un"
	buildingPrefix     = "bd"
	servicePrefix      = "sv"
)

var (
	buildingCodePattern = regexp.MustCompile(`^[A-Z]{1,3}$`)
	roomFeatures        = map[string]bool{"DOUBLE_BED": true, "TWIN_BEDS": true, "WINDOW": true, "BATHTUB": true}
	serviceCodeChars    = regexp.MustCompile(`[^A-Z0-9]+`)
)

// Setup is buildings, rooms, rate plans and items: the owner's one-time and occasional configuration.
// Stock never changes here except through movements (opening, restock).
type Setup struct {
	uow   UnitOfWork
	repo  SetupRepo
	idem  IdempotencyStore
	audit AuditWriter
	ids   IDGenerator
	clock Clock
	authz access.Authorizer
	// alerts is optional: without it a stocktake raises no alert.
	alerts AlertWriter
}

func NewSetup(uow UnitOfWork, repo SetupRepo, idem IdempotencyStore, audit AuditWriter, ids IDGenerator, clock Clock) *Setup {
	return &Setup{uow: uow, repo: repo, idem: idem, audit: audit, ids: ids, clock: clock}
}

// idempotent runs effect once per (route, key): a replay decodes the stored response instead.
func idempotent[T any](ctx context.Context, idem IdempotencyStore, tx Tx, route, key string, request any, effect func() (T, error)) (T, error) {
	var zero T
	body, err := json.Marshal(request)
	if err != nil {
		return zero, fmt.Errorf("encode request: %w", err)
	}
	oc, err := idem.Begin(ctx, tx, route, key, RequestHash(body))
	if err != nil {
		return zero, err
	}
	if oc.Replay {
		var out T
		if err = json.Unmarshal(oc.Body, &out); err != nil {
			return zero, fmt.Errorf("stored response: %w", err)
		}
		return out, nil
	}
	out, err := effect()
	if err != nil {
		return zero, err
	}
	stored, err := json.Marshal(out)
	if err != nil {
		return zero, fmt.Errorf("encode response: %w", err)
	}
	return out, idem.Complete(ctx, tx, route, key, statusCreated, stored)
}

func (s *Setup) validKey(key string) error {
	if key == "" || len(key) > maxIdempotencyKeyBytes {
		return ErrInvalidIdempotencyKey
	}
	return nil
}

func (s *Setup) record(ctx context.Context, tx Tx, c Caller, action, entity, entityID string, after map[string]any) error {
	e := AuditEntry{ID: s.ids.New(auditPrefix), ActorID: c.UserID, Action: action, EntityType: entity, EntityID: entityID}
	if after != nil {
		raw, err := json.Marshal(after)
		if err != nil {
			return err
		}
		e.After = raw
	}
	return s.audit.Append(ctx, tx, e)
}

// ---- buildings, floors and rooms ----

// BuildingInput is the body of createBuilding.
type BuildingInput struct {
	Code, Name, UnitTypeCode string
	Floors, RoomsPerFloor    int
}

// CreateBuilding adds a building with generated floors and rooms. A new building starts with no staff access.
func (s *Setup) CreateBuilding(ctx context.Context, c Caller, key string, in BuildingInput) (BuildingView, error) {
	if err := s.authz.Check("createBuilding", c.Role, access.EDIT); err != nil {
		return BuildingView{}, err
	}
	if err := s.validKey(key); err != nil {
		return BuildingView{}, err
	}
	switch {
	case !buildingCodePattern.MatchString(in.Code):
		return BuildingView{}, &ValidationError{"code", "one to three capital letters"}
	case strings.TrimSpace(in.Name) == "" || len(in.Name) > maxNameLen:
		return BuildingView{}, &ValidationError{"name", "required, at most 80 characters"}
	case in.Floors < 1 || in.Floors > 30:
		return BuildingView{}, &ValidationError{"floors", "between 1 and 30"}
	case in.RoomsPerFloor < 0 || in.RoomsPerFloor > 50:
		return BuildingView{}, &ValidationError{"roomsPerFloor", "between 0 and 50"}
	}
	var out BuildingView
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		var err error
		out, err = idempotent(ctx, s.idem, tx, "POST /v1/owner/buildings", key, in, func() (BuildingView, error) { return s.createBuilding(ctx, tx, c, in) })
		return err
	})
	return out, err
}

func (s *Setup) createBuilding(ctx context.Context, tx Tx, c Caller, in BuildingInput) (BuildingView, error) {
	ut, ok, err := s.repo.UnitTypeByCode(ctx, tx, in.UnitTypeCode)
	if err != nil || !ok {
		return BuildingView{}, errOr(err, &ValidationError{"unitTypeCode", "unknown room type"})
	}
	prop, ok, err := s.repo.FirstProperty(ctx, tx)
	if err != nil || !ok {
		return BuildingView{}, errOr(err, ErrNotFound)
	}
	id := s.ids.New(buildingPrefix)
	if err = s.repo.InsertBuilding(ctx, tx, id, prop, in.Code, in.Name); err != nil {
		return BuildingView{}, err
	}
	var codes []string
	for f := 1; f <= in.Floors; f++ {
		for n := 1; n <= in.RoomsPerFloor; n++ {
			codes = append(codes, fmt.Sprintf("%s%d%02d", in.Code, f, n))
		}
	}
	if err = s.checkFree(ctx, tx, codes); err != nil {
		return BuildingView{}, err
	}
	for f := 1; f <= in.Floors; f++ {
		fid := s.ids.New(floorPrefix)
		if err = s.repo.InsertFloor(ctx, tx, fid, id, f, nil); err != nil {
			return BuildingView{}, err
		}
		for n := 1; n <= in.RoomsPerFloor; n++ {
			r := NewRoom{ID: s.ids.New(roomPrefix), BuildingID: id, FloorID: fid, UnitTypeID: ut.ID, Code: fmt.Sprintf("%s%d%02d", in.Code, f, n), Status: statusVacant, Attributes: []byte("{}")}
			if err = s.repo.InsertRoom(ctx, tx, r); err != nil {
				return BuildingView{}, err
			}
		}
	}
	if err = s.record(ctx, tx, c, "BUILDING_CREATED", "building", id, map[string]any{"floors": in.Floors, "rooms": len(codes)}); err != nil {
		return BuildingView{}, err
	}
	return s.buildingView(ctx, tx, BuildingRow{ID: id, Code: in.Code, Name: in.Name})
}

func (s *Setup) buildingView(ctx context.Context, tx Tx, b BuildingRow) (BuildingView, error) {
	counts, err := s.repo.StatusCounts(ctx, tx, b.ID)
	// ponytail: overdue is derived from running stays and is counted as occupied here; listBuildings has the exact split.
	return BuildingView{ID: b.ID, Code: b.Code, Name: b.Name, Level: access.EDIT, Counts: counts}, err
}

// checkFree is ErrConflict when any of the room codes already exists.
func (s *Setup) checkFree(ctx context.Context, tx Tx, codes []string) error {
	if len(codes) == 0 {
		return nil
	}
	taken, err := s.repo.ExistingCodes(ctx, tx, codes)
	if err != nil {
		return err
	}
	if len(taken) > 0 {
		return ErrConflict
	}
	return nil
}

func (s *Setup) UpdateBuilding(ctx context.Context, c Caller, id string, name *string) (BuildingView, error) {
	if err := s.authz.Check("updateBuilding", c.Role, access.EDIT); err != nil {
		return BuildingView{}, err
	}
	if name != nil && (strings.TrimSpace(*name) == "" || len(*name) > maxNameLen) {
		return BuildingView{}, &ValidationError{"name", "required, at most 80 characters"}
	}
	var out BuildingView
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		b, ok, err := s.repo.Building(ctx, tx, id)
		if err != nil || !ok {
			return errOr(err, ErrNotFound)
		}
		if name != nil && *name != b.Name {
			if err = s.repo.RenameBuilding(ctx, tx, id, *name); err != nil {
				return err
			}
			b.Name = *name
			if err = s.record(ctx, tx, c, "BUILDING_RENAMED", "building", id, nil); err != nil {
				return err
			}
		}
		out, err = s.buildingView(ctx, tx, b)
		return err
	})
	return out, err
}

// FloorRooms is the optional room range of createFloor.
type FloorRooms struct {
	Count                   int
	StartCode, UnitTypeCode string
}

// FloorInput is the body of createFloor.
type FloorInput struct {
	Name  string
	Rooms *FloorRooms
}

// CreateFloor adds the next floor of a building, optionally with a run of rooms.
func (s *Setup) CreateFloor(ctx context.Context, c Caller, key, buildingID string, in FloorInput) ([]RoomView, error) {
	if err := s.authz.Check("createFloor", c.Role, access.EDIT); err != nil {
		return nil, err
	}
	if err := s.validKey(key); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Name) == "" || len(in.Name) > maxNameLen {
		return nil, &ValidationError{"name", "required, at most 80 characters"}
	}
	if r := in.Rooms; r != nil && (r.Count < 1 || r.Count > 50) {
		return nil, &ValidationError{"rooms.count", "between 1 and 50"}
	}
	var out []RoomView
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		var err error
		route := "POST /v1/owner/buildings/" + buildingID + "/floors"
		out, err = idempotent(ctx, s.idem, tx, route, key, in, func() ([]RoomView, error) { return s.createFloor(ctx, tx, c, buildingID, in) })
		return err
	})
	return out, err
}

func (s *Setup) createFloor(ctx context.Context, tx Tx, c Caller, buildingID string, in FloorInput) ([]RoomView, error) {
	if _, ok, err := s.repo.Building(ctx, tx, buildingID); err != nil || !ok {
		return nil, errOr(err, ErrNotFound)
	}
	level, err := s.repo.MaxFloorLevel(ctx, tx, buildingID)
	if err != nil {
		return nil, err
	}
	level++
	fid := s.ids.New(floorPrefix)
	name := in.Name
	if err = s.repo.InsertFloor(ctx, tx, fid, buildingID, level, &name); err != nil {
		return nil, err
	}
	out := []RoomView{}
	if in.Rooms != nil {
		codes, err := codeRange(in.Rooms.StartCode, in.Rooms.Count)
		if err != nil {
			return nil, err
		}
		out, err = s.insertRooms(ctx, tx, buildingID, fid, level, in.Rooms.UnitTypeCode, codes, nil, statusVacant)
		if err != nil {
			return nil, err
		}
	}
	if err = s.record(ctx, tx, c, "FLOOR_CREATED", "floor", fid, map[string]any{"level": level, "rooms": len(out)}); err != nil {
		return nil, err
	}
	return out, nil
}

// RoomsInput is the body of createRooms: one code (From equal To) or a range.
type RoomsInput struct {
	BuildingID, FloorID, From, To, UnitTypeCode string
	Features                                    []string
	AvailableNow                                bool
}

func (s *Setup) CreateRooms(ctx context.Context, c Caller, key string, in RoomsInput) ([]RoomView, error) {
	if err := s.authz.Check("createRooms", c.Role, access.EDIT); err != nil {
		return nil, err
	}
	if err := s.validKey(key); err != nil {
		return nil, err
	}
	if err := validateFeatures(in.Features); err != nil {
		return nil, err
	}
	codes, err := codeSpan(in.From, in.To)
	if err != nil {
		return nil, err
	}
	var out []RoomView
	err = s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		var err error
		out, err = idempotent(ctx, s.idem, tx, "POST /v1/owner/rooms", key, in, func() ([]RoomView, error) {
			f, ok, err := s.repo.Floor(ctx, tx, in.FloorID)
			if err != nil || !ok || f.BuildingID != in.BuildingID {
				return nil, errOr(err, &ValidationError{"floorId", "unknown floor of this building"})
			}
			status := statusVacant
			if !in.AvailableNow {
				status = statusToClean // not ready: it waits for cleaning (design: "Turn off if the room is not ready")
			}
			views, err := s.insertRooms(ctx, tx, in.BuildingID, in.FloorID, f.Level, in.UnitTypeCode, codes, in.Features, status)
			if err != nil {
				return nil, err
			}
			return views, s.record(ctx, tx, c, "ROOMS_CREATED", "building", in.BuildingID, map[string]any{"count": len(views)})
		})
		return err
	})
	return out, err
}

func (s *Setup) insertRooms(ctx context.Context, tx Tx, buildingID, floorID string, level int, typeCode string, codes, features []string, status string) ([]RoomView, error) {
	ut, ok, err := s.repo.UnitTypeByCode(ctx, tx, typeCode)
	if err != nil || !ok {
		return nil, errOr(err, &ValidationError{"unitTypeCode", "unknown room type"})
	}
	if err = s.checkFree(ctx, tx, codes); err != nil {
		return nil, err
	}
	attrs, err := attributesWith(nil, features, nil)
	if err != nil {
		return nil, err
	}
	out := make([]RoomView, 0, len(codes))
	for _, code := range codes {
		r := NewRoom{ID: s.ids.New(roomPrefix), BuildingID: buildingID, FloorID: floorID, UnitTypeID: ut.ID, Code: code, Status: status, Attributes: attrs}
		if err = s.repo.InsertRoom(ctx, tx, r); err != nil {
			return nil, err
		}
		out = append(out, RoomView{ID: r.ID, Code: code, BuildingID: buildingID, Floor: level, UnitTypeCode: ut.Code, UnitTypeName: ut.Name, Status: room.Status(status)})
	}
	return out, nil
}

// codeSpan lists the room codes from..to: the same letters, then numbers counted up, zero-padded like from.
func codeSpan(from, to string) ([]string, error) {
	fp, fn, width, ok := splitCode(from)
	tp, tn, _, ok2 := splitCode(to)
	switch {
	case !ok || !ok2 || fp != tp:
		return nil, &ValidationError{"fromCode", "codes need the same letters and a number, such as A101"}
	case tn < fn:
		return nil, &ValidationError{"toCode", "must not be before fromCode"}
	case tn-fn+1 > maxRoomsPerRequest:
		return nil, &ValidationError{"toCode", "at most 200 rooms at a time"}
	}
	return numbered(fp, fn, tn-fn+1, width), nil
}

func codeRange(start string, count int) ([]string, error) {
	p, n, width, ok := splitCode(start)
	if !ok {
		return nil, &ValidationError{"rooms.startCode", "a code such as A401"}
	}
	return numbered(p, n, count, width), nil
}

func numbered(prefix string, from, count, width int) []string {
	out := make([]string, count)
	for i := range out {
		out[i] = prefix + fmt.Sprintf("%0*d", width, from+i)
	}
	return out
}

// splitCode cuts "A107" into "A", 107 and the digit width 3. A code must end in digits and fit maxRoomCodeLen.
func splitCode(code string) (prefix string, n, width int, ok bool) {
	i := len(code)
	for i > 0 && code[i-1] >= '0' && code[i-1] <= '9' {
		i--
	}
	if i == len(code) || len(code) > maxRoomCodeLen || len(code)-i > 6 {
		return "", 0, 0, false
	}
	for _, r := range code[:i] {
		if r == ' ' || r < 0x21 {
			return "", 0, 0, false
		}
	}
	v, err := strconv.Atoi(code[i:])
	return code[:i], v, len(code) - i, err == nil
}

func validateFeatures(f []string) error {
	for _, x := range f {
		if !roomFeatures[x] {
			return &ValidationError{"features", "unknown feature"}
		}
	}
	return nil
}

// attributesWith returns the room attributes JSON with the given features and note replaced when not nil.
func attributesWith(current []byte, features []string, note *roomNote) ([]byte, error) {
	m := map[string]any{}
	if len(current) > 0 {
		if err := json.Unmarshal(current, &m); err != nil {
			return nil, fmt.Errorf("room attributes: %w", err)
		}
	}
	if features != nil {
		m["features"] = features
	}
	if note != nil {
		delete(m, "note")
		delete(m, "expectedBackOn")
		if note.reason != "" {
			m["note"] = note.reason
		}
		if note.backOn != "" {
			m["expectedBackOn"] = note.backOn
		}
	}
	return json.Marshal(m)
}

// roomNote is the maintenance reason and expected date; an empty one clears both.
type roomNote struct{ reason, backOn string }

// MaintenanceInput turns maintenance on or off.
type MaintenanceInput struct {
	On             bool
	Reason         *string
	ExpectedBackOn *string // YYYY-MM-DD
}

// RoomUpdateInput is the body of updateRoom; nil means unchanged.
type RoomUpdateInput struct {
	Code, UnitTypeCode *string
	Features           *[]string
	Maintenance        *MaintenanceInput
	Retired            *bool
}

// UpdateRoom edits a room. A room with a guest cannot change type, go into maintenance or be retired.
func (s *Setup) UpdateRoom(ctx context.Context, c Caller, roomID string, in RoomUpdateInput) (RoomView, error) {
	if err := s.authz.Check("updateRoom", c.Role, access.EDIT); err != nil {
		return RoomView{}, err
	}
	if in.Code != nil && (strings.TrimSpace(*in.Code) == "" || len(*in.Code) > maxRoomCodeLen) {
		return RoomView{}, &ValidationError{"code", "required, at most 20 characters"}
	}
	if in.Features != nil {
		if err := validateFeatures(*in.Features); err != nil {
			return RoomView{}, err
		}
	}
	if m := in.Maintenance; m != nil && m.ExpectedBackOn != nil {
		if _, err := time.Parse("2006-01-02", *m.ExpectedBackOn); err != nil {
			return RoomView{}, &ValidationError{"maintenance.expectedBackOn", "a date as YYYY-MM-DD"}
		}
	}
	var out RoomView
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		cur, ok, err := s.repo.Room(ctx, tx, roomID)
		if err != nil || !ok {
			return errOr(err, ErrNotFound)
		}
		upd, err := s.applyRoomUpdate(ctx, tx, cur, in)
		if err != nil {
			return err
		}
		if err = s.repo.UpdateRoom(ctx, tx, upd); err != nil {
			return err
		}
		if err = s.record(ctx, tx, c, "ROOM_UPDATED", "room", roomID, map[string]any{"status": upd.Status, "retired": upd.Retired}); err != nil {
			return err
		}
		out, err = s.roomView(ctx, tx, roomID)
		return err
	})
	return out, err
}

func (s *Setup) applyRoomUpdate(ctx context.Context, tx Tx, cur RoomSetupRow, in RoomUpdateInput) (RoomUpdate, error) {
	upd := RoomUpdate{ID: cur.ID, Code: cur.Code, UnitTypeID: cur.UnitTypeID, Status: cur.Status, Retired: cur.Retired}
	if in.UnitTypeCode != nil && *in.UnitTypeCode != cur.UnitTypeCode {
		if cur.HasGuest {
			return upd, ErrRoomOccupied
		}
		ut, ok, err := s.repo.UnitTypeByCode(ctx, tx, *in.UnitTypeCode)
		if err != nil || !ok {
			return upd, errOr(err, &ValidationError{"unitTypeCode", "unknown room type"})
		}
		upd.UnitTypeID = ut.ID
	}
	if in.Retired != nil && *in.Retired && !cur.Retired && cur.HasGuest {
		return upd, ErrRoomOccupied
	}
	if in.Retired != nil {
		upd.Retired = *in.Retired
	}
	if in.Code != nil && *in.Code != cur.Code {
		if err := s.checkFree(ctx, tx, []string{*in.Code}); err != nil {
			return upd, err
		}
		upd.Code = *in.Code
	}
	var features []string
	if in.Features != nil {
		features = *in.Features
		if features == nil {
			features = []string{}
		}
	}
	var note *roomNote
	if m := in.Maintenance; m != nil {
		if m.On {
			if cur.HasGuest || cur.Status == "OCCUPIED" {
				return upd, ErrRoomOccupied
			}
			upd.Status, note = statusMaintenance, &roomNote{reason: deref(m.Reason), backOn: deref(m.ExpectedBackOn)}
		} else if cur.Status == statusMaintenance {
			upd.Status, note = statusVacant, &roomNote{}
		}
	}
	var err error
	upd.Attributes, err = attributesWith(cur.Attributes, features, note)
	return upd, err
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// roomView reads the room back as the room map shows it (no active stay: setup answers describe the room itself).
func (s *Setup) roomView(ctx context.Context, tx Tx, id string) (RoomView, error) {
	r, ok, err := s.repo.Room(ctx, tx, id)
	if err != nil || !ok {
		return RoomView{}, errOr(err, ErrNotFound)
	}
	v := RoomView{ID: r.ID, Code: r.Code, BuildingID: r.BuildingID, Floor: r.FloorLevel, UnitTypeCode: r.UnitTypeCode, UnitTypeName: r.UnitTypeName, Status: room.Status(r.Status)}
	var attrs struct {
		Note *string `json:"note"`
	}
	if json.Unmarshal(r.Attributes, &attrs) == nil {
		v.Note = attrs.Note
	}
	return v, nil
}

// ---- rate plans ----

// RatePlanInput is the owner's rate plan without version and currency (the server sets both).
type RatePlanInput struct {
	GraceMinutes             int
	HourlyFirst, HourlyExtra int64
	Overnight, Daily         WindowInput
}

type WindowInput struct {
	Price      int64
	Start, End string
}

// UnitTypeRates is a room type with its current rate plan.
type UnitTypeRates struct {
	Code      string
	Name      LocalizedName
	RatePlan  pricing.RatePlan
	UpdatedAt time.Time
}

// planJSON is the rate plan document ParseRatePlan validates; it carries the version the plan will have.
func (r RatePlanInput) planJSON(version int) ([]byte, error) {
	win := func(w WindowInput) map[string]any {
		return map[string]any{"price": w.Price, "windowStart": w.Start, "windowEnd": w.End}
	}
	return json.Marshal(map[string]any{
		"version": version, "currency": pricing.CurrencyVND, "graceMinutes": r.GraceMinutes,
		"hourly":    map[string]any{"firstHour": r.HourlyFirst, "extraHour": r.HourlyExtra},
		"overnight": win(r.Overnight), "daily": win(r.Daily),
	})
}

func (s *Setup) parsePlan(in RatePlanInput, version int) (pricing.RatePlan, error) {
	raw, err := in.planJSON(version)
	if err != nil {
		return pricing.RatePlan{}, err
	}
	plan, err := pricing.ParseRatePlan(raw)
	var pve *pricing.ValidationError
	if errors.As(err, &pve) { // the owner's draft is bad: field codes for the 422
		fes := make([]stay.FieldError, len(pve.Errors))
		for i, fe := range pve.Errors {
			fes[i] = stay.FieldError{Path: fe.Path, Code: fe.Code}
		}
		return pricing.RatePlan{}, &stay.ValidationError{Errors: fes}
	}
	return plan, err
}

func ratesOf(u UnitTypeRow) (UnitTypeRates, error) {
	plan, err := pricing.ParseRatePlan(u.RatePlan)
	if err != nil {
		return UnitTypeRates{}, fmt.Errorf("stored rate plan of %s: %w", u.Code, err)
	}
	return UnitTypeRates{Code: u.Code, Name: u.Name, RatePlan: plan, UpdatedAt: u.UpdatedAt}, nil
}

func (s *Setup) ListRatePlans(ctx context.Context, c Caller) ([]UnitTypeRates, error) {
	if err := s.authz.Check("listRatePlans", c.Role, access.EDIT); err != nil {
		return nil, err
	}
	var out []UnitTypeRates
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		rows, err := s.repo.UnitTypes(ctx, tx)
		if err != nil {
			return err
		}
		for _, u := range rows {
			r, err := ratesOf(u)
			if err != nil {
				return err
			}
			out = append(out, r)
		}
		return nil
	})
	return out, err
}

// UpdateRatePlan saves a new version. Stays keep the snapshot they were priced with; later check-ins use this one.
func (s *Setup) UpdateRatePlan(ctx context.Context, c Caller, code string, in RatePlanInput) (UnitTypeRates, error) {
	if err := s.authz.Check("updateRatePlan", c.Role, access.EDIT); err != nil {
		return UnitTypeRates{}, err
	}
	var out UnitTypeRates
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		u, ok, err := s.repo.UnitTypeByCode(ctx, tx, code)
		if err != nil || !ok {
			return errOr(err, ErrNotFound)
		}
		plan, err := s.parsePlan(in, u.Version+1)
		if err != nil {
			return err
		}
		now := s.clock.Now()
		if err = s.repo.UpdateRatePlan(ctx, tx, u.ID, plan.Snapshot(), u.Version+1, now); err != nil {
			return err
		}
		if err = s.record(ctx, tx, c, "RATE_PLAN_UPDATED", "unit_type", u.ID, map[string]any{"code": code, "version": u.Version + 1}); err != nil {
			return err
		}
		out = UnitTypeRates{Code: u.Code, Name: u.Name, RatePlan: plan, UpdatedAt: now}
		return nil
	})
	return out, err
}

// PreviewPrice prices a sample stay with a draft plan through the same pricing engine as check-out.
func (s *Setup) PreviewPrice(ctx context.Context, c Caller, rental string, checkIn, checkOut time.Time, in RatePlanInput) (QuoteView, error) {
	if err := s.authz.Check("previewPrice", c.Role, access.EDIT); err != nil {
		return QuoteView{}, err
	}
	rt, err := pricing.ParseRentalType(rental)
	if err != nil {
		return QuoteView{}, &ValidationError{"rentalType", "unknown rental type"}
	}
	if !checkOut.After(checkIn) {
		return QuoteView{}, &ValidationError{"checkOut", "must be after checkIn"}
	}
	plan, err := s.parsePlan(in, pricing.MinVersion)
	if err != nil {
		return QuoteView{}, err
	}
	var loc *time.Location
	err = s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		zone, err := s.repo.Timezone(ctx, tx)
		if err != nil {
			return err
		}
		loc, err = time.LoadLocation(zone)
		return err
	})
	if err != nil {
		return QuoteView{}, fmt.Errorf("tenant time zone: %w", err)
	}
	q, err := pricing.Price(plan, rt, checkIn, checkOut, loc)
	if err != nil {
		return QuoteView{}, fmt.Errorf("price preview: %w", err)
	}
	bill, err := pricing.Assemble(q, nil, money.Vnd(0))
	if err != nil {
		return QuoteView{}, fmt.Errorf("price preview bill: %w", err)
	}
	return quoteViewOf(checkOut, q, bill, 0), nil
}

// ---- items and stock ----

// ServiceInput is the body of createService.
type ServiceInput struct {
	Name            LocalizedName
	Price, UnitCost int64
	OpeningQuantity int
	Unit            string
	LowStockAt      int
	OnSale          *bool
}

func validateItem(name LocalizedName, price int64, unit string, lowStockAt int) error {
	switch {
	case strings.TrimSpace(name.VI) == "" || strings.TrimSpace(name.EN) == "" || len(name.VI) > maxItemNameLen || len(name.EN) > maxItemNameLen:
		return &ValidationError{"name", "Vietnamese and English names, at most 80 characters each"}
	case price < 0:
		return &ValidationError{"price", "must not be negative"}
	case strings.TrimSpace(unit) == "" || len(unit) > maxUnitLen:
		return &ValidationError{"unit", "required, at most 20 characters"}
	case lowStockAt < 0:
		return &ValidationError{"lowStockAt", "must not be negative"}
	}
	return nil
}

// serviceCodeFrom reduces the English name like the migration did: capitals and digits, joined by underscores.
func serviceCodeFrom(name string) string {
	c := strings.Trim(serviceCodeChars.ReplaceAllString(strings.ToUpper(name), "_"), "_")
	if c == "" {
		c = "ITEM"
	}
	if len(c) > maxServiceCodeLen-4 { // room for a numeric suffix
		c = c[:maxServiceCodeLen-4]
	}
	return c
}

// CreateService adds an item with its opening stock as an OPENING movement.
func (s *Setup) CreateService(ctx context.Context, c Caller, key string, in ServiceInput) (ServiceItem, error) {
	if err := s.authz.Check("createService", c.Role, access.EDIT); err != nil {
		return ServiceItem{}, err
	}
	if err := s.validKey(key); err != nil {
		return ServiceItem{}, err
	}
	if err := validateItem(in.Name, in.Price, in.Unit, in.LowStockAt); err != nil {
		return ServiceItem{}, err
	}
	if in.UnitCost < 0 || in.OpeningQuantity < 0 {
		return ServiceItem{}, &ValidationError{"openingQuantity", "cost and quantity must not be negative"}
	}
	var out ServiceItem
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		var err error
		out, err = idempotent(ctx, s.idem, tx, "POST /v1/owner/services", key, in, func() (ServiceItem, error) { return s.createService(ctx, tx, c, in) })
		return err
	})
	return out, err
}

func (s *Setup) createService(ctx context.Context, tx Tx, c Caller, in ServiceInput) (ServiceItem, error) {
	code, err := s.freeServiceCode(ctx, tx, serviceCodeFrom(in.Name.EN))
	if err != nil {
		return ServiceItem{}, err
	}
	cost := in.UnitCost
	item := ServiceItem{ID: s.ids.New(servicePrefix), Code: code, Name: in.Name, Price: in.Price, Stock: int64(in.OpeningQuantity),
		Unit: in.Unit, LowStockAt: in.LowStockAt, OnSale: in.OnSale == nil || *in.OnSale, LatestUnitCost: &cost}
	if err = s.repo.InsertService(ctx, tx, item); err != nil {
		return ServiceItem{}, err
	}
	if in.OpeningQuantity > 0 {
		m := StockMovement{ID: s.ids.New(movementPrefix), ServiceID: item.ID, Kind: kindOpening, Quantity: int64(in.OpeningQuantity), UnitCost: &cost, ActorID: &c.UserID, At: s.clock.Now()}
		if err = s.repo.InsertMovement(ctx, tx, m); err != nil {
			return ServiceItem{}, err
		}
	}
	return item, s.record(ctx, tx, c, "SERVICE_CREATED", "service", item.ID, map[string]any{"code": code, "openingQuantity": in.OpeningQuantity})
}

func (s *Setup) freeServiceCode(ctx context.Context, tx Tx, base string) (string, error) {
	for n := 1; n < 100; n++ {
		code := base
		if n > 1 {
			code = fmt.Sprintf("%s_%d", base, n)
		}
		taken, err := s.repo.ServiceCodeTaken(ctx, tx, code)
		if err != nil {
			return "", err
		}
		if !taken {
			return code, nil
		}
	}
	return "", ErrConflict
}

// UpdateService edits details; the stock count is not a field here and never changes.
func (s *Setup) UpdateService(ctx context.Context, c Caller, code string, p ServicePatch) (ServiceItem, error) {
	if err := s.authz.Check("updateService", c.Role, access.EDIT); err != nil {
		return ServiceItem{}, err
	}
	var out ServiceItem
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		cur, ok, err := s.repo.Service(ctx, tx, code)
		if err != nil || !ok {
			return errOr(err, ErrNotFound)
		}
		next := cur
		if p.Name != nil {
			next.Name = *p.Name
		}
		if p.Price != nil {
			next.Price = *p.Price
		}
		if p.Unit != nil {
			next.Unit = *p.Unit
		}
		if p.LowStockAt != nil {
			next.LowStockAt = *p.LowStockAt
		}
		if err = validateItem(next.Name, next.Price, next.Unit, next.LowStockAt); err != nil {
			return err
		}
		if err = s.repo.UpdateService(ctx, tx, cur.ID, p); err != nil {
			return err
		}
		if p.OnSale != nil {
			next.OnSale = *p.OnSale
		}
		out = next
		return s.record(ctx, tx, c, "SERVICE_UPDATED", "service", cur.ID, nil)
	})
	return out, err
}

// RestockService records stock in with its unit cost as an IN movement and returns the item with its new stock.
func (s *Setup) RestockService(ctx context.Context, c Caller, key, code string, quantity, unitCost int64) (ServiceItem, error) {
	if err := s.authz.Check("restockService", c.Role, access.EDIT); err != nil {
		return ServiceItem{}, err
	}
	if err := s.validKey(key); err != nil {
		return ServiceItem{}, err
	}
	if quantity < 1 || quantity > maxRestock || unitCost < 0 {
		return ServiceItem{}, &ValidationError{"quantity", "a positive quantity and a cost that is not negative"}
	}
	var out ServiceItem
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		var err error
		route := "POST /v1/owner/services/" + code + "/restock"
		req := map[string]int64{"quantity": quantity, "unitCost": unitCost}
		out, err = idempotent(ctx, s.idem, tx, route, key, req, func() (ServiceItem, error) { return s.restock(ctx, tx, c, code, quantity, unitCost) })
		return err
	})
	return out, err
}

const maxRestock = 1_000_000

func (s *Setup) restock(ctx context.Context, tx Tx, c Caller, code string, quantity, unitCost int64) (ServiceItem, error) {
	item, ok, err := s.repo.Service(ctx, tx, code)
	if err != nil || !ok {
		return ServiceItem{}, errOr(err, ErrNotFound)
	}
	if item.Stock+quantity > maxStock {
		return ServiceItem{}, &ValidationError{"quantity", "too much stock"}
	}
	m := StockMovement{ID: s.ids.New(movementPrefix), ServiceID: item.ID, Kind: kindIn, Quantity: quantity, UnitCost: &unitCost, ActorID: &c.UserID, At: s.clock.Now()}
	if err = s.repo.InsertMovement(ctx, tx, m); err != nil {
		return ServiceItem{}, err
	}
	if item.Stock, err = s.repo.AddStock(ctx, tx, item.ID, quantity, &unitCost); err != nil {
		return ServiceItem{}, err
	}
	item.LatestUnitCost = &unitCost
	return item, s.record(ctx, tx, c, "STOCK_IN", "service", item.ID, map[string]any{"quantity": quantity})
}

const maxStock = 100_000_000
