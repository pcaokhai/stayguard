package httpadapter

import (
	"context"
	"errors"

	"github.com/pcaokhai/stayguard/api/internal/adapter/http/gen"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
)

// RoomService is what the room map handlers need from the application layer (SG-201).
type RoomService interface {
	ListBuildings(ctx context.Context, c app.Caller) ([]app.BuildingView, error)
	ListRooms(ctx context.Context, c app.Caller, buildingID string, status *room.Status) ([]app.RoomView, error)
	GetRoom(ctx context.Context, c app.Caller, roomID string) (app.RoomView, error)
}

// errInvalidParam is a query value the generated binding does not check (enums); problem.go maps it to 400.
var errInvalidParam = errors.New("invalid query parameter")

// roomCaller returns the caller, or the flag/auth error that must end the request before any use case.
func (s Server) roomCaller(ctx context.Context) (app.Caller, error) {
	return s.featureCaller(ctx, s.roomMap)
}

// featureCaller is the shared gate: the flag is checked first, then the authenticated caller.
func (Server) featureCaller(ctx context.Context, enabled bool) (app.Caller, error) {
	if !enabled {
		return app.Caller{}, app.ErrFeatureDisabled
	}
	c, ok := app.CallerFrom(ctx)
	if !ok {
		return app.Caller{}, app.ErrUnauthenticated
	}
	return c, nil
}

func (s Server) ListBuildings(ctx context.Context, _ gen.ListBuildingsRequestObject) (gen.ListBuildingsResponseObject, error) {
	c, err := s.roomCaller(ctx)
	if err != nil {
		return nil, err
	}
	views, err := s.rooms.ListBuildings(ctx, c)
	if err != nil {
		return nil, err
	}
	items := make([]gen.Building, 0, len(views))
	for _, v := range views {
		items = append(items, toBuilding(v))
	}
	return gen.ListBuildings200JSONResponse{Items: items}, nil
}

func (s Server) ListRooms(ctx context.Context, req gen.ListRoomsRequestObject) (gen.ListRoomsResponseObject, error) {
	c, err := s.roomCaller(ctx)
	if err != nil {
		return nil, err
	}
	var status *room.Status
	if req.Params.Status != nil {
		if !req.Params.Status.Valid() {
			return nil, errInvalidParam
		}
		st := room.Status(*req.Params.Status)
		status = &st
	}
	views, err := s.rooms.ListRooms(ctx, c, req.BuildingId, status)
	if err != nil {
		return nil, err
	}
	items := make([]gen.Room, 0, len(views))
	for _, v := range views {
		items = append(items, toRoom(v))
	}
	return gen.ListRooms200JSONResponse{Items: items}, nil
}

func (s Server) GetRoom(ctx context.Context, req gen.GetRoomRequestObject) (gen.GetRoomResponseObject, error) {
	c, err := s.roomCaller(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.rooms.GetRoom(ctx, c, req.RoomId)
	if err != nil {
		return nil, err
	}
	return gen.GetRoom200JSONResponse(toRoom(v)), nil
}

func toRoom(v app.RoomView) gen.Room {
	r := gen.Room{
		Id: v.ID, Code: v.Code, BuildingId: v.BuildingID, Floor: v.Floor, FloorId: strPtr(v.FloorID), FloorName: strPtr(v.FloorName),
		Status: gen.RoomStatus(v.Status), Note: v.Note,
	}
	r.UnitType.Code = v.UnitTypeCode
	r.UnitType.Name = gen.LocalizedText{Vi: v.UnitTypeName.VI, En: v.UnitTypeName.EN}
	if s := v.ActiveStay; s != nil {
		r.ActiveStay = &gen.StaySummary{
			Id: s.ID, RentalType: gen.RentalType(s.RentalType), GuestName: s.GuestName,
			CheckInAt: s.CheckInAt, ElapsedMinutes: s.ElapsedMinutes, RunningTotal: s.RunningTotal,
		}
	}
	return r
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func toBuilding(v app.BuildingView) gen.Building {
	var floors []struct {
		Id    string `json:"id"`
		Name  string `json:"name"`
		Order int    `json:"order"`
	}
	for _, f := range v.Floors {
		floors = append(floors, struct {
			Id    string `json:"id"`
			Name  string `json:"name"`
			Order int    `json:"order"`
		}{f.ID, f.Name, f.Order})
	}
	return gen.Building{Floors: &floors,
		Id: v.ID, Code: v.Code, Name: v.Name, Level: permissionLevel(v.Level),
		Counts: gen.StatusCounts{
			Vacant: v.Counts.Vacant, Occupied: v.Counts.Occupied, Overdue: v.Counts.Overdue,
			ToClean: v.Counts.ToClean, Maintenance: v.Counts.Maintenance,
		},
	}
}
