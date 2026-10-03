package httpadapter

import (
	"github.com/pcaokhai/stayguard/api/internal/adapter/http/gen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// toStay maps the app view to the contract type. A fresh and a replayed answer both come through
// here, so a replay is byte-identical. Only the masked ID number exists in the view.
func toStay(d app.StayDetail) gen.Stay {
	extras := make([]gen.ExtraLine, len(d.Extras))
	for i, e := range d.Extras {
		extras[i] = gen.ExtraLine{
			Amount: e.Amount, Name: gen.LocalizedText{Vi: e.Name.VI, En: e.Name.EN},
			Quantity: int(e.Quantity), ServiceCode: e.ServiceCode, UnitAmount: e.UnitAmount,
		}
	}
	out := gen.Stay{
		Id: d.ID, RoomId: d.RoomID, RoomCode: d.RoomCode, RentalType: gen.RentalType(d.RentalType),
		Status: gen.StayStatus(d.Status), CheckInAt: d.CheckInAt, CheckOutAt: d.CheckOutAt,
		GuestName: d.GuestName, GuestPhone: d.GuestPhone, GuestId: &gen.GuestIdIndicators{HasIdNumber: d.GuestID.HasIDNumber, HasFrontPhoto: d.GuestID.HasFrontPhoto, HasBackPhoto: d.GuestID.HasBackPhoto},
		Deposit: d.Deposit, Extras: extras, Quote: toQuote(d.Quote), PricingVersion: d.PricingVersion,
		PendingPayment: toPendingPayment(d.PendingPayment), PaidAt: d.PaidAt,
	}
	if d.Invoice != nil {
		inv := toInvoice(*d.Invoice)
		out.Invoice = &inv
		out.InvoiceId, out.BillCode = &inv.Id, inv.BillCode
		state := gen.StayPaymentState(d.PaymentState)
		out.PaymentState = &state
	}
	return out
}

func toPendingPayment(p *app.PendingPayment) *gen.PendingPayment {
	if p == nil {
		return nil
	}
	out := &gen.PendingPayment{Total: p.Total, Deposit: p.Deposit, Received: p.Received, Remaining: p.Remaining, RefundDue: p.RefundDue, CreatedAt: p.CreatedAt}
	if p.PaymentID != "" {
		out.PaymentId = &p.PaymentID
	}
	return out
}

func toQuote(q app.QuoteView) gen.Quote {
	lines := make([]gen.BillLine, len(q.Lines))
	for i, l := range q.Lines {
		lines[i] = gen.BillLine{Code: l.Code, Quantity: int(l.Quantity), UnitAmount: l.UnitAmount, Amount: l.Amount}
	}
	return gen.Quote{
		AsOf: q.AsOf, StayAmount: q.StayAmount, ExtrasAmount: q.ExtrasAmount, Total: q.Total,
		DepositPaid: q.DepositPaid, BalanceDue: q.BalanceDue, RefundDue: q.RefundDue, Capped: q.Capped, Lines: lines,
	}
}
