package app

import (
	"context"
	"fmt"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/invoice"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

// GetStay returns a stay with its live quote: at the injected clock while ACTIVE, at check-out after.
func (s *Stays) GetStay(ctx context.Context, c Caller, stayID string) (StayDetail, error) {
	const op = "getStay"
	if err := s.checkRole(op, c); err != nil {
		return StayDetail{}, err
	}
	var out StayDetail
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		rec, ok, err := s.repo.StayByID(ctx, tx, stayID)
		if err != nil {
			return fmt.Errorf("stay: %w", err)
		}
		if !ok {
			return ErrNotFound
		}
		if err := s.checkBuilding(ctx, op, c, rec.BuildingID); err != nil {
			return err
		}
		if out, err = s.detail(ctx, tx, c, rec); err != nil {
			return err
		}
		if rec.Status == string(stay.StatusCheckedOut) {
			return s.finish(ctx, tx, rec, &out)
		}
		return nil
	})
	return out, err
}

func (s *Stays) detail(ctx context.Context, tx Tx, c Caller, rec StayRecord) (StayDetail, error) {
	loc, err := s.zone(ctx, tx)
	if err != nil {
		return StayDetail{}, err
	}
	asOf, err := quoteInstant(rec, s.clock.Now())
	if err != nil {
		return StayDetail{}, err
	}
	return detailFor(rec, asOf, loc)
}

// quoteInstant fails closed on a stored status or a CHECKED_OUT stay without a check-out time.
func quoteInstant(rec StayRecord, now time.Time) (time.Time, error) {
	st, err := stay.ParseStatus(rec.Status)
	if err != nil {
		return time.Time{}, fmt.Errorf("stay status: %w", err)
	}
	if st == stay.StatusActive {
		return now, nil
	}
	if rec.CheckOutAt == nil {
		return time.Time{}, fmt.Errorf("stay %s has no check-out time", rec.ID)
	}
	return *rec.CheckOutAt, nil
}

// Payment states of a checked-out stay.
const (
	PaymentStatePaid            = "PAID"
	PaymentStateAwaitingPayment = "AWAITING_PAYMENT"
	PaymentStateRefundPending   = "REFUND_PENDING"
)

// finish adds what a finished stay shows: the frozen invoice (its quote is the bill, never re-priced), where the payment stands,
// and what is still open.
func (s *Stays) finish(ctx context.Context, tx Tx, rec StayRecord, out *StayDetail) error {
	inv, ok, err := s.repo.InvoiceByStay(ctx, tx, rec.ID)
	if err != nil {
		return fmt.Errorf("invoice: %w", err)
	}
	if !ok {
		return fmt.Errorf("stay %s is checked out without an invoice", rec.ID)
	}
	view, err := invoiceViewOf(inv, rec.RoomCode)
	if err != nil {
		return err
	}
	out.Invoice, out.Quote, out.PaidAt = &view, view.Quote, utcPtr(inv.PaidAt)
	switch {
	case inv.Status == string(invoice.StatusPaid):
		out.PaymentState = PaymentStatePaid
		return nil
	case view.Quote.BalanceDue == 0 && view.Quote.RefundDue > 0:
		out.PaymentState = PaymentStateRefundPending
	default:
		out.PaymentState = PaymentStateAwaitingPayment
	}
	if out.PendingPayment, err = s.repo.PendingPayment(ctx, tx, rec.ID); err != nil {
		return fmt.Errorf("pending payment: %w", err)
	}
	return nil
}
