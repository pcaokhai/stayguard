package httpadapter

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/pcaokhai/stayguard/api/internal/app"
)

func problemOf(t *testing.T, rec *httptest.ResponseRecorder) (int, string) {
	t.Helper()
	var p struct {
		Status int    `json:"status"`
		Code   string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("not a problem body: %q", rec.Body.String())
	}
	return p.Status, p.Code
}

func do(h http.Handler, method, path, auth string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAuthMiddleware_SG102_AC4(t *testing.T) {
	h := newTestRouter(&bytes.Buffer{})
	cases := []struct{ name, auth, code string }{
		{"missing", "", "UNAUTHENTICATED"},
		{"wrong scheme", "Basic abc", "UNAUTHENTICATED"},
		{"bearer without token", "Bearer ", "UNAUTHENTICATED"},
		{"token with space", "Bearer a b", "UNAUTHENTICATED"},
		{"unknown token", "Bearer nope", "UNAUTHENTICATED"},
		{"expired token", "Bearer " + expiredToken, "SESSION_EXPIRED"},
	}
	for _, tc := range cases {
		// /v1/not/registered proves protection is by prefix, not by route table.
		for _, path := range []string{"/v1/me", "/v1/not/registered"} {
			rec := do(h, "GET", path, tc.auth)
			status, code := problemOf(t, rec)
			if rec.Code != 401 || status != 401 || code != tc.code {
				t.Errorf("%s %s: code=%d problem=%d/%s", tc.name, path, rec.Code, status, code)
			}
			if rec.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Errorf("%s %s: missing WWW-Authenticate", tc.name, path)
			}
		}
	}
}

func TestAuthMiddlewarePublicAndOutsideV1_SG102_AC4(t *testing.T) {
	h := newTestRouter(&bytes.Buffer{})
	if rec := do(h, "POST", "/v1/demo/sessions", ""); rec.Code == 401 {
		t.Errorf("demo sessions must be public, got 401")
	}
	if rec := do(h, "POST", "/v1/webhooks/bank", ""); rec.Code == 401 {
		t.Errorf("bank webhook must be public, got 401")
	}
	// Same path, other method, is not public.
	if rec := do(h, "GET", "/v1/demo/sessions", ""); rec.Code != 401 && rec.Code != 405 {
		t.Errorf("GET demo sessions: code=%d", rec.Code)
	}
	if rec := do(h, "GET", "/v1/demo/sessions/", ""); rec.Code != 401 {
		t.Errorf("trailing slash must not be public, got %d", rec.Code)
	}
	for _, p := range []string{"/healthz", "/readyz", "/index.html"} {
		if rec := do(h, "GET", p, ""); rec.Code == 401 {
			t.Errorf("%s must not need a token", p)
		}
	}
}

func TestAuthMiddlewareCaller_SG102_AC4(t *testing.T) {
	var got app.Caller
	var ok bool
	r := chi.NewRouter()
	r.Use(authenticate(slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)), &fakeSessions{}))
	r.Get("/v1/probe", func(_ http.ResponseWriter, req *http.Request) { got, ok = app.CallerFrom(req.Context()) })
	do(r, "GET", "/v1/probe", "bearer "+goodToken) // scheme is case-insensitive
	if !ok || got.TenantID != "tn_a" || got.UserID != "us_1" {
		t.Fatalf("caller not in context: %+v ok=%v", got, ok)
	}
}

func TestAuthMiddlewareIgnoresQueryAndCookie_SG102_AC4(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/me?access_token="+goodToken, nil)
	req.Header.Set("Cookie", "token="+goodToken)
	rec := httptest.NewRecorder()
	newTestRouter(&bytes.Buffer{}).ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("token outside the Authorization header accepted: %d", rec.Code)
	}
}

func TestNoSecretsInLogs_SG102_AC2(t *testing.T) {
	var logs bytes.Buffer
	h := newTestRouter(&logs)
	do(h, "GET", "/v1/me", "Bearer "+goodToken)
	do(h, "GET", "/v1/me", "Bearer "+expiredToken)
	do(h, "GET", "/v1/me", "Bearer wrong-secret")
	for _, secret := range []string{goodToken, app.HashToken(goodToken), expiredToken, "wrong-secret", fakeUserName, "Authorization"} {
		if strings.Contains(logs.String(), secret) {
			t.Errorf("log contains %q: %s", secret, logs.String())
		}
	}
	if logs.Len() == 0 {
		t.Fatal("expected request log lines")
	}
}
