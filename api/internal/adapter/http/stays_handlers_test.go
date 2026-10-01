package httpadapter

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/pricing"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

const (
	wantStayMinimal = `{"checkInAt":"2026-10-01T01:30:00Z","deposit":100000,"extras":[],"guestName":"Nguyen An","guestPhone":"0900000001","id":"st1","pricingVersion":3,"quote":{"asOf":"2026-10-01T01:30:00Z","balanceDue":150000,"capped":false,"depositPaid":100000,"extrasAmount":0,"lines":[{"amount":250000,"code":"OVERNIGHT","quantity":1,"unitAmount":250000}],"refundDue":0,"stayAmount":250000,"total":250000},"rentalType":"OVERNIGHT","roomCode":"A101","roomId":"r1","status":"ACTIVE"}`
	wantStayFull    = `{"checkInAt":"2026-10-01T01:30:00Z","checkOutAt":"2026-10-01T05:00:00Z","deposit":100000,"extras":[{"amount":20000,"name":{"en":"Water","vi":"Nuoc"},"quantity":2,"serviceCode":"sv1","unitAmount":10000}],"guestName":"Nguyen An","guestPhone":"0900000001","id":"st1","idNumberMasked":"*****456","pricingVersion":3,"quote":{"asOf":"2026-10-01T01:30:00Z","balanceDue":170000,"capped":true,"depositPaid":100000,"extrasAmount":20000,"lines":[{"amount":250000,"code":"OVERNIGHT","quantity":1,"unitAmount":250000}],"refundDue":0,"stayAmount":250000,"total":270000},"rentalType":"OVERNIGHT","roomCode":"A101","roomId":"r1","status":"CHECKED_OUT"}`
)

func TestCreateStayHTTP_SG203_AC1(t *testing.T) {
	f := &fakeStays{detail: sampleStay(false)}
	h := stayRouter(f, true)

	rec := postStay(h, "r1", idemKey, stayInput)
	if rec.Code != 201 || strings.TrimSpace(rec.Body.String()) != wantStayMinimal {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body)
	}
	wantIn := app.CreateStayInput{RentalType: "OVERNIGHT", GuestName: "Nguyen An", GuestPhone: "0900000001", Deposit: 100_000}
	if f.roomID != "r1" || f.key != idemKey || f.input.GuestName != wantIn.GuestName || f.input.Deposit != wantIn.Deposit ||
		f.input.IDNumber == nil || *f.input.IDNumber != "ABC123456" || f.caller.TenantID != "tn_a" {
		t.Errorf("use case args: %+v key=%q room=%q caller=%+v", f.input, f.key, f.roomID, f.caller)
	}

	f.detail = sampleStay(true)
	if rec = postStay(h, "r1", idemKey, stayInput); rec.Code != 201 || strings.TrimSpace(rec.Body.String()) != wantStayFull {
		t.Errorf("full: code=%d body=%s", rec.Code, rec.Body)
	}

	// A replay maps exactly like a fresh answer, so the bytes are identical.
	f.detail, f.replayed = sampleStay(true), false
	fresh := postStay(h, "r1", idemKey, stayInput).Body.String()
	f.replayed = true
	if replay := postStay(h, "r1", idemKey, stayInput); replay.Code != 201 || replay.Body.String() != fresh {
		t.Errorf("replay differs: %s vs %s", replay.Body, fresh)
	}
}

func TestCreateStayHTTPRejectsEarly_SG203_AC1(t *testing.T) {
	f := &fakeStays{forbid: t}
	h := stayRouter(f, true)
	for name, key := range map[string]string{"missing": "", "malformed": "not-a-uuid"} {
		rec := postStay(h, "r1", key, stayInput)
		if status, code := problemOf(t, rec); rec.Code != 400 || status != 400 || code != "BAD_REQUEST" {
			t.Errorf("%s key: code=%d problem=%d/%s", name, rec.Code, status, code)
		}
	}
	if rec := postStay(h, "r1", idemKey, "{not json"); rec.Code != 400 {
		t.Errorf("malformed body: %d", rec.Code)
	}
	if rec := do(h, "POST", "/v1/rooms/r1/stays", ""); rec.Code != 401 {
		t.Errorf("no token: %d", rec.Code)
	}
	if f.calls != 0 {
		t.Errorf("use case called %d times", f.calls)
	}
}

func TestStaysFlagOff_SG203_AC1(t *testing.T) {
	f := &fakeStays{forbid: t}
	h := stayRouter(f, false)
	for _, rec := range []*httptest.ResponseRecorder{postStay(h, "r1", idemKey, stayInput), getAuthed(h, "/v1/stays/st1")} {
		if status, code := problemOf(t, rec); rec.Code != 404 || status != 404 || code != "FEATURE_DISABLED" {
			t.Errorf("code=%d problem=%d/%s", rec.Code, status, code)
		}
	}
}

func TestStayProblemMapping_SG203_AC1(t *testing.T) {
	vErr := fmt.Errorf("check-in input: %w", &stay.ValidationError{Errors: []stay.FieldError{
		{Path: "guestName", Code: "REQUIRED"}, {Path: "guestPhone", Code: "TOO_SHORT"}, {Path: "idNumber", Code: "PATTERN"}}})
	cases := []struct {
		err    error
		status int
		code   string
		errors string
	}{
		{room.ErrNotVacant, 409, "ROOM_NOT_VACANT", ""},
		{app.ErrIdempotencyKeyReused, 409, "IDEMPOTENCY_KEY_REUSED", ""},
		{access.ErrRoleForbidden, 403, "ROLE_FORBIDDEN", ""},
		{access.ErrBuildingForbidden, 403, "BUILDING_FORBIDDEN", ""},
		{app.ErrNotFound, 404, "NOT_FOUND", ""},
		{vErr, 422, "VALIDATION_FAILED", `"errors":[{"code":"REQUIRED","field":"guestName"},{"code":"TOO_SHORT","field":"guestPhone"},{"code":"PATTERN","field":"idNumber"}]`},
		{fmt.Errorf("check-in input: %w", pricing.ErrUnknownRentalType), 422, "VALIDATION_FAILED", `"errors":[{"code":"INVALID","field":"rentalType"}]`},
		{app.ErrInvalidIdempotencyKey, 422, "VALIDATION_FAILED", `"errors":[{"code":"INVALID","field":"Idempotency-Key"}]`},
		{errors.New("boom MARKER-LEAK"), 500, "INTERNAL", ""},
	}
	for _, c := range cases {
		h := stayRouter(&fakeStays{err: fmt.Errorf("wrapped MARKER-LEAK: %w", c.err)}, true)
		for _, rec := range []*httptest.ResponseRecorder{postStay(h, "r1", idemKey, stayInput), getAuthed(h, "/v1/stays/st1")} {
			status, code := problemOf(t, rec)
			if rec.Code != c.status || status != c.status || code != c.code {
				t.Errorf("%v: code=%d problem=%d/%s", c.err, rec.Code, status, code)
			}
			if strings.Contains(rec.Body.String(), "MARKER-LEAK") || strings.Contains(rec.Body.String(), "wrapped") {
				t.Errorf("%v leaked error text: %s", c.err, rec.Body)
			}
			if has := strings.Contains(rec.Body.String(), `"errors"`); has != (c.errors != "") || (c.errors != "" && !strings.Contains(rec.Body.String(), c.errors)) {
				t.Errorf("%v: errors[] = %s", c.err, rec.Body)
			}
			if rec.Header().Get("Content-Type") != "application/problem+json" {
				t.Errorf("content-type %q", rec.Header().Get("Content-Type"))
			}
		}
	}
}

func TestGetStayHTTP_SG203_AC6(t *testing.T) {
	f := &fakeStays{detail: sampleStay(true)}
	h := stayRouter(f, true)
	rec := getAuthed(h, "/v1/stays/st1")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != wantStayFull || f.gotStay != "st1" {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body)
	}
	f.detail = sampleStay(false)
	if rec = getAuthed(h, "/v1/stays/st1"); rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != wantStayMinimal {
		t.Errorf("minimal: %s", rec.Body)
	}
	if rec = do(h, "GET", "/v1/stays/st1", ""); rec.Code != 401 {
		t.Errorf("no token: %d", rec.Code)
	}

	// A foreign tenant's id and an unknown id are the same use case error: byte-identical bodies.
	f.err = app.ErrNotFound
	a, b := getAuthed(h, "/v1/stays/foreign"), getAuthed(h, "/v1/stays/random")
	if a.Code != 404 || a.Body.String() != b.Body.String() {
		t.Errorf("404 bodies differ: %q vs %q", a.Body, b.Body)
	}
}
