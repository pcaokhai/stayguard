package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/invoice"
	"github.com/pcaokhai/stayguard/api/internal/domain/payment"
	"github.com/pcaokhai/stayguard/api/internal/domain/shift"
)

const (
	paymentIDPrefix = "pm"
	eventIDPrefix   = "pe"
	simProvider     = "simulator"
	entityPayment   = "payment"
	auditPayCreated = "payment.created"
	auditPaySettled = "payment.settled"
	routePayFmt     = "POST /v1/invoices/%s/payments"
	maskKeep        = 4
	// maxAlertNoteRunes bounds the transfer note copied into an alert.
	maxAlertNoteRunes = 100
)

// PaymentQR is what the client renders as a QR image; the account is always the tenant's own.
type PaymentQR struct {
	Payload         string `json:"payload"`
	AccountNoMasked string `json:"accountNoMasked"`
	AccountName     string `json:"accountName"`
	TransferNote    string `json:"transferNote"`
	Amount          int64  `json:"amount"`
}

// PaymentView is the answer of createPayment, getPayment and simulatePaymentReceived; it is also the stored
// idempotency response body.
type PaymentView struct {
	ID             string     `json:"id"`
	InvoiceID      string     `json:"invoiceId"`
	Method         string     `json:"method"`
	Status         string     `json:"status"`
	Amount         int64      `json:"amount"`
	ReceivedAmount *int64     `json:"receivedAmount"`
	Remaining      int64      `json:"remaining"`
	PaidAt         *time.Time `json:"paidAt"`
	TransactionID  *string    `json:"transactionId"`
	QR             *PaymentQR `json:"qr"`
}

// SettleResult is the outcome of one payment event.
type SettleResult struct{ Result, PaymentID string }

// Payments holds the payment use cases and the one settlement handler (SG-301, SG-302).
type Payments struct {
	uow   UnitOfWork
	repo  PaymentRepo
	enc   Encryptor
	idem  IdempotencyStore
	audit AuditWriter
	ids   IDGenerator
	clock Clock
	// alerts is optional (nil raises nothing): PAYMENT_MISMATCH and UNMATCHED_TRANSFER for the owner (SG-801).
	alerts AlertWriter
	// cash is optional (nil records nothing): cash taken and refunded goes on the drawer ledger (SG-503).
	cash CashLedger
	guard
}

// WithCash puts cash payments and refunds on the receptionist's drawer ledger.
func (p *Payments) WithCash(l CashLedger) *Payments { p.cash = l; return p }

// WithAlerts turns on the owner alerts for bank events that cannot settle an invoice.
func (p *Payments) WithAlerts(a AlertWriter) *Payments { p.alerts = a; return p }

func NewPayments(uow UnitOfWork, repo PaymentRepo, levels BuildingLevels, enc Encryptor, idem IdempotencyStore,
	audit AuditWriter, ids IDGenerator, clock Clock) *Payments {
	return &Payments{uow: uow, repo: repo, enc: enc, idem: idem, audit: audit, ids: ids, clock: clock, guard: guard{levels: levels}}
}

// CreatePayment starts a payment for an open invoice. CASH settles at once; TRANSFER stays PENDING until a
// payment event settles it. Both are for the invoice's frozen balance, never for a caller-supplied amount.
func (p *Payments) CreatePayment(ctx context.Context, c Caller, invoiceID, retryID, method string) (PaymentView, bool, error) {
	const op = "createPayment"
	if err := p.checkRole(op, c); err != nil {
		return PaymentView{}, false, err
	}
	if err := checkKey(retryID); err != nil {
		return PaymentView{}, false, err
	}
	m, err := payment.ParseMethod(method)
	if err != nil {
		return PaymentView{}, false, &ValidationError{"method", "must be CASH or TRANSFER"}
	}
	var out PaymentView
	var replayed bool
	err = p.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		inv, ok, err := p.repo.LockInvoice(ctx, tx, invoiceID)
		if err != nil {
			return fmt.Errorf("lock invoice: %w", err)
		}
		if !ok {
			return ErrNotFound
		}
		if err := p.checkBuilding(ctx, op, c, inv.BuildingID); err != nil {
			return err
		}
		route := fmt.Sprintf(routePayFmt, invoiceID)
		oc, err := p.idem.Begin(ctx, tx, route, retryID, RequestHash([]byte(`{"method":"`+m+`"}`)))
		if err != nil {
			return err
		}
		if oc.Replay {
			replayed = true
			return json.Unmarshal(oc.Body, &out)
		}
		if out, err = p.create(ctx, tx, c, inv, m); err != nil {
			return err
		}
		body, err := json.Marshal(out)
		if err != nil {
			return fmt.Errorf("encode response: %w", err)
		}
		return p.idem.Complete(ctx, tx, route, retryID, statusCreated, body)
	})
	return out, replayed, err
}

func (p *Payments) create(ctx context.Context, tx Tx, c Caller, inv PayInvoice, method string) (PaymentView, error) {
	if inv.Status != string(invoice.StatusOpen) {
		return PaymentView{}, ErrInvoiceNotOpen
	}
	var q QuoteView
	if err := json.Unmarshal(inv.Quote, &q); err != nil {
		return PaymentView{}, fmt.Errorf("stored invoice quote: %w", err)
	}
	now := storedTime(p.clock.Now())
	if method == payment.MethodTransfer {
		return p.createTransfer(ctx, tx, c, inv, q.BalanceDue, now)
	}
	if err := p.repo.ExpirePending(ctx, tx, inv.ID); err != nil { // a late transfer must not settle a closed invoice
		return PaymentView{}, fmt.Errorf("expire pending: %w", err)
	}
	got, err := p.repo.ReceivedForInvoice(ctx, tx, inv.ID) // bank money already matched to this bill
	if err != nil {
		return PaymentView{}, fmt.Errorf("received for invoice: %w", err)
	}
	due := max(q.BalanceDue-got, 0) // cash pays only what the bank has not
	n := NewPayment{ID: p.ids.New(paymentIDPrefix), InvoiceID: inv.ID, BillCode: inv.BillCode, Amount: due, At: now}
	if err := p.repo.InsertCashPayment(ctx, tx, n); err != nil {
		return PaymentView{}, fmt.Errorf("insert cash payment: %w", err)
	}
	if err := p.repo.CloseInvoice(ctx, tx, inv.ID, inv.StayID, now); err != nil {
		return PaymentView{}, err
	}
	if err := p.repo.SettleInvoiceEvents(ctx, tx, inv.ID); err != nil {
		return PaymentView{}, err
	}
	if err := p.auditPayment(ctx, tx, c.UserID, auditPaySettled, n.ID, payment.MethodCash, inv.ID, due); err != nil {
		return PaymentView{}, err
	}
	if err := p.recordCash(ctx, tx, c, inv, n.ID, due, q.RefundDue); err != nil {
		return PaymentView{}, err
	}
	paid := now
	recv := due
	return PaymentView{ID: n.ID, InvoiceID: inv.ID, Method: payment.MethodCash, Status: payment.StatusPaid,
		Amount: due, ReceivedAmount: &recv, PaidAt: &paid}, nil
}

// recordCash writes what the drawer received (the balance) and gave back (a deposit refund) to the ledger.
func (p *Payments) recordCash(ctx context.Context, tx Tx, c Caller, inv PayInvoice, paymentID string, due, refund int64) error {
	if p.cash == nil {
		return nil
	}
	for _, e := range []CashRecord{
		{Kind: shift.Payment, StayID: inv.StayID, PaymentID: paymentID, Amount: due},
		{Kind: shift.Refund, StayID: inv.StayID, PaymentID: paymentID, Amount: refund},
	} {
		if err := p.cash.Record(ctx, tx, c, e); err != nil {
			return fmt.Errorf("record cash: %w", err)
		}
	}
	return nil
}

func (p *Payments) createTransfer(ctx context.Context, tx Tx, c Caller, inv PayInvoice, amount int64, now time.Time) (PaymentView, error) {
	if amount <= 0 {
		return PaymentView{}, &ValidationError{"method", "nothing to pay by transfer"}
	}
	if pend, ok, err := p.repo.PendingForInvoice(ctx, tx, inv.ID); err != nil {
		return PaymentView{}, fmt.Errorf("pending payment: %w", err)
	} else if ok { // one pending transfer per invoice: show the same QR again
		return p.view(ctx, tx, pend)
	}
	n := NewPayment{ID: p.ids.New(paymentIDPrefix), InvoiceID: inv.ID, BillCode: inv.BillCode, Amount: amount, At: now}
	if err := p.repo.InsertPendingTransfer(ctx, tx, n); err != nil {
		return PaymentView{}, fmt.Errorf("insert transfer: %w", err)
	}
	if err := p.auditPayment(ctx, tx, c.UserID, auditPayCreated, n.ID, payment.MethodTransfer, inv.ID, amount); err != nil {
		return PaymentView{}, err
	}
	return p.view(ctx, tx, PaymentRecord{ID: n.ID, InvoiceID: inv.ID, Method: payment.MethodTransfer,
		Status: payment.StatusPending, Amount: amount, BillCode: inv.BillCode})
}

// GetPayment is the polling read the web app calls every 3 seconds.
func (p *Payments) GetPayment(ctx context.Context, c Caller, id string) (PaymentView, error) {
	const op = "getPayment"
	if err := p.checkRole(op, c); err != nil {
		return PaymentView{}, err
	}
	var out PaymentView
	err := p.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		rec, err := p.load(ctx, tx, op, c, id)
		if err != nil {
			return err
		}
		out, err = p.view(ctx, tx, rec)
		return err
	})
	return out, err
}

// Simulate is the demo bank: it reports the exact amount and bill code of a pending transfer as received and
// runs the same handler a provider webhook will. The route is gated by DEMO_MODE in the HTTP layer.
func (p *Payments) Simulate(ctx context.Context, c Caller, id string) (PaymentView, error) {
	const op = "simulatePaymentReceived"
	if err := p.checkRole(op, c); err != nil {
		return PaymentView{}, err
	}
	var out PaymentView
	err := p.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		rec, err := p.load(ctx, tx, op, c, id)
		if err != nil {
			return err
		}
		if rec.Method == payment.MethodTransfer && rec.Status == payment.StatusPending {
			ev := PaymentEvent{TenantID: c.TenantID, Provider: simProvider, ExternalID: p.ids.New("sim"),
				Content: rec.BillCode, Amount: rec.Amount, ReceivedAt: storedTime(p.clock.Now())}
			if _, err := p.settle(ctx, tx, ev); err != nil {
				return err
			}
			if rec, err = p.load(ctx, tx, op, c, id); err != nil {
				return err
			}
		}
		out, err = p.view(ctx, tx, rec)
		return err
	})
	return out, err
}

// RecordIgnored stores a bank event that must not settle anything (an outgoing transfer, or money to an account that
// is not the tenant's). The row says UNMATCHED, deduplicated like every event; no alert is raised.
func (p *Payments) RecordIgnored(ctx context.Context, ev PaymentEvent) error {
	return p.uow.Do(ctx, ev.TenantID, func(ctx context.Context, tx Tx) error {
		_, err := p.repo.InsertEvent(ctx, tx, p.ids.New(eventIDPrefix), ev)
		return err
	})
}

// Settle is the one settlement handler. Every transfer becomes PAID here and nowhere else.
func (p *Payments) Settle(ctx context.Context, ev PaymentEvent) (SettleResult, error) {
	var out SettleResult
	err := p.uow.Do(ctx, ev.TenantID, func(ctx context.Context, tx Tx) error {
		var err error
		out, err = p.settle(ctx, tx, ev)
		return err
	})
	return out, err
}

func (p *Payments) settle(ctx context.Context, tx Tx, ev PaymentEvent) (SettleResult, error) {
	fresh, err := p.repo.InsertEvent(ctx, tx, p.ids.New(eventIDPrefix), ev)
	if err != nil {
		return SettleResult{}, fmt.Errorf("insert payment event: %w", err)
	}
	if !fresh {
		return SettleResult{Result: payment.ResultDuplicate}, nil
	}
	pend, err := p.repo.PendingTransfers(ctx, tx)
	if err != nil {
		return SettleResult{}, fmt.Errorf("pending transfers: %w", err)
	}
	codes := make([]string, len(pend))
	for i, t := range pend {
		codes[i] = t.BillCode
	}
	ix, found := payment.FindBillCode(ev.Content, codes)
	if !found {
		a := AlertDraft{Kind: AlertUnmatchedTransfer, Amount: &ev.Amount, Details: map[string]string{"transferNote": clip(ev.Content)}}
		if err := p.raise(ctx, tx, a); err != nil {
			return SettleResult{}, err
		}
		return SettleResult{Result: payment.ResultUnmatched}, nil // the event row already says UNMATCHED
	}
	t := pend[ix]
	before, err := p.repo.ReceivedForInvoice(ctx, tx, t.InvoiceID)
	if err != nil {
		return SettleResult{}, fmt.Errorf("received for invoice: %w", err)
	}
	total := before + ev.Amount // bank money accumulates on the bill code until it covers the invoice
	if total >= t.Amount {
		if total > t.Amount {
			excess := total - t.Amount
			a := AlertDraft{Kind: AlertOverpaid, RoomCode: t.RoomCode, StayID: t.StayID, Amount: &excess,
				Details: map[string]string{"billCode": t.BillCode, "expected": strconv.FormatInt(t.Amount, 10),
					"received": strconv.FormatInt(total, 10), "excess": strconv.FormatInt(excess, 10)}}
			if err := p.raise(ctx, tx, a); err != nil {
				return SettleResult{}, err
			}
		}
		return p.settleMatched(ctx, tx, ev, t, "", total)
	}
	if err := p.repo.SetEventMatched(ctx, tx, ev, t.InvoiceID, payment.ResultPartial); err != nil {
		return SettleResult{}, fmt.Errorf("set event matched: %w", err)
	}
	if err := p.repo.SetTransferReceived(ctx, tx, t.PaymentID, total); err != nil {
		return SettleResult{}, err
	}
	a := AlertDraft{Kind: AlertPaymentMismatch, RoomCode: t.RoomCode, StayID: t.StayID, Amount: &ev.Amount,
		Details: map[string]string{"billCode": t.BillCode, "expected": strconv.FormatInt(t.Amount, 10),
			"received": strconv.FormatInt(total, 10)}}
	if err := p.raise(ctx, tx, a); err != nil {
		return SettleResult{}, err
	}
	return SettleResult{Result: payment.ResultPartial, PaymentID: t.PaymentID}, nil
}

// settleMatched is the tail of settlement: the pending transfer becomes PAID, the invoice closes and the event is
// SETTLED. The bank-event handler and the owner's link of an unmatched transfer (linkTransferToInvoice) both end here.
// actor is empty for a provider event.
func (p *Payments) settleMatched(ctx context.Context, tx Tx, ev PaymentEvent, t PendingTransfer, actor string, received int64) (SettleResult, error) {
	now := storedTime(p.clock.Now()) // payment time is the server's, not the provider's
	if err := p.repo.SettleTransfer(ctx, tx, t.PaymentID, now, received, ev.ExternalID); err != nil {
		return SettleResult{}, err
	}
	if err := p.repo.CloseInvoice(ctx, tx, t.InvoiceID, t.StayID, now); err != nil {
		return SettleResult{}, err
	}
	if err := p.auditPayment(ctx, tx, actor, auditPaySettled, t.PaymentID, payment.MethodTransfer, t.InvoiceID, received); err != nil {
		return SettleResult{}, err
	}
	if err := p.repo.SetEventMatched(ctx, tx, ev, t.InvoiceID, payment.ResultSettled); err != nil {
		return SettleResult{}, fmt.Errorf("set event matched: %w", err)
	}
	if err := p.repo.SettleInvoiceEvents(ctx, tx, t.InvoiceID); err != nil {
		return SettleResult{}, err
	}
	return SettleResult{Result: payment.ResultSettled, PaymentID: t.PaymentID}, nil
}

// raise writes an alert in the settlement transaction; without an alert writer it does nothing.
func (p *Payments) raise(ctx context.Context, tx Tx, a AlertDraft) error {
	if p.alerts == nil {
		return nil
	}
	a.ID = p.ids.New(alertIDPrefix)
	if err := p.alerts.Raise(ctx, tx, a); err != nil {
		return fmt.Errorf("raise alert: %w", err)
	}
	return nil
}

// clip bounds a bank transfer note shown in an alert.
func clip(s string) string {
	if r := []rune(s); len(r) > maxAlertNoteRunes {
		return string(r[:maxAlertNoteRunes])
	}
	return s
}

// load finds a payment (a foreign or unknown id is a 404 before it can be a 403) and checks the building.
func (p *Payments) load(ctx context.Context, tx Tx, op string, c Caller, id string) (PaymentRecord, error) {
	rec, ok, err := p.repo.PaymentByID(ctx, tx, id)
	if err != nil {
		return PaymentRecord{}, fmt.Errorf("payment: %w", err)
	}
	if !ok {
		return PaymentRecord{}, ErrNotFound
	}
	return rec, p.checkBuilding(ctx, op, c, rec.BuildingID)
}

// view adds the QR to a pending transfer; the account is the tenant's default, never taken from the request.
func (p *Payments) view(ctx context.Context, tx Tx, r PaymentRecord) (PaymentView, error) {
	v := PaymentView{ID: r.ID, InvoiceID: r.InvoiceID, Method: r.Method, Status: r.Status, Amount: r.Amount,
		ReceivedAmount: r.ReceivedAmount, PaidAt: r.PaidAt, TransactionID: r.TransactionID}
	if r.Method != payment.MethodTransfer || r.Status != payment.StatusPending {
		return v, nil
	}
	if r.ReceivedAmount != nil {
		v.Remaining = max(r.Amount-*r.ReceivedAmount, 0)
	} else {
		v.Remaining = r.Amount
	}
	r.Amount = v.Remaining // the QR asks for what is still owed, with the same bill code
	qr, err := p.qr(ctx, tx, r)
	v.QR = qr
	return v, err
}

func (p *Payments) qr(ctx context.Context, tx Tx, r PaymentRecord) (*PaymentQR, error) {
	id, raw, err := p.repo.DefaultBankAccount(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("bank account: %w", err)
	}
	if id == "" || len(raw) == 0 {
		return nil, ErrNoBankAccount
	}
	plain, err := p.enc.Decrypt(tx.TenantID(), bankAccountFieldOf(id), raw)
	if err != nil {
		return nil, fmt.Errorf("decrypt bank account: %w", err)
	}
	var acc struct{ BankBin, AccountNo, AccountName string }
	if err := json.Unmarshal(plain, &acc); err != nil {
		return nil, fmt.Errorf("bank account: %w", err)
	}
	payload, err := payment.QRPayload(acc.BankBin, acc.AccountNo, r.Amount, r.BillCode)
	if err != nil {
		return nil, fmt.Errorf("qr: %w", err)
	}
	return &PaymentQR{Payload: payload, AccountNoMasked: maskAccount(acc.AccountNo), AccountName: acc.AccountName,
		TransferNote: r.BillCode, Amount: r.Amount}, nil
}

func maskAccount(no string) string {
	if len(no) <= maskKeep {
		return no
	}
	return strings.Repeat("*", len(no)-maskKeep) + no[len(no)-maskKeep:]
}

func (p *Payments) auditPayment(ctx context.Context, tx Tx, actor, action, paymentID, method, invoiceID string, amount int64) error {
	raw, err := json.Marshal(map[string]any{"invoiceId": invoiceID, "method": method, "amount": amount})
	if err != nil {
		return fmt.Errorf("encode audit: %w", err)
	}
	e := AuditEntry{ID: p.ids.New(auditIDPrefix), ActorID: actor, Action: action, EntityType: entityPayment, EntityID: paymentID, After: raw}
	if err := p.audit.Append(ctx, tx, e); err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	return nil
}
