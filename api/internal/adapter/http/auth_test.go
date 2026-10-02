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

		{"token with space", "Bearer a b", "UNAUTHENTICATED"},
		{"two spaces", "Bearer  " + goodToken, "UNAUTHENTICATED"},
		{"scheme alone", "Bearer", "UNAUTHENTICATED"},
		{"empty token", "Bearer ", "UNAUTHENTICATED"},
		{"basic", "Basic " + goodToken, "UNAUTHENTICATED"},
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
	// Only the exact method and path are public: everything else about the same route is 401.
	for _, c := range [][2]string{{"GET", "/v1/demo/sessions"}, {"PUT", "/v1/demo/sessions"}, {"POST", "/v1/demo/sessions/"},
		{"GET", "/v1/webhooks/bank"}, {"POST", "/v1/webhooks/bank/"}} {
		if rec := do(h, c[0], c[1], ""); rec.Code != 401 {
			t.Errorf("%s %s: code=%d, want 401", c[0], c[1], rec.Code)
		}
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

func TestAuthMiddlewareMethodsAndBarePrefix_SG102_AC4(t *testing.T) {
	h := newTestRouter(&bytes.Buffer{})
	for _, c := range [][2]string{{"HEAD", "/v1/me"}, {"OPTIONS", "/v1/me"}, {"GET", "/v1"}, {"DELETE", "/v1"}} {
		if rec := do(h, c[0], c[1], ""); rec.Code != 401 {
			t.Errorf("%s %s: code=%d, want 401", c[0], c[1], rec.Code)
		}
	}
}

// Paths that merely look like /v1 never reach an API handler: they are not under the API prefix
// (static 404) or, once decoded, are the protected path itself (401).
func TestAuthMiddlewareLookalikePaths_SG102_AC4(t *testing.T) {
	f := &fakeSessions{forbid: t}
	h := newRouterWith(&bytes.Buffer{}, f, true)
	for _, p := range []string{"//v1/me", "/V1/me", "/v1x/me", "/v1%2Fme", "/v1/../v1/me", "/./v1/me"} {
		rec := do(h, "GET", p, "")
		if rec.Code/100 == 2 || rec.Code == 501 {
			t.Errorf("%s reached a handler: code=%d", p, rec.Code)
		}
	}
}

func TestAuthMiddlewareDuplicateHeader_SG102_AC4(t *testing.T) {
	f := &fakeSessions{forbid: t}
	h := newRouterWith(&bytes.Buffer{}, f, true)
	req := httptest.NewRequest("GET", "/v1/me", nil)
	req.Header.Add("Authorization", "Bearer "+goodToken)
	req.Header.Add("Authorization", "Bearer "+goodToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if _, code := problemOf(t, rec); rec.Code != 401 || code != "UNAUTHENTICATED" {
		t.Fatalf("code=%d problem=%s", rec.Code, code)
	}
}

// The scheme is matched case-insensitively (RFC 9110 section 11.1); this is deliberate.
func TestAuthMiddlewareSchemeCase_SG102_AC4(t *testing.T) {
	h := newTestRouter(&bytes.Buffer{})
	for _, scheme := range []string{"Bearer", "bearer", "BEARER"} {
		if rec := do(h, "GET", "/v1/me", scheme+" "+goodToken); rec.Code != 200 {
			t.Errorf("%s: code=%d, want 200 (authenticated)", scheme, rec.Code)
		}
	}
}
