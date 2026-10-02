package httpadapter

import (
	"context"

	"github.com/pcaokhai/stayguard/api/internal/adapter/http/gen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// StayOpsService is what the stay history and correction handlers need from the application layer (SG-801, SG-802, SG-904).
type StayOpsService interface {
	EditCheckIn(ctx context.Context, c app.Caller, stayID, retryID string, in app.EditCheckInInput) (app.StayDetail, error)
	MoveStay(ctx context.Context, c app.Caller, stayID, retryID string, in app.MoveStayInput) (app.StayDetail, error)
	ListStays(ctx context.Context, c app.Caller, q app.StayListQuery) (app.StayListPage, error)
	Timeline(ctx context.Context, c app.Caller, stayID string) ([]app.TimelineRow, error)
	Receipt(ctx context.Context, c app.Caller, invoiceID string) (app.ReceiptView, error)
}

func (s Server) EditCheckInTime(ctx context.Context, req gen.EditCheckInTimeRequestObject) (gen.EditCheckInTimeResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	b := req.Body
	in := app.EditCheckInInput{NewCheckInAt: b.NewCheckInAt, ReasonCode: string(b.ReasonCode), Note: b.Note}
	d, err := s.stayOps.EditCheckIn(ctx, c, req.StayId, req.Params.IdempotencyKey.String(), in)
	if err != nil {
		return nil, err
	}
	return gen.EditCheckInTime200JSONResponse(toStay(d)), nil
}

func (s Server) MoveStay(ctx context.Context, req gen.MoveStayRequestObject) (gen.MoveStayResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	d, err := s.stayOps.MoveStay(ctx, c, req.StayId, req.Params.IdempotencyKey.String(),
		app.MoveStayInput{ToRoomID: req.Body.ToRoomId, RentalType: string(req.Body.RentalType)})
	if err != nil {
		return nil, err
	}
	return gen.MoveStay200JSONResponse(toStay(d)), nil
}

func (s Server) ListStays(ctx context.Context, req gen.ListStaysRequestObject) (gen.ListStaysResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	p := req.Params
	q := app.StayListQuery{Query: deref(p.Q), BuildingID: deref(p.BuildingId), State: deref(p.State), Cursor: deref(p.Cursor)}
	if p.Date != nil {
		q.Date = &p.Date.Time
	}
	if p.From != nil {
		q.From = &p.From.Time
	}
	if p.To != nil {
		q.To = &p.To.Time
	}
	page, err := s.stayOps.ListStays(ctx, c, q)
	if err != nil {
		return nil, err
	}
	items := make([]gen.StayListItem, len(page.Items))
	for i, r := range page.Items {
		items[i] = toStayListItem(r)
	}
	out := gen.ListStays200JSONResponse{Items: items}
	if page.NextCursor != "" {
		out.NextCursor = &page.NextCursor
	}
	return out, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// toStayListItem carries guestId indicators as all false until the guest ID task (F-A2) fills them.
func toStayListItem(r app.StayListRow) gen.StayListItem {
	state := gen.StayListItemState(r.State)
	item := gen.StayListItem{Id: r.ID, RoomCode: r.RoomCode, GuestName: r.GuestName, RentalType: gen.RentalType(r.RentalType),
		CheckInAt: r.CheckInAt, CheckOutAt: r.CheckOutAt, Total: r.Total, State: &state, Status: gen.StayStatus(r.Status),
		GuestId: gen.GuestIdIndicators{}, InvoiceId: nilIfEmpty(r.InvoiceID), BillCode: nilIfEmpty(r.BillCode)}
	if r.PaymentMethod != "" {
		m := gen.PaymentMethod(r.PaymentMethod)
		item.PaymentMethod = &m
	}
	if r.FrontDeskName != "" {
		item.FrontDeskName = &r.FrontDeskName
	}
	return item
}

func (s Server) GetStayTimeline(ctx context.Context, req gen.GetStayTimelineRequestObject) (gen.GetStayTimelineResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	rows, err := s.stayOps.Timeline(ctx, c, req.StayId)
	if err != nil {
		return nil, err
	}
	items := make([]gen.StayTimelineEvent, len(rows))
	for i, r := range rows {
		d := r.Details
		items[i] = gen.StayTimelineEvent{At: r.At, Kind: gen.StayTimelineEventKind(r.Kind), ActorName: r.ActorName, Details: &d}
	}
	return gen.GetStayTimeline200JSONResponse{Items: items}, nil
}

func (s Server) GetReceipt(ctx context.Context, req gen.GetReceiptRequestObject) (gen.GetReceiptResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	v, err := s.stayOps.Receipt(ctx, c, req.InvoiceId)
	if err != nil {
		return nil, err
	}
	return gen.GetReceipt200JSONResponse(toReceipt(v)), nil
}

func toReceipt(v app.ReceiptView) gen.Receipt {
	lines := make([]gen.BillLine, len(v.Lines))
	for i, l := range v.Lines {
		lines[i] = gen.BillLine{Code: l.Code, Quantity: int(l.Quantity), UnitAmount: l.UnitAmount, Amount: l.Amount}
	}
	extras := make([]gen.ExtraLine, len(v.Extras))
	for i, e := range v.Extras {
		extras[i] = gen.ExtraLine{Amount: e.Amount, Name: gen.LocalizedText{Vi: e.Name.VI, En: e.Name.EN},
			Quantity: int(e.Quantity), ServiceCode: e.ServiceCode, UnitAmount: e.UnitAmount}
	}
	pays := make([]gen.PaymentSummary, len(v.Payments))
	for i, p := range v.Payments {
		pays[i] = gen.PaymentSummary{PaymentId: p.ID, Method: gen.PaymentMethod(p.Method), Amount: p.Amount, At: p.At, RoomCode: v.RoomCode}
	}
	deposit := v.Deposit
	return gen.Receipt{PropertyName: v.PropertyName, BillCode: v.BillCode, RoomCode: v.RoomCode, CheckInAt: v.CheckInAt,
		CheckOutAt: v.CheckOutAt, Lines: lines, Extras: &extras, Total: v.Total, Deposit: &deposit, Payments: pays}
}
