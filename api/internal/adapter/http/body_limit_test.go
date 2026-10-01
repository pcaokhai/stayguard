package httpadapter

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

func bodyOf(n int) string {
	pad := strings.Repeat("a", n-len(`{"guestName":""}`))
	return `{"guestName":"` + pad + `"}`
}

// sendBody posts a body with a valid token; chunked drops the Content-Length so only the reader cap can stop it.
func sendBody(h http.Handler, method, path, body string, chunked bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+goodToken)
	req.Header.Set("Idempotency-Key", retryID)
	if chunked {
		req.ContentLength = -1
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRequestBodyCap_SG203_AC1(t *testing.T) {
	routes := []struct{ method, path string }{
		{"POST", "/v1/rooms/r1/stays"}, {"POST", "/v1/demo/sessions"}, {"PUT", "/v1/me/locale"},
	}
	for _, enabled := range []bool{true, false} {
		f := &fakeStays{forbid: t}
		h := stayRouter(f, enabled)
		for _, r := range routes {
			for _, chunked := range []bool{false, true} {
				rec := sendBody(h, r.method, r.path, bodyOf(maxRequestBodyBytes+1), chunked)
				status, code := problemOf(t, rec)
				if rec.Code != 413 || status != 413 || code != "PAYLOAD_TOO_LARGE" || strings.Contains(rec.Body.String(), "aaaa") ||
					rec.Header().Get("Content-Type") != "application/problem+json" {
					t.Errorf("%s %s chunked=%v flag=%v: %d %s", r.method, r.path, chunked, enabled, rec.Code, rec.Body)
				}
			}
		}
		if f.calls != 0 {
			t.Errorf("service called %d times", f.calls)
		}
	}
}

func TestRequestBodyJustUnderCap_SG203_AC1(t *testing.T) {
	f := &fakeStays{detail: sampleStay(false)}
	rec := sendBody(stayRouter(f, true), "POST", "/v1/rooms/r1/stays", bodyOf(maxRequestBodyBytes), true)
	if rec.Code != 201 || f.calls != 1 {
		t.Errorf("code=%d calls=%d", rec.Code, f.calls)
	}
}

// With the flag off the generated binding still runs first: a bad key or body is a 400, and no
// use case or database I/O happens either way.
func TestStaysFlagOffBindingFirst_SG203_AC1(t *testing.T) {
	f := &fakeStays{forbid: t}
	h := stayRouter(f, false)
	if rec := postStay(h, "r1", "", stayInput); rec.Code != 400 {
		t.Errorf("missing key: %d", rec.Code)
	}
	if rec := postStay(h, "r1", retryID, "{not json"); rec.Code != 400 {
		t.Errorf("malformed body: %d", rec.Code)
	}
}

// JSON null decodes to a zero-value body, so the use case's domain validation answers 422 with
// REQUIRED errors; the handler has no separate nil-body branch.
func TestCreateStayNullBody_SG203_AC1(t *testing.T) {
	f := &fakeStays{err: fmt.Errorf("check-in input: %w", &stay.ValidationError{Errors: []stay.FieldError{
		{Path: "guestName", Code: "REQUIRED"}, {Path: "guestPhone", Code: "REQUIRED"}}})}
	rec := postStay(stayRouter(f, true), "r1", retryID, "null")
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), `{"code":"REQUIRED","field":"guestName"}`) || f.input.GuestName != "" || f.calls != 1 {
		t.Errorf("code=%d calls=%d input=%+v body=%s", rec.Code, f.calls, f.input, rec.Body)
	}
}
