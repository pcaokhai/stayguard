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
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

const (
	extrasPath   = "/v1/stays/st1/extras"
	checkoutPath = "/v1/stays/st1/checkout"
	wantServices = `{"items":[{"code":"BEER","name":{"en":"Beer","vi":"Bia"},"price":25000,"stock":5},{"code":"WATER","name":{"en":"Water","vi":"Nuoc"},"price":10000,"stock":0}]}`
	wantInvoice  = `{"billCode":"PH1001A101","createdAt":"2026-10-01T05:00:00Z","id":"iv1","quote":{"asOf":"2026-10-01T05:00:00Z","balanceDue":0,"capped":false,"depositPaid":300000,"extrasAmount":20000,"lines":[{"amount":250000,"code":"OVERNIGHT","quantity":1,"unitAmount":250000}],"refundDue":30000,"stayAmount":250000,"total":270000},"roomCode":"A101","status":"OPEN","stayId":"st1"}`
)

func sampleServices() []app.Service {
	return []app.Service{
		{Code: "BEER", Name: app.LocalizedName{VI: "Bia", EN: "Beer"}, Price: 25_000, Stock: 5},
		{Code: "WATER", Name: app.LocalizedName{VI: "Nuoc", EN: "Water"}, Price: 10_000},
	}
}

func TestBillingHTTP_SG205_AC1(t *testing.T) {
	f := &fakeBilling{services: sampleServices(), detail: sampleStay(true), invoice: sampleInvoice()}
	h := billingRouter(f, true)

	if rec := getAuthed(h, "/v1/services"); rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != wantServices || f.caller.TenantID != "tn_a" {
		t.Errorf("list: code=%d body=%s", rec.Code, rec.Body)
	}
	// addStayExtras answers with the contract Stay through the same mapper as getStay.
	rec := postBilling(h, extrasPath, retryID, extrasInput)
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != wantStayFull {
		t.Fatalf("extras: code=%d body=%s", rec.Code, rec.Body)
	}
	want := []stay.ExtraInput{{ServiceCode: "WATER", Quantity: 2}, {ServiceCode: "BEER", Quantity: 1}}
	if f.stayID != "st1" || f.key != retryID || fmt.Sprint(f.items) != fmt.Sprint(want) {
		t.Errorf("extras args: stay=%q key=%q items=%v", f.stayID, f.key, f.items)
	}
	rec = postBilling(h, checkoutPath, retryID, "")
	if rec.Code != 201 || strings.TrimSpace(rec.Body.String()) != wantInvoice || f.stayID != "st1" || f.key != retryID {
		t.Errorf("checkout: code=%d body=%s", rec.Code, rec.Body)
	}
	// A replay maps exactly like a fresh answer, so the bytes are identical.
	fresh := postBilling(h, checkoutPath, retryID, "").Body.String()
	f.replayed = true
	if replay := postBilling(h, checkoutPath, retryID, ""); replay.Code != 201 || replay.Body.String() != fresh {
		t.Errorf("checkout replay differs: %s vs %s", replay.Body, fresh)
	}
	f.replayed = false
	fresh = postBilling(h, extrasPath, retryID, extrasInput).Body.String()
	f.replayed = true
	if replay := postBilling(h, extrasPath, retryID, extrasInput); replay.Body.String() != fresh {
		t.Errorf("extras replay differs: %s vs %s", replay.Body, fresh)
	}
}

func TestBillingHTTPRejectsEarly_SG205_AC1(t *testing.T) {
	f := &fakeBilling{forbid: t}
	h := billingRouter(f, true)
	for name, key := range map[string]string{"missing": "", "malformed": "not-a-uuid"} {
		for _, p := range []string{extrasPath, checkoutPath} {
			rec := postBilling(h, p, key, extrasInput)
			if status, code := problemOf(t, rec); rec.Code != 400 || status != 400 || code != "BAD_REQUEST" {
				t.Errorf("%s key on %s: code=%d problem=%d/%s", name, p, rec.Code, status, code)
			}
		}
	}
	if rec := postBilling(h, extrasPath, retryID, "{not json"); rec.Code != 400 {
		t.Errorf("malformed body: %d", rec.Code)
	}
	for _, r := range [][2]string{{"GET", "/v1/services"}, {"POST", extrasPath}, {"POST", checkoutPath}} {
		if rec := do(h, r[0], r[1], ""); rec.Code != 401 {
			t.Errorf("%s %s without a token: %d", r[0], r[1], rec.Code)
		}
	}
	for _, p := range []string{extrasPath, checkoutPath} { // oversized bodies stop at the reader cap, chunked or not
		for _, chunked := range []bool{false, true} {
			if chunked && p == checkoutPath {
				continue // check-out has no body: a chunked one is never read, only the declared length is capped
			}
			rec := sendBody(h, "POST", p, bodyOf(maxRequestBodyBytes+1), chunked)
			if status, code := problemOf(t, rec); rec.Code != 413 || status != 413 || code != "PAYLOAD_TOO_LARGE" || strings.Contains(rec.Body.String(), "aaaa") {
				t.Errorf("%s chunked=%v: %d %s", p, chunked, rec.Code, rec.Body)
			}
		}
	}
	if f.calls != 0 {
		t.Errorf("use case called %d times", f.calls)
	}
}

func TestBillingFlagOff_SG205_AC3(t *testing.T) {
	f := &fakeBilling{forbid: t}
	h := billingRouter(f, false)
	recs := []*httptest.ResponseRecorder{getAuthed(h, "/v1/services"), postBilling(h, extrasPath, retryID, extrasInput), postBilling(h, checkoutPath, retryID, "")}
	for _, rec := range recs {
		if status, code := problemOf(t, rec); rec.Code != 404 || status != 404 || code != "FEATURE_DISABLED" {
			t.Errorf("code=%d problem=%d/%s", rec.Code, status, code)
		}
	}
	// The binding still runs first: a missing key is a 400 before the flag is looked at.
	if rec := postBilling(h, extrasPath, "", extrasInput); rec.Code != 400 {
		t.Errorf("missing key with the flag off: %d", rec.Code)
	}
}

func TestBillingProblemMapping_SG205_AC1(t *testing.T) {
	vErr := fmt.Errorf("extras input: %w", stay.NewValidationError([]stay.FieldError{
		{Path: "items[0].serviceCode", Code: "UNKNOWN"}, {Path: "items[1].quantity", Code: "OUT_OF_RANGE"}}))
	cases := []struct {
		err    error
		status int
		code   string
		errors string
	}{
		{stay.ErrInsufficientStock, 409, "INSUFFICIENT_STOCK", ""},
		{stay.ErrNotActive, 409, "STAY_NOT_ACTIVE", ""},
		{app.ErrIdempotencyKeyReused, 409, "IDEMPOTENCY_KEY_REUSED", ""},
		{access.ErrRoleForbidden, 403, "ROLE_FORBIDDEN", ""},
		{access.ErrBuildingForbidden, 403, "BUILDING_FORBIDDEN", ""},
		{app.ErrNotFound, 404, "NOT_FOUND", ""},
		{vErr, 422, "VALIDATION_FAILED", `"errors":[{"code":"UNKNOWN","field":"items[0].serviceCode"},{"code":"OUT_OF_RANGE","field":"items[1].quantity"}]`},
		{app.ErrInvalidIdempotencyKey, 422, "VALIDATION_FAILED", `"errors":[{"code":"INVALID","field":"Idempotency-Key"}]`},
		{errors.New("boom MARKER-LEAK"), 500, "INTERNAL", ""},
	}
	for _, c := range cases {
		h := billingRouter(&fakeBilling{err: fmt.Errorf("wrapped MARKER-LEAK: %w", c.err)}, true)
		recs := []*httptest.ResponseRecorder{getAuthed(h, "/v1/services"), postBilling(h, extrasPath, retryID, extrasInput), postBilling(h, checkoutPath, retryID, "")}
		for _, rec := range recs {
			status, code := problemOf(t, rec)
			if rec.Code != c.status || status != c.status || code != c.code {
				t.Errorf("%v: code=%d problem=%d/%s", c.err, rec.Code, status, code)
			}
			if strings.Contains(rec.Body.String(), "MARKER-LEAK") || strings.Contains(rec.Body.String(), "wrapped") || strings.Contains(rec.Body.String(), "WATER") {
				t.Errorf("%v leaked text: %s", c.err, rec.Body)
			}
			if has := strings.Contains(rec.Body.String(), `"errors"`); has != (c.errors != "") || (c.errors != "" && !strings.Contains(rec.Body.String(), c.errors)) {
				t.Errorf("%v: errors[] = %s", c.err, rec.Body)
			}
		}
	}
}

// A foreign tenant's id and an unknown id are the same use case error: byte-identical bodies.
func TestBillingNotFoundIdentical_SG205_AC2(t *testing.T) {
	h := billingRouter(&fakeBilling{err: app.ErrNotFound}, true)
	for _, suffix := range []string{"extras", "checkout"} {
		a := postBilling(h, "/v1/stays/foreign/"+suffix, retryID, extrasInput)
		b := postBilling(h, "/v1/stays/random/"+suffix, retryID, extrasInput)
		if a.Code != http.StatusNotFound || a.Body.String() != b.Body.String() {
			t.Errorf("%s: 404 bodies differ: %q vs %q", suffix, a.Body, b.Body)
		}
	}
}
