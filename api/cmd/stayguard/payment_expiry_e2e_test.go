//go:build integration

package main

import (
	"context"
	"testing"
	"time"
)

func (r payRig) setQRExpiry(minutes int) {
	r.e.exec(`UPDATE app.properties SET qr_expiry_minutes = $2 WHERE tenant_id = $1`, r.tenant, minutes)
}

// TT-15: a pending transfer older than the property's qrExpiryMinutes reads as EXPIRED (derived on read, with the injected clock); the
// stored row is not changed, so money to the old bill code after that is still recorded.
func TestPaymentExpiry_DerivedOnRead_FU(t *testing.T) {
	r := deskRig(t) // creates the pending transfer at the current clock
	r.setQRExpiry(10)
	var id string
	if err := r.e.owner.QueryRow(context.Background(), `SELECT id FROM app.payments WHERE tenant_id = $1 AND invoice_id = $2`, r.tenant, r.invoice).Scan(&id); err != nil {
		t.Fatal(err)
	}
	var created time.Time
	_ = r.e.owner.QueryRow(context.Background(), `SELECT created_at FROM app.payments WHERE id = $1`, id).Scan(&created)

	status := func(at time.Time) (string, bool) {
		r.e.clock.set(at)
		got := r.payment(id)
		_, hasQR := got.body["qr"].(map[string]any)
		return got.str("status"), hasQR
	}
	if s, qr := status(created.Add(10 * time.Minute)); s != "PENDING" || !qr {
		t.Fatalf("exactly at the expiry it is still pending with its QR: %s %v", s, qr)
	}
	if s, qr := status(created.Add(10*time.Minute + time.Second)); s != "EXPIRED" || qr {
		t.Fatalf("a second after: %s (qr shown: %v)", s, qr)
	}
	if r.status("payments", id) != "PENDING" {
		t.Fatal("the stored row must stay PENDING: money to the old bill code is still recorded")
	}
	// A longer expiry applies at once (the property is read each time).
	r.setQRExpiry(240)
	if s, _ := status(created.Add(time.Hour)); s != "PENDING" {
		t.Fatalf("with a 240-minute expiry an hour later is pending: %s", s)
	}
}

// A signed bank transfer with the old bill code after the derived expiry still settles the open invoice.
func TestPaymentExpiry_LateTransferStillSettles_FU(t *testing.T) {
	r := deskRig(t)
	r.setQRExpiry(5)
	r.e.clock.set(r.e.clock.Now().Add(2 * time.Hour))
	res, err := r.handler().Settle(context.Background(), r.event("bank-late", r.balance, r.code))
	if err != nil || res.Result != "SETTLED" || r.status("invoices", r.invoice) != "PAID" {
		t.Fatalf("the late transfer must settle: %+v %v, invoice %s", res, err, r.status("invoices", r.invoice))
	}
}

// After the expiry a new payment can be created; it keeps the bill code and what the bank already sent.
func TestPaymentExpiry_NewPaymentKeepsTheBillCodeAndTheMoneyIn_FU(t *testing.T) {
	r := deskRig(t)
	r.setQRExpiry(10)
	var oldID string
	_ = r.e.owner.QueryRow(context.Background(), `SELECT id FROM app.payments WHERE tenant_id = $1 AND invoice_id = $2`, r.tenant, r.invoice).Scan(&oldID)
	first := r.balance * 3 / 10
	if res, err := r.handler().Settle(context.Background(), r.event("bank-e1", first, r.code)); err != nil || res.Result != "PARTIAL" {
		t.Fatalf("partial: %+v %v", res, err)
	}
	r.e.clock.set(r.e.clock.Now().Add(time.Hour))
	st, p := r.pay("TRANSFER")
	qr, _ := p["qr"].(map[string]any)
	if st != 201 || p["status"] != "PENDING" || p["id"] == oldID || qr == nil || qr["transferNote"] != r.code || num(p["remaining"]) != r.balance-first || num(qr["amount"]) != r.balance-first {
		t.Fatalf("a new payment after the expiry: %d %v", st, p)
	}
	if r.status("payments", oldID) != "EXPIRED" {
		t.Fatalf("the replaced payment is EXPIRED: %s", r.status("payments", oldID))
	}
	// The new payment still settles with the rest.
	if res, err := r.handler().Settle(context.Background(), r.event("bank-e2", r.balance-first, r.code)); err != nil || res.Result != "SETTLED" {
		t.Fatalf("rest: %+v %v", res, err)
	}
}

// An EXPIRED transfer (derived) still says what is owed, from the bank events: the whole balance when nothing arrived, the rest after a
// short transfer. At the boundary it is still pending with the same figure.
func TestPaymentExpiry_RemainingIsWhatIsStillOwed_FU(t *testing.T) {
	r := deskRig(t)
	r.setQRExpiry(10)
	var id string
	var created time.Time
	if err := r.e.owner.QueryRow(context.Background(), `SELECT id, created_at FROM app.payments WHERE tenant_id = $1 AND invoice_id = $2`, r.tenant, r.invoice).Scan(&id, &created); err != nil {
		t.Fatal(err)
	}
	at := func(d time.Duration) map[string]any {
		r.e.clock.set(created.Add(d))
		return r.payment(id).body
	}
	if got := at(10 * time.Minute); got["status"] != "PENDING" || num(got["remaining"]) != r.balance {
		t.Fatalf("at the boundary, pending, whole balance owed: %v", got)
	}
	if got := at(10*time.Minute + time.Second); got["status"] != "EXPIRED" || num(got["remaining"]) != r.balance || got["qr"] != nil {
		t.Fatalf("expired with nothing received owes the whole balance: %v", got)
	}
	first := r.balance * 4 / 10
	if res, err := r.handler().Settle(context.Background(), r.event("bank-x1", first, r.code)); err != nil || res.Result != "PARTIAL" {
		t.Fatalf("partial: %+v %v", res, err)
	}
	if got := at(time.Hour); got["status"] != "EXPIRED" || num(got["remaining"]) != r.balance-first || num(got["receivedAmount"]) != first {
		t.Fatalf("expired after a short transfer owes the rest (%d): %v", r.balance-first, got)
	}
}
