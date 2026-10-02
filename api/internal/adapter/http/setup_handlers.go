package httpadapter

import (
	"context"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/adapter/http/gen"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/pricing"
)

// SetupService is the building, room, rate plan and item surface the handlers need.
type SetupService interface {
	CreateBuilding(ctx context.Context, c app.Caller, key string, in app.BuildingInput) (app.BuildingView, error)
	UpdateBuilding(ctx context.Context, c app.Caller, id string, name *string) (app.BuildingView, error)
	CreateFloor(ctx context.Context, c app.Caller, key, buildingID string, in app.FloorInput) ([]app.RoomView, error)
	CreateRooms(ctx context.Context, c app.Caller, key string, in app.RoomsInput) ([]app.RoomView, error)
	UpdateRoom(ctx context.Context, c app.Caller, roomID string, in app.RoomUpdateInput) (app.RoomView, error)
	ListRatePlans(ctx context.Context, c app.Caller) ([]app.UnitTypeRates, error)
	UpdateRatePlan(ctx context.Context, c app.Caller, code string, in app.RatePlanInput) (app.UnitTypeRates, error)
	PreviewPrice(ctx context.Context, c app.Caller, rental string, checkIn, checkOut time.Time, in app.RatePlanInput) (app.QuoteView, error)
	CreateService(ctx context.Context, c app.Caller, key string, in app.ServiceInput) (app.ServiceItem, error)
	UpdateService(ctx context.Context, c app.Caller, code string, p app.ServicePatch) (app.ServiceItem, error)
	RestockService(ctx context.Context, c app.Caller, key, code string, quantity, unitCost int64) (app.ServiceItem, error)
}

// WithSetup adds the setup use cases; the router always sets them.
func (s Server) WithSetup(v SetupService) Server { s.setup = v; return s }

func toRooms(vs []app.RoomView) []gen.Room {
	out := make([]gen.Room, len(vs))
	for i, v := range vs {
		out[i] = toRoom(v)
	}
	return out
}

func ratePlanInput(r gen.RatePlan) app.RatePlanInput {
	return app.RatePlanInput{GraceMinutes: r.GraceMinutes, HourlyFirst: r.Hourly.FirstHour, HourlyExtra: r.Hourly.ExtraHour,
		Overnight: app.WindowInput{Price: r.Overnight.Price, Start: r.Overnight.WindowStart, End: r.Overnight.WindowEnd},
		Daily:     app.WindowInput{Price: r.Daily.Price, Start: r.Daily.WindowStart, End: r.Daily.WindowEnd}}
}

func toRatePlan(p pricing.RatePlan) gen.RatePlan {
	var r gen.RatePlan
	v := int(p.Version)
	r.Version, r.GraceMinutes = &v, p.GraceMinutes
	r.Hourly.FirstHour, r.Hourly.ExtraHour = p.Hourly.FirstHour.Int64(), p.Hourly.ExtraHour.Int64()
	r.Overnight.Price, r.Overnight.WindowStart, r.Overnight.WindowEnd = p.Overnight.Price.Int64(), p.Overnight.Start.String(), p.Overnight.End.String()
	r.Daily.Price, r.Daily.WindowStart, r.Daily.WindowEnd = p.Daily.Price.Int64(), p.Daily.Start.String(), p.Daily.End.String()
	return r
}

func toUnitTypeRates(u app.UnitTypeRates) gen.UnitTypeRates {
	return gen.UnitTypeRates{Code: u.Code, Name: gen.LocalizedText{Vi: u.Name.VI, En: u.Name.EN}, RatePlan: toRatePlan(u.RatePlan), UpdatedAt: u.UpdatedAt}
}

func toServiceItem(s app.ServiceItem) gen.Service {
	out := gen.Service{Code: s.Code, Name: gen.LocalizedText{Vi: s.Name.VI, En: s.Name.EN}, Price: s.Price, Stock: int(s.Stock),
		Unit: &s.Unit, LowStockAt: &s.LowStockAt, OnSale: &s.OnSale}
	if s.LatestUnitCost != nil {
		out.LatestUnitCost = s.LatestUnitCost
	}
	return out
}

func (s Server) CreateBuilding(ctx context.Context, req gen.CreateBuildingRequestObject) (gen.CreateBuildingResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, &app.ValidationError{Field: "body", Reason: "required"}
	}
	b := req.Body
	v, err := s.setup.CreateBuilding(ctx, c, req.Params.IdempotencyKey.String(), app.BuildingInput{Code: b.Code, Name: b.Name, UnitTypeCode: b.UnitTypeCode, Floors: b.Floors, RoomsPerFloor: b.RoomsPerFloor})
	if err != nil {
		return nil, err
	}
	return gen.CreateBuilding201JSONResponse(toBuilding(v)), nil
}

func (s Server) UpdateBuilding(ctx context.Context, req gen.UpdateBuildingRequestObject) (gen.UpdateBuildingResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, &app.ValidationError{Field: "body", Reason: "required"}
	}
	v, err := s.setup.UpdateBuilding(ctx, c, req.BuildingId, req.Body.Name)
	if err != nil {
		return nil, err
	}
	return gen.UpdateBuilding200JSONResponse(toBuilding(v)), nil
}

func (s Server) CreateFloor(ctx context.Context, req gen.CreateFloorRequestObject) (gen.CreateFloorResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, &app.ValidationError{Field: "body", Reason: "required"}
	}
	in := app.FloorInput{Name: req.Body.Name}
	if r := req.Body.Rooms; r != nil {
		in.Rooms = &app.FloorRooms{Count: r.Count, StartCode: r.StartCode, UnitTypeCode: r.UnitTypeCode}
	}
	rooms, err := s.setup.CreateFloor(ctx, c, req.Params.IdempotencyKey.String(), req.BuildingId, in)
	if err != nil {
		return nil, err
	}
	return gen.CreateFloor201JSONResponse{Items: toRooms(rooms)}, nil
}

func (s Server) CreateRooms(ctx context.Context, req gen.CreateRoomsRequestObject) (gen.CreateRoomsResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, &app.ValidationError{Field: "body", Reason: "required"}
	}
	b := req.Body
	in := app.RoomsInput{BuildingID: b.BuildingId, FloorID: b.FloorId, From: b.FromCode, To: b.ToCode, UnitTypeCode: b.UnitTypeCode, AvailableNow: b.AvailableNow == nil || *b.AvailableNow}
	if b.Features != nil {
		in.Features = []string{}
		for _, f := range *b.Features {
			in.Features = append(in.Features, string(f))
		}
	}
	rooms, err := s.setup.CreateRooms(ctx, c, req.Params.IdempotencyKey.String(), in)
	if err != nil {
		return nil, err
	}
	return gen.CreateRooms201JSONResponse{Items: toRooms(rooms)}, nil
}

func (s Server) UpdateRoom(ctx context.Context, req gen.UpdateRoomRequestObject) (gen.UpdateRoomResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, &app.ValidationError{Field: "body", Reason: "required"}
	}
	b := req.Body
	in := app.RoomUpdateInput{Code: b.Code, UnitTypeCode: b.UnitTypeCode, Retired: b.Retired}
	if b.Features != nil {
		f := []string{}
		for _, x := range *b.Features {
			f = append(f, string(x))
		}
		in.Features = &f
	}
	if m := b.Maintenance; m != nil {
		mi := app.MaintenanceInput{On: m.On, Reason: m.Reason}
		if m.ExpectedBackOn != nil {
			d := m.ExpectedBackOn.Format("2006-01-02")
			mi.ExpectedBackOn = &d
		}
		in.Maintenance = &mi
	}
	v, err := s.setup.UpdateRoom(ctx, c, req.RoomId, in)
	if err != nil {
		return nil, err
	}
	return gen.UpdateRoom200JSONResponse(toRoom(v)), nil
}

func (s Server) ListRatePlans(ctx context.Context, _ gen.ListRatePlansRequestObject) (gen.ListRatePlansResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	vs, err := s.setup.ListRatePlans(ctx, c)
	if err != nil {
		return nil, err
	}
	items := make([]gen.UnitTypeRates, len(vs))
	for i, v := range vs {
		items[i] = toUnitTypeRates(v)
	}
	return gen.ListRatePlans200JSONResponse{Items: items}, nil
}

func (s Server) UpdateRatePlan(ctx context.Context, req gen.UpdateRatePlanRequestObject) (gen.UpdateRatePlanResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, &app.ValidationError{Field: "body", Reason: "required"}
	}
	v, err := s.setup.UpdateRatePlan(ctx, c, req.UnitTypeCode, ratePlanInput(*req.Body))
	if err != nil {
		return nil, err
	}
	return gen.UpdateRatePlan200JSONResponse(toUnitTypeRates(v)), nil
}

func (s Server) PreviewPrice(ctx context.Context, req gen.PreviewPriceRequestObject) (gen.PreviewPriceResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, &app.ValidationError{Field: "body", Reason: "required"}
	}
	b := req.Body
	q, err := s.setup.PreviewPrice(ctx, c, string(b.RentalType), b.CheckIn, b.CheckOut, ratePlanInput(b.RatePlan))
	if err != nil {
		return nil, err
	}
	return gen.PreviewPrice200JSONResponse(toQuote(q)), nil
}

func (s Server) CreateService(ctx context.Context, req gen.CreateServiceRequestObject) (gen.CreateServiceResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, &app.ValidationError{Field: "body", Reason: "required"}
	}
	b := req.Body
	v, err := s.setup.CreateService(ctx, c, req.Params.IdempotencyKey.String(), app.ServiceInput{
		Name: app.LocalizedName{VI: b.Name.Vi, EN: b.Name.En}, Price: b.Price, UnitCost: b.UnitCost, OpeningQuantity: b.OpeningQuantity,
		Unit: b.Unit, LowStockAt: b.LowStockAt, OnSale: b.OnSale})
	if err != nil {
		return nil, err
	}
	return gen.CreateService201JSONResponse(toServiceItem(v)), nil
}

func (s Server) UpdateService(ctx context.Context, req gen.UpdateServiceRequestObject) (gen.UpdateServiceResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, &app.ValidationError{Field: "body", Reason: "required"}
	}
	b := req.Body
	p := app.ServicePatch{Price: b.Price, Unit: b.Unit, LowStockAt: b.LowStockAt, OnSale: b.OnSale}
	if b.Name != nil {
		p.Name = &app.LocalizedName{VI: b.Name.Vi, EN: b.Name.En}
	}
	v, err := s.setup.UpdateService(ctx, c, req.ServiceCode, p)
	if err != nil {
		return nil, err
	}
	return gen.UpdateService200JSONResponse(toServiceItem(v)), nil
}

func (s Server) RestockService(ctx context.Context, req gen.RestockServiceRequestObject) (gen.RestockServiceResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, &app.ValidationError{Field: "body", Reason: "required"}
	}
	v, err := s.setup.RestockService(ctx, c, req.Params.IdempotencyKey.String(), req.ServiceCode, int64(req.Body.Quantity), req.Body.UnitCost)
	if err != nil {
		return nil, err
	}
	return gen.RestockService200JSONResponse(toServiceItem(v)), nil
}
