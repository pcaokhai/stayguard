package httpadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

const oneTimeToken = "one-time-pin-token"

// pinChangeSessions authenticates the one-time-PIN session on top of the usual fake.
type pinChangeSessions struct{ *fakeSessions }

func (p pinChangeSessions) Authenticate(ctx context.Context, token string) (app.Caller, error) {
	if token == oneTimeToken {
		return app.Caller{TenantID: "tn_a", UserID: "us_1", Role: access.RoleReceptionist, PinChangeRequired: true, SessionHash: "h"}, nil
	}
	return p.fakeSessions.Authenticate(ctx, token)
}

type fakeAuth struct {
	signErr, changeErr error
	ip, pin            string
	signedOut          bool
}

func (f *fakeAuth) SignIn(_ context.Context, ip, _, _, pin string) (app.SignInResult, error) {
	f.ip, f.pin = ip, pin
	if f.signErr != nil {
		return app.SignInResult{}, f.signErr
	}
	u := app.User{ID: "us_1", Name: "ann", Role: access.RoleReceptionist, Locale: "vi"}
	return app.SignInResult{DemoSession: app.DemoSession{Token: "tok", ExpiresAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), TenantID: "tn_a", User: u}, MustChangePin: true}, nil
}

func (f *fakeAuth) SignOut(context.Context, app.Caller) error { f.signedOut = true; return nil }

func (f *fakeAuth) ChangePin(context.Context, app.Caller, string, string) error { return f.changeErr }

func authRouter(a *fakeAuth, logs *bytes.Buffer, trustProxy bool) http.Handler {
	return NewRouter(slog.New(slog.NewJSONHandler(logs, nil)), Options{
		Probe: readyProbe{}, Sessions: pinChangeSessions{&fakeSessions{}}, Auth: a, TrustProxy: trustProxy,
	})
}

func send(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

const signInBody = `{"guesthouseCode":"casa","username":"ann","pin":"482915"}`

func TestSignInHTTP_PublicAndMapsErrors_SG701(t *testing.T) {
	logs := &bytes.Buffer{}
	a := &fakeAuth{}
	h := authRouter(a, logs, false)
	rec := send(h, "POST", "/v1/auth/sign-in", "", signInBody) // no token needed
	var got map[string]any
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &got) != nil || got["accessToken"] != "tok" || got["mustChangePin"] != true {
		t.Fatalf("sign-in: %d %s", rec.Code, rec.Body)
	}
	if a.pin != "482915" {
		t.Fatalf("PIN not passed: %q", a.pin)
	}
	until := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for name, c := range map[string]struct {
		err    error
		status int
		code   string
	}{
		"invalid":       {app.ErrPinInvalid, 401, "PIN_INVALID"},
		"locked":        {&app.AccountLockedError{Until: until}, 401, "ACCOUNT_LOCKED"},
		"limited":       {app.ErrTooManyRequests, 429, "RATE_LIMITED"},
		"invalid input": {&app.ValidationError{Field: "pin", Reason: "x"}, 422, "VALIDATION_FAILED"},
	} {
		a.signErr = c.err
		rec = send(h, "POST", "/v1/auth/sign-in", "", signInBody)
		if status, code := problemOf(t, rec); rec.Code != c.status || status != c.status || code != c.code {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
	a.signErr = &app.AccountLockedError{Until: until}
	if rec = send(h, "POST", "/v1/auth/sign-in", "", signInBody); !strings.Contains(rec.Body.String(), `"lockedUntil":"2026-01-02T03:04:05Z"`) {
		t.Errorf("lockedUntil missing: %s", rec.Body)
	}
	if rec = send(h, "POST", "/v1/auth/sign-in", "", `{"guesthouseCode":"casa","username":"ann"}`); rec.Code != 422 {
		t.Errorf("missing pin: %d", rec.Code)
	}
	if strings.Contains(logs.String(), "482915") {
		t.Fatal("PIN reached the logs")
	}
}

func TestSignInHTTP_ClientIP_SG701_AC5(t *testing.T) {
	a := &fakeAuth{}
	for _, trust := range []bool{false, true} {
		req := httptest.NewRequest("POST", "/v1/auth/sign-in", strings.NewReader(signInBody))
		req.RemoteAddr = "10.0.0.9:5555"
		req.Header.Set("X-Forwarded-For", "6.6.6.6, 203.0.113.7")
		authRouter(a, &bytes.Buffer{}, trust).ServeHTTP(httptest.NewRecorder(), req)
		want := map[bool]string{false: "10.0.0.9", true: "203.0.113.7"}[trust]
		if a.ip != want {
			t.Errorf("trust=%v ip=%q want %q", trust, a.ip, want)
		}
	}
}

func TestOneTimePinSessionOnlyChangesPin_SG701_AC3(t *testing.T) {
	a := &fakeAuth{}
	h := authRouter(a, &bytes.Buffer{}, false)
	rec := send(h, "GET", "/v1/buildings", oneTimeToken, "")
	if status, code := problemOf(t, rec); rec.Code != 403 || status != 403 || code != "PIN_CHANGE_REQUIRED" {
		t.Fatalf("other route: %d %s", rec.Code, rec.Body)
	}
	if rec = send(h, "GET", "/v1/me", oneTimeToken, ""); rec.Code != 200 {
		t.Errorf("getMe: %d", rec.Code)
	}
	if rec = send(h, "PUT", "/v1/me/pin", oneTimeToken, `{"currentPin":"482915","newPin":"736201"}`); rec.Code != 204 {
		t.Errorf("changeMyPin: %d %s", rec.Code, rec.Body)
	}
	a.changeErr = app.ErrPinTooSimple
	if rec = send(h, "PUT", "/v1/me/pin", oneTimeToken, `{"currentPin":"482915","newPin":"123456"}`); rec.Code != 422 {
		t.Errorf("too simple: %d", rec.Code)
	} else if _, code := problemOf(t, rec); code != "PIN_TOO_SIMPLE" {
		t.Errorf("code %s", code)
	}
	if rec = send(h, "POST", "/v1/auth/sign-out", oneTimeToken, ""); rec.Code != 204 || !a.signedOut {
		t.Errorf("signOut: %d", rec.Code)
	}
	// A normal session is not gated.
	if rec = send(h, "GET", "/v1/me", goodToken, ""); rec.Code != 200 {
		t.Errorf("normal session: %d", rec.Code)
	}
}
