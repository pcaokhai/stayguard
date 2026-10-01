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
)

const (
	idemKey   = "3f2b8c1e-5d4a-4b7e-9c1d-2a6f8e0b7c11"
	stayInput = `{"rentalType":"OVERNIGHT","guestName":"Nguyen An","guestPhone":"0900000001","idNumber":"ABC123456","deposit":100000}`
)

// fakeStays records the arguments of the last call; forbid fails the test when it is called at all.
type fakeStays struct {
	forbid   *testing.T
	err      error
	detail   app.StayDetail
	replayed bool

	calls   int
	caller  app.Caller
	roomID  string
	key     string
	input   app.CreateStayInput
	gotStay string
}

func (f *fakeStays) touch(c app.Caller) {
	f.calls++
	f.caller = c
	if f.forbid != nil {
		f.forbid.Errorf("stays use case called although the request must be rejected earlier")
	}
}

func (f *fakeStays) CreateStay(_ context.Context, c app.Caller, roomID, key string, in app.CreateStayInput) (app.StayDetail, bool, error) {
	f.touch(c)
	f.roomID, f.key, f.input = roomID, key, in
	return f.detail, f.replayed, f.err
}

func (f *fakeStays) GetStay(_ context.Context, c app.Caller, stayID string) (app.StayDetail, error) {
	f.touch(c)
	f.gotStay = stayID
	return f.detail, f.err
}

func stayRouter(f *fakeStays, enabled bool) http.Handler {
	return NewRouter(slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)), Options{
		Probe: readyProbe{}, Sessions: &fakeSessions{}, Stays: f, CheckInEnabled: enabled,
	})
}

// postStay sends a createStay request; an empty key omits the header.
func postStay(h http.Handler, roomID, key, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/v1/rooms/"+roomID+"/stays", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+goodToken)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func sampleStay(full bool) app.StayDetail {
	d := app.StayDetail{
		ID: "st1", RoomID: "r1", RoomCode: "A101", RentalType: "OVERNIGHT", Status: "ACTIVE",
		CheckInAt: time.Date(2026, 10, 1, 1, 30, 0, 0, time.UTC), GuestName: "Nguyen An", GuestPhone: "0900000001",
		Deposit: 100_000, Extras: []app.ExtraView{}, PricingVersion: 3,
		Quote: app.QuoteView{
			AsOf: time.Date(2026, 10, 1, 1, 30, 0, 0, time.UTC), StayAmount: 250_000, Total: 250_000,
			DepositPaid: 100_000, BalanceDue: 150_000, Lines: []app.LineView{{Code: "OVERNIGHT", Quantity: 1, UnitAmount: 250_000, Amount: 250_000}},
		},
	}
	if full {
		masked, out := "*****456", time.Date(2026, 10, 1, 5, 0, 0, 0, time.UTC)
		d.IDNumberMasked, d.CheckOutAt, d.Status = &masked, &out, "CHECKED_OUT"
		d.Extras = []app.ExtraView{{ServiceCode: "sv1", Name: app.LocalizedName{VI: "Nuoc", EN: "Water"}, Quantity: 2, UnitAmount: 10_000, Amount: 20_000}}
		d.Quote.ExtrasAmount, d.Quote.Total, d.Quote.BalanceDue, d.Quote.Capped = 20_000, 270_000, 170_000, true
	}
	return d
}
