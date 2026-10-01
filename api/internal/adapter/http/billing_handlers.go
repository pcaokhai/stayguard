package httpadapter

import (
	"context"

	"github.com/pcaokhai/stayguard/api/internal/adapter/http/gen"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

// BillingService is what the extras and check-out handlers need from the application layer (SG-205).
type BillingService interface {
	ListServices(ctx context.Context, c app.Caller) ([]app.Service, error)
	// AddExtras and Checkout report whether the answer is the stored response of an earlier identical call.
	AddExtras(ctx context.Context, c app.Caller, stayID, retryID string, items []stay.ExtraInput) (app.StayDetail, bool, error)
	Checkout(ctx context.Context, c app.Caller, stayID, retryID string) (app.InvoiceView, bool, error)
}

func (s Server) ListServices(ctx context.Context, _ gen.ListServicesRequestObject) (gen.ListServicesResponseObject, error) {
	c, err := s.featureCaller(ctx, s.checkout)
	if err != nil {
		return nil, err
	}
	svcs, err := s.billing.ListServices(ctx, c)
	if err != nil {
		return nil, err
	}
	return gen.ListServices200JSONResponse{Items: toServices(svcs)}, nil
}

func (s Server) AddStayExtras(ctx context.Context, req gen.AddStayExtrasRequestObject) (gen.AddStayExtrasResponseObject, error) {
	c, err := s.featureCaller(ctx, s.checkout)
	if err != nil {
		return nil, err
	}
	var items []stay.ExtraInput
	if req.Body != nil {
		for _, it := range req.Body.Items {
			items = append(items, stay.ExtraInput{ServiceCode: it.ServiceCode, Quantity: it.Quantity})
		}
	}
	// The generated binding already typed the key as a UUID; the use case bounds its length again.
	d, _, err := s.billing.AddExtras(ctx, c, req.StayId, req.Params.IdempotencyKey.String(), items)
	if err != nil {
		return nil, err
	}
	return gen.AddStayExtras200JSONResponse(toStay(d)), nil
}

func (s Server) CheckoutStay(ctx context.Context, req gen.CheckoutStayRequestObject) (gen.CheckoutStayResponseObject, error) {
	c, err := s.featureCaller(ctx, s.checkout)
	if err != nil {
		return nil, err
	}
	v, _, err := s.billing.Checkout(ctx, c, req.StayId, req.Params.IdempotencyKey.String())
	if err != nil {
		return nil, err
	}
	return gen.CheckoutStay201JSONResponse(toInvoice(v)), nil
}

func toServices(svcs []app.Service) []gen.Service {
	out := make([]gen.Service, len(svcs))
	for i, s := range svcs {
		out[i] = gen.Service{Code: s.Code, Name: gen.LocalizedText{Vi: s.Name.VI, En: s.Name.EN}, Price: s.Price, Stock: int(s.Stock)}
	}
	return out
}

// toInvoice maps the frozen invoice view; the quote goes through the same mapper as the stay quote.
func toInvoice(v app.InvoiceView) gen.Invoice {
	code := v.BillCode
	return gen.Invoice{
		Id: v.ID, StayId: v.StayID, RoomCode: v.RoomCode, BillCode: &code,
		Status: gen.InvoiceStatus(v.Status), CreatedAt: v.CreatedAt, Quote: toQuote(v.Quote),
	}
}
