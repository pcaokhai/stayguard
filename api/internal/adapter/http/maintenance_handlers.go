package httpadapter

import (
	"context"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/pcaokhai/stayguard/api/internal/adapter/http/gen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// MaintenanceService is what the damage, ticket and usage-report handlers need from the application layer (SG-1201, SG-401).
type MaintenanceService interface {
	ReportDamage(ctx context.Context, c app.Caller, roomID, retryID string, in app.DamageInput) (app.TicketView, error)
	ListTickets(ctx context.Context, c app.Caller, status string) ([]app.TicketView, error)
	GetTicket(ctx context.Context, c app.Caller, id string) (app.TicketView, error)
	UpdateTicket(ctx context.Context, c app.Caller, id string, in app.UpdateTicketInput) (app.TicketView, error)
	ReportUsage(ctx context.Context, c app.Caller, roomID, retryID, note string) (app.AlertRow, error)
}

// WithMaintenance adds the damage report, ticket and unused-room report use cases.
func (s Server) WithMaintenance(m MaintenanceService) Server { s.maintenance = m; return s }

func (s Server) ReportDamage(ctx context.Context, req gen.ReportDamageRequestObject) (gen.ReportDamageResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	b := req.Body
	t, err := s.maintenance.ReportDamage(ctx, c, req.RoomId, req.Params.IdempotencyKey.String(),
		app.DamageInput{Category: string(b.Category), Description: b.Description, Severity: string(b.Severity), PhotoAssetIDs: photoIDs(b.PhotoAssetIds)})
	if err != nil {
		return nil, err
	}
	return gen.ReportDamage201JSONResponse(toTicket(t)), nil
}

func (s Server) ListTickets(ctx context.Context, req gen.ListTicketsRequestObject) (gen.ListTicketsResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	status := ""
	if req.Params.Status != nil {
		status = string(*req.Params.Status)
	}
	rows, err := s.maintenance.ListTickets(ctx, c, status)
	if err != nil {
		return nil, err
	}
	items := make([]gen.MaintenanceTicket, len(rows))
	for i, r := range rows {
		items[i] = toTicket(r)
	}
	return gen.ListTickets200JSONResponse{Items: items}, nil
}

func (s Server) GetTicket(ctx context.Context, req gen.GetTicketRequestObject) (gen.GetTicketResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	t, err := s.maintenance.GetTicket(ctx, c, req.TicketId)
	if err != nil {
		return nil, err
	}
	return gen.GetTicket200JSONResponse(toTicket(t)), nil
}

func (s Server) UpdateTicket(ctx context.Context, req gen.UpdateTicketRequestObject) (gen.UpdateTicketResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	b := req.Body
	in := app.UpdateTicketInput{RoomLocked: b.RoomLocked, PartsCost: b.PartsCost, LabourCost: b.LabourCost, Repairer: b.Repairer, Note: b.Note}
	if b.Status != nil {
		st := string(*b.Status)
		in.Status = &st
	}
	if b.ExpectedDoneOn != nil {
		in.ExpectedDoneOn = &b.ExpectedDoneOn.Time
	}
	t, err := s.maintenance.UpdateTicket(ctx, c, req.TicketId, in)
	if err != nil {
		return nil, err
	}
	return gen.UpdateTicket200JSONResponse(toTicket(t)), nil
}

func (s Server) ReportRoomUsage(ctx context.Context, req gen.ReportRoomUsageRequestObject) (gen.ReportRoomUsageResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	a, err := s.maintenance.ReportUsage(ctx, c, req.RoomId, req.Params.IdempotencyKey.String(), deref(req.Body.Note))
	if err != nil {
		return nil, err
	}
	return gen.ReportRoomUsage201JSONResponse(toAlert(a)), nil
}

func toTicket(t app.TicketView) gen.MaintenanceTicket {
	out := gen.MaintenanceTicket{Id: t.ID, Code: t.Code, RoomId: t.RoomID, RoomCode: t.RoomCode, Category: gen.DamageCategory(t.Category),
		Description: t.Description, Status: gen.TicketStatus(t.Status), RoomLocked: t.RoomLocked, ReportedBy: t.ReportedBy,
		ReportedAt: t.ReportedAt, PartsCost: t.PartsCost, LabourCost: t.LabourCost, TotalCost: t.TotalCost, Repairer: t.Repairer,
		CompletedAt: t.CompletedAt}
	if t.ExpectedDoneOn != nil {
		out.ExpectedDoneOn = &openapi_types.Date{Time: *t.ExpectedDoneOn}
	}
	return out
}

func photoIDs(p *[]string) []string {
	if p == nil {
		return nil
	}
	return *p
}
