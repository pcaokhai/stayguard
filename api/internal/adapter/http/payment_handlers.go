package httpadapter

import (
	"context"

	"github.com/pcaokhai/stayguard/api/internal/adapter/http/gen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// PaymentService is what the payment handlers need from the application layer (SG-301, SG-302).
type PaymentService interface {
	CreatePayment(ctx context.Context, c app.Caller, invoiceID, retryID, method string) (app.PaymentView, bool, error)
	GetPayment(ctx context.Context, c app.Caller, id string) (app.PaymentView, error)
	Simulate(ctx context.Context, c app.Caller, id string) (app.PaymentView, error)
}

func (s Server) CreatePayment(ctx context.Context, req gen.CreatePaymentRequestObject) (gen.CreatePaymentResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	method := ""
	if req.Body != nil {
		method = string(req.Body.Method)
	}
	v, _, err := s.payments.CreatePayment(ctx, c, req.InvoiceId, req.Params.IdempotencyKey.String(), method)
	if err != nil {
		return nil, err
	}
	return gen.CreatePayment201JSONResponse(toPayment(v)), nil
}

func (s Server) GetPayment(ctx context.Context, req gen.GetPaymentRequestObject) (gen.GetPaymentResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	v, err := s.payments.GetPayment(ctx, c, req.PaymentId)
	if err != nil {
		return nil, err
	}
	return gen.GetPayment200JSONResponse(toPayment(v)), nil
}

func (s Server) SimulatePaymentReceived(ctx context.Context, req gen.SimulatePaymentReceivedRequestObject) (gen.SimulatePaymentReceivedResponseObject, error) {
	if !s.demoEnabled { // before any I/O, like the other demo route
		return nil, app.ErrDemoDisabled
	}
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	v, err := s.payments.Simulate(ctx, c, req.PaymentId)
	if err != nil {
		return nil, err
	}
	return gen.SimulatePaymentReceived200JSONResponse(toPayment(v)), nil
}

func toPayment(v app.PaymentView) gen.Payment {
	out := gen.Payment{Id: v.ID, InvoiceId: v.InvoiceID, Method: gen.PaymentMethod(v.Method), Status: gen.PaymentStatus(v.Status),
		Amount: v.Amount, Remaining: v.Remaining, PaidAt: v.PaidAt, TransactionId: v.TransactionID}
	if v.ReceivedAmount != nil {
		r := *v.ReceivedAmount
		out.ReceivedAmount = &r
	}
	if q := v.QR; q != nil {
		out.Qr = &gen.PaymentQr{Payload: q.Payload, AccountNoMasked: q.AccountNoMasked, AccountName: q.AccountName,
			TransferNote: q.TransferNote, Amount: q.Amount}
	}
	return out
}
