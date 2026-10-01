package httpadapter

import (
	"context"

	"github.com/pcaokhai/stayguard/api/internal/adapter/http/gen"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

// StayService is what the check-in handlers need from the application layer (SG-203).
type StayService interface {
	// CreateStay reports whether the answer is the stored response of an earlier identical call.
	CreateStay(ctx context.Context, c app.Caller, roomID, idemKey string, in app.CreateStayInput) (app.StayDetail, bool, error)
	GetStay(ctx context.Context, c app.Caller, stayID string) (app.StayDetail, error)
}

// errBodyRequired is a JSON `null` body: the binding accepts it, the use case cannot.
var errBodyRequired = &stay.ValidationError{Errors: []stay.FieldError{{Path: "body", Code: "REQUIRED"}}}

func (s Server) CreateStay(ctx context.Context, req gen.CreateStayRequestObject) (gen.CreateStayResponseObject, error) {
	c, err := s.featureCaller(ctx, s.checkIn)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, errBodyRequired
	}
	b := req.Body
	in := app.CreateStayInput{RentalType: string(b.RentalType), GuestName: b.GuestName, GuestPhone: b.GuestPhone,
		IDNumber: b.IdNumber, Deposit: b.Deposit}
	// The generated binding already typed the key as a UUID; the use case bounds its length again.
	d, _, err := s.stays.CreateStay(ctx, c, req.RoomId, req.Params.IdempotencyKey.String(), in)
	if err != nil {
		return nil, err
	}
	return gen.CreateStay201JSONResponse(toStay(d)), nil
}

func (s Server) GetStay(ctx context.Context, req gen.GetStayRequestObject) (gen.GetStayResponseObject, error) {
	c, err := s.featureCaller(ctx, s.checkIn)
	if err != nil {
		return nil, err
	}
	d, err := s.stays.GetStay(ctx, c, req.StayId)
	if err != nil {
		return nil, err
	}
	return gen.GetStay200JSONResponse(toStay(d)), nil
}
