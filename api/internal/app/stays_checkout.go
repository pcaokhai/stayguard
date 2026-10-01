package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/invoice"
	"github.com/pcaokhai/stayguard/api/internal/domain/money"
	"github.com/pcaokhai/stayguard/api/internal/domain/pricing"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

const (
	auditCheckOut    = "stay.checked_out"
	routeCheckoutFmt = "POST /v1/stays/%s/checkout"
	// maxBillCodeAttempts bounds the probe loop. The code has no year (contract example), so a busy room
	// accumulates suffixes over the years; more than this is a fault.
	maxBillCodeAttempts = 200
	// maxCheckoutTries re-runs the whole unit of work when a racing check-out took the probed bill code.
	maxCheckoutTries = 3
)

// emptyBodyHash is the request hash of a check-out: it has no body.
var emptyBodyHash = RequestHash([]byte("{}"))

func checkoutRoute(stayID string) string { return fmt.Sprintf(routeCheckoutFmt, stayID) }

// Checkout ends an ACTIVE stay: it freezes the quote into an invoice with a bill code. A stay that is
// already CHECKED_OUT returns its invoice, also under a new key. The room is not touched: it stays
// OCCUPIED until the invoice is paid (SG-302).
func (b *Billing) Checkout(ctx context.Context, c Caller, stayID, retryID string) (InvoiceView, bool, error) {
	const op = "checkoutStay"
	if err := b.checkRole(op, c); err != nil {
		return InvoiceView{}, false, err
	}
	if err := checkKey(retryID); err != nil {
		return InvoiceView{}, false, err
	}
	for try := 1; ; try++ {
		out, replayed, err := b.checkoutOnce(ctx, c, stayID, retryID)
		if errors.Is(err, ErrBillCodeConflict) && try < maxCheckoutTries {
			continue // rolled back: the key is gone and the probe runs again
		}
		if errors.Is(err, ErrBillCodeConflict) {
			return InvoiceView{}, false, fmt.Errorf("check-out: no bill code after %d tries: %w", try, err)
		}
		return out, replayed, err
	}
}

func (b *Billing) checkoutOnce(ctx context.Context, c Caller, stayID, retryID string) (InvoiceView, bool, error) {
	const op = "checkoutStay"
	var out InvoiceView
	var replayed bool
	err := b.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		route := checkoutRoute(stayID)
		oc, err := b.begin(ctx, tx, op, c, stayID, route, retryID, emptyBodyHash)
		if err != nil {
			return err
		}
		if oc.Replay {
			replayed = true
			return json.Unmarshal(oc.Body, &out)
		}
		if out, err = b.checkoutLocked(ctx, tx, c, stayID); err != nil {
			return err
		}
		return b.complete(ctx, tx, route, retryID, statusCreated, out)
	})
	return out, replayed, err
}

func (b *Billing) checkoutLocked(ctx context.Context, tx Tx, c Caller, stayID string) (InvoiceView, error) {
	rec, ok, err := b.stays.LockStay(ctx, tx, stayID)
	if err != nil {
		return InvoiceView{}, fmt.Errorf("lock stay: %w", err)
	}
	if !ok {
		return InvoiceView{}, ErrNotFound
	}
	status, err := stay.ParseStatus(rec.Status)
	if err != nil {
		return InvoiceView{}, fmt.Errorf("stay status: %w", err)
	}
	if status == stay.StatusCheckedOut {
		return b.existingInvoice(ctx, tx, rec)
	}
	return b.freeze(ctx, tx, c, rec)
}

func (b *Billing) existingInvoice(ctx context.Context, tx Tx, rec StayRecord) (InvoiceView, error) {
	inv, ok, err := b.stays.InvoiceByStay(ctx, tx, rec.ID)
	if err != nil {
		return InvoiceView{}, fmt.Errorf("invoice: %w", err)
	}
	if !ok { // a checked-out stay always has one: this is a server fault, not a 404
		return InvoiceView{}, fmt.Errorf("stay %s is checked out without an invoice", rec.ID)
	}
	return invoiceViewOf(inv, rec.RoomCode)
}

// freeze prices the stay at the server clock, stores the invoice and marks the stay CHECKED_OUT.
func (b *Billing) freeze(ctx context.Context, tx Tx, c Caller, rec StayRecord) (InvoiceView, error) {
	at := b.clock.Now()
	loc, err := loadZone(ctx, tx, b.stays)
	if err != nil {
		return InvoiceView{}, err
	}
	quote, err := billFor(rec, at, loc)
	if err != nil {
		return InvoiceView{}, err
	}
	code, err := b.nextBillCode(ctx, tx, at.In(loc), rec.RoomCode)
	if err != nil {
		return InvoiceView{}, err
	}
	raw, err := json.Marshal(quote)
	if err != nil {
		return InvoiceView{}, fmt.Errorf("encode quote: %w", err)
	}
	n := NewInvoice{ID: b.ids.New(invoiceIDPrefix), StayID: rec.ID, BillCode: code, Quote: raw, Total: quote.Total, CreatedAt: at}
	if err := b.stays.InsertInvoice(ctx, tx, n); err != nil {
		return InvoiceView{}, fmt.Errorf("insert invoice: %w", err)
	}
	if err := b.stays.MarkCheckedOut(ctx, tx, rec.ID, at); err != nil {
		return InvoiceView{}, fmt.Errorf("check out stay: %w", err) // keeps stay.ErrNotActive visible
	}
	after := map[string]any{"stayId": rec.ID, "invoiceId": n.ID, "billCode": code, "status": stay.StatusCheckedOut,
		"total": quote.Total, "balanceDue": quote.BalanceDue, "refundDue": quote.RefundDue}
	if err := b.auditAppend(ctx, tx, c, auditCheckOut, entityStay, rec.ID, after); err != nil {
		return InvoiceView{}, err
	}
	return InvoiceView{ID: n.ID, StayID: rec.ID, RoomCode: rec.RoomCode, BillCode: code, Status: string(invoice.StatusOpen),
		CreatedAt: at.UTC(), Quote: quote}, nil
}

// nextBillCode probes exact candidates (attempt 1, 2, ...) until one is free. It runs under the stay lock
// and the table's unique constraint is the backstop for two stays racing on one room.
func (b *Billing) nextBillCode(ctx context.Context, tx Tx, day time.Time, roomCode string) (string, error) {
	for attempt := 1; attempt <= maxBillCodeAttempts; attempt++ {
		code, err := invoice.BillCode(day, roomCode, attempt)
		if err != nil {
			return "", fmt.Errorf("bill code: %w", err)
		}
		taken, err := b.stays.BillCodeTaken(ctx, tx, code)
		if err != nil {
			return "", fmt.Errorf("probe bill code: %w", err)
		}
		if !taken {
			return code, nil
		}
	}
	return "", fmt.Errorf("bill code: no free code in %d attempts", maxBillCodeAttempts)
}

// billFor prices check-in to at from the stay's own snapshot (never the current rate plan) and
// settles extras and deposit. QuoteRunning equals Price whenever at is after check-in.
func billFor(rec StayRecord, at time.Time, loc *time.Location) (QuoteView, error) {
	plan, err := pricing.ParseRatePlan(rec.RatePlanSnapshot)
	if err != nil {
		return QuoteView{}, fmt.Errorf("stay rate plan snapshot: %w", err)
	}
	rental, err := pricing.ParseRentalType(rec.RentalType)
	if err != nil {
		return QuoteView{}, errCorruptStoredRentalType
	}
	q, err := pricing.QuoteRunning(plan, rental, rec.CheckInAt, at, loc)
	if err != nil {
		return QuoteView{}, fmt.Errorf("stay quote: %w", err)
	}
	extras, _, err := extrasFor(rec.Extras)
	if err != nil {
		return QuoteView{}, err
	}
	deposit, err := money.NewVnd(rec.Deposit)
	if err != nil {
		return QuoteView{}, fmt.Errorf("stay deposit: %w", err)
	}
	bill, err := pricing.Assemble(q, extras, deposit)
	if err != nil {
		return QuoteView{}, fmt.Errorf("stay bill: %w", err)
	}
	return quoteViewOf(at, q, bill, rec.Deposit), nil
}
