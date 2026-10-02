package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pcaokhai/stayguard/api/internal/domain/invoice"
	"github.com/pcaokhai/stayguard/api/internal/domain/payment"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

const (
	routeLinkFmt   = "POST /v1/owner/payment-events/%s/link"
	auditPayLinked = "payment.linked"
)

// LinkTransfer lets the owner say which unpaid invoice an unmatched bank transfer was for (docs/15 rule 1). Only an
// event the bank reported and nobody matched can be linked, once, and only to an open invoice whose balance equals the
// reported amount. The invoice is then settled by the same code as a matched event (settleMatched); the link is audited.
func (p *Payments) LinkTransfer(ctx context.Context, c Caller, eventID, retryID, invoiceID string) (TransactionRow, error) {
	const op = "linkTransferToInvoice"
	if err := p.checkRole(op, c); err != nil {
		return TransactionRow{}, err
	}
	if err := checkKey(retryID); err != nil {
		return TransactionRow{}, err
	}
	if invoiceID == "" {
		return TransactionRow{}, stay.NewValidationError([]stay.FieldError{{Path: "invoiceId", Code: stay.CodeRequired}})
	}
	var out TransactionRow
	err := p.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		stored, ok, err := p.repo.LockEvent(ctx, tx, eventID)
		if err != nil {
			return fmt.Errorf("lock event: %w", err)
		}
		if !ok {
			return ErrNotFound
		}
		route := fmt.Sprintf(routeLinkFmt, eventID)
		oc, err := p.idem.Begin(ctx, tx, route, retryID, RequestHash([]byte(`{"invoiceId":`+quoteJSON(invoiceID)+`}`)))
		if err != nil {
			return err
		}
		if oc.Replay {
			return json.Unmarshal(oc.Body, &out)
		}
		if out, err = p.link(ctx, tx, c, stored, invoiceID); err != nil {
			return err
		}
		body, err := json.Marshal(out)
		if err != nil {
			return fmt.Errorf("encode response: %w", err)
		}
		return p.idem.Complete(ctx, tx, route, retryID, statusOK, body)
	})
	return out, err
}

func quoteJSON(s string) string {
	b, _ := json.Marshal(s) // a string: cannot fail
	return string(b)
}

func (p *Payments) link(ctx context.Context, tx Tx, c Caller, stored StoredEvent, invoiceID string) (TransactionRow, error) {
	if stored.Event.Provider == simProvider { // the demo bank is not the bank: its events are never money
		return TransactionRow{}, stay.NewValidationError([]stay.FieldError{{Path: "eventId", Code: stay.CodePattern}})
	}
	if stored.Result != payment.ResultUnmatched {
		return TransactionRow{}, ErrEventNotLinkable
	}
	inv, ok, err := p.repo.LockInvoice(ctx, tx, invoiceID)
	if err != nil {
		return TransactionRow{}, fmt.Errorf("lock invoice: %w", err)
	}
	if !ok {
		return TransactionRow{}, ErrNotFound
	}
	if inv.Status != string(invoice.StatusOpen) {
		return TransactionRow{}, ErrInvoiceNotOpen
	}
	var q QuoteView
	if err := json.Unmarshal(inv.Quote, &q); err != nil {
		return TransactionRow{}, fmt.Errorf("stored invoice quote: %w", err)
	}
	if q.BalanceDue <= 0 || stored.Event.Amount != q.BalanceDue {
		return TransactionRow{}, stay.NewValidationError([]stay.FieldError{{Path: "invoiceId", Code: "AMOUNT_MISMATCH"}})
	}
	t, err := p.pendingTransferFor(ctx, tx, inv, q.BalanceDue)
	if err != nil {
		return TransactionRow{}, err
	}
	res, err := p.settleMatched(ctx, tx, stored.Event, t, c.UserID)
	if err != nil {
		return TransactionRow{}, err
	}
	after, err := json.Marshal(map[string]any{"eventId": stored.ID, "paymentId": res.PaymentID, "invoiceId": inv.ID, "amount": stored.Event.Amount})
	if err != nil {
		return TransactionRow{}, fmt.Errorf("encode audit: %w", err)
	}
	e := AuditEntry{ID: p.ids.New(auditIDPrefix), ActorID: c.UserID, Action: auditPayLinked, EntityType: "invoice", EntityID: inv.ID, After: after}
	if err := p.audit.Append(ctx, tx, e); err != nil {
		return TransactionRow{}, fmt.Errorf("audit: %w", err)
	}
	return TransactionRow{ID: res.PaymentID, At: storedTime(p.clock.Now()), Amount: stored.Event.Amount, Method: payment.MethodTransfer,
		RoomCode: t.RoomCode, BillCode: inv.BillCode, Reconciliation: "MATCHED", PaymentEventID: stored.ID}, nil
}

// pendingTransferFor returns the invoice's pending transfer, creating it when the guest was never shown a QR.
func (p *Payments) pendingTransferFor(ctx context.Context, tx Tx, inv PayInvoice, amount int64) (PendingTransfer, error) {
	if _, ok, err := p.repo.PendingForInvoice(ctx, tx, inv.ID); err != nil {
		return PendingTransfer{}, fmt.Errorf("pending payment: %w", err)
	} else if !ok {
		n := NewPayment{ID: p.ids.New(paymentIDPrefix), InvoiceID: inv.ID, BillCode: inv.BillCode, Amount: amount, At: storedTime(p.clock.Now())}
		if err := p.repo.InsertPendingTransfer(ctx, tx, n); err != nil {
			return PendingTransfer{}, fmt.Errorf("insert transfer: %w", err)
		}
	}
	pend, err := p.repo.PendingTransfers(ctx, tx)
	if err != nil {
		return PendingTransfer{}, fmt.Errorf("pending transfers: %w", err)
	}
	for _, t := range pend {
		if t.InvoiceID == inv.ID {
			return t, nil
		}
	}
	return PendingTransfer{}, fmt.Errorf("invoice %s has no pending transfer after creating one", inv.ID)
}
