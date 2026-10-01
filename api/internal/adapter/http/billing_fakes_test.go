package httpadapter

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

const extrasInput = `{"items":[{"serviceCode":"WATER","quantity":2},{"serviceCode":"BEER","quantity":1}]}`

// fakeBilling records the arguments of the last call; forbid fails the test when it is called at all.
type fakeBilling struct {
	forbid   *testing.T
	err      error
	services []app.Service
	detail   app.StayDetail
	invoice  app.InvoiceView
	replayed bool

	calls  int
	caller app.Caller
	stayID string
	key    string
	items  []stay.ExtraInput
}

func (f *fakeBilling) touch(c app.Caller) {
	f.calls++
	f.caller = c
	if f.forbid != nil {
		f.forbid.Errorf("billing use case called although the request must be rejected earlier")
	}
}

func (f *fakeBilling) ListServices(_ context.Context, c app.Caller) ([]app.Service, error) {
	f.touch(c)
	return f.services, f.err
}

func (f *fakeBilling) AddExtras(_ context.Context, c app.Caller, stayID, key string, items []stay.ExtraInput) (app.StayDetail, bool, error) {
	f.touch(c)
	f.stayID, f.key, f.items = stayID, key, items
	return f.detail, f.replayed, f.err
}

func (f *fakeBilling) Checkout(_ context.Context, c app.Caller, stayID, key string) (app.InvoiceView, bool, error) {
	f.touch(c)
	f.stayID, f.key = stayID, key
	return f.invoice, f.replayed, f.err
}

func billingRouter(f *fakeBilling, enabled bool) http.Handler {
	return NewRouter(slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)), Options{
		Probe: readyProbe{}, Sessions: &fakeSessions{}, Billing: f, CheckoutEnabled: enabled,
	})
}

// postBilling sends a POST with a valid token; an empty key omits the Idempotency-Key header.
func postBilling(h http.Handler, path, key, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+goodToken)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func sampleInvoice() app.InvoiceView {
	return app.InvoiceView{
		ID: "iv1", StayID: "st1", RoomCode: "A101", BillCode: "PH1001A101", Status: "OPEN",
		CreatedAt: time.Date(2026, 10, 1, 5, 0, 0, 0, time.UTC),
		Quote: app.QuoteView{
			AsOf: time.Date(2026, 10, 1, 5, 0, 0, 0, time.UTC), StayAmount: 250_000, ExtrasAmount: 20_000, Total: 270_000,
			DepositPaid: 300_000, RefundDue: 30_000, Lines: []app.LineView{{Code: "OVERNIGHT", Quantity: 1, UnitAmount: 250_000, Amount: 250_000}},
		},
	}
}
