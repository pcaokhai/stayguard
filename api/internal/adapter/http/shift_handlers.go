package httpadapter

import (
	"time"

	"context"

	"github.com/pcaokhai/stayguard/api/internal/adapter/http/gen"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/shift"
)

// ShiftService is what the shift handlers need from the application layer (SG-503, SG-905).
type ShiftService interface {
	Current(ctx context.Context, c app.Caller) (app.ShiftView, error)
	Payout(ctx context.Context, c app.Caller, retryID string, in app.PayoutInput) (app.ShiftView, error)
	Close(ctx context.Context, c app.Caller, retryID string, in app.CloseShiftInput) (app.ShiftReview, error)
	Review(ctx context.Context, c app.Caller, shiftID string) (app.ShiftReview, error)
	ListClosed(ctx context.Context, c app.Caller, q app.ClosedShiftsQuery) (app.ClosedShiftPage, error)
}

// WithShifts adds the shift use cases.
func (s Server) WithShifts(svc ShiftService) Server { s.shifts = svc; return s }

func (s Server) GetCurrentShift(ctx context.Context, _ gen.GetCurrentShiftRequestObject) (gen.GetCurrentShiftResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	v, err := s.shifts.Current(ctx, c)
	if err != nil {
		return nil, err
	}
	return gen.GetCurrentShift200JSONResponse(toShift(v)), nil
}

func (s Server) RecordCashPayout(ctx context.Context, req gen.RecordCashPayoutRequestObject) (gen.RecordCashPayoutResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	v, err := s.shifts.Payout(ctx, c, req.Params.IdempotencyKey.String(),
		app.PayoutInput{Amount: int64(req.Body.Amount), Description: req.Body.Description})
	if err != nil {
		return nil, err
	}
	return gen.RecordCashPayout200JSONResponse(toShift(v)), nil
}

func (s Server) CloseShift(ctx context.Context, req gen.CloseShiftRequestObject) (gen.CloseShiftResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	b := req.Body
	in := app.CloseShiftInput{FloatLeft: b.FloatLeft, Reason: deref(b.Reason), HandoverToUserID: deref(b.HandoverToUserId)}
	for _, cc := range b.Counts {
		in.Counts = append(in.Counts, shift.Count{Denomination: int64(cc.Denomination), Quantity: int64(cc.Quantity)})
	}
	r, err := s.shifts.Close(ctx, c, req.Params.IdempotencyKey.String(), in)
	if err != nil {
		return nil, err
	}
	return gen.CloseShift200JSONResponse(toShiftReview(r)), nil
}

func (s Server) GetShiftReview(ctx context.Context, req gen.GetShiftReviewRequestObject) (gen.GetShiftReviewResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	r, err := s.shifts.Review(ctx, c, req.ShiftId)
	if err != nil {
		return nil, err
	}
	return gen.GetShiftReview200JSONResponse(toShiftReview(r)), nil
}

func (s Server) ListClosedShifts(ctx context.Context, req gen.ListClosedShiftsRequestObject) (gen.ListClosedShiftsResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	p := req.Params
	q := app.ClosedShiftsQuery{Month: deref(p.Month), UserID: deref(p.UserId), Cursor: deref(p.Cursor)}
	if p.OnlyDifferences != nil {
		q.OnlyDifferences = *p.OnlyDifferences
	}
	page, err := s.shifts.ListClosed(ctx, c, q)
	if err != nil {
		return nil, err
	}
	items := make([]gen.ClosedShift, len(page.Items))
	for i, r := range page.Items {
		code := gen.ShiftCode(r.Code)
		items[i] = gen.ClosedShift{Id: r.ID, UserName: r.UserName, Shift: &code, OpenedAt: r.OpenedAt, ClosedAt: r.ClosedAt, Difference: int(r.Difference)}
	}
	out := gen.ListClosedShifts200JSONResponse{Items: items}
	if page.NextCursor != "" {
		out.NextCursor = &page.NextCursor
	}
	return out, nil
}

func toShift(v app.ShiftView) gen.Shift {
	return gen.Shift{Id: v.ID, UserId: v.UserID, UserName: v.UserName, Status: gen.ShiftStatus(v.Status), OpenedAt: v.OpenedAt,
		ClosedAt: v.ClosedAt, OpeningFloat: v.OpeningFloat, CashIn: v.CashIn, CashOut: v.CashOut, ExpectedCash: v.ExpectedCash,
		TransfersReceived: v.TransfersReceived, BuildingIds: v.BuildingIDs}
}

func toShiftReview(r app.ShiftReview) gen.ShiftReview {
	out := gen.ShiftReview{Shift: toShift(r.Shift), CountedCash: r.CountedCash, Difference: r.Difference, Reason: r.Reason,
		ReasonRecordedAt: r.ReasonRecordedAt}
	out.CashPayments = make([]struct {
		Amount     gen.Vnd        `json:"amount"`
		At         time.Time      `json:"at"`
		RentalType gen.RentalType `json:"rentalType"`
		RoomCode   string         `json:"roomCode"`
	}, len(r.CashPayments))
	for i, p := range r.CashPayments {
		out.CashPayments[i].Amount, out.CashPayments[i].At = p.Amount, p.At
		out.CashPayments[i].RentalType, out.CashPayments[i].RoomCode = gen.RentalType(p.RentalType), p.RoomCode
	}
	out.StaffHistory.ShiftsWithDifference, out.StaffHistory.TotalShort = int(r.ShiftsWithDiff), r.TotalShort
	return out
}
