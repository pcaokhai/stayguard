package httpadapter

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/app"
)

func post(h *fakeSessions, demo bool, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/demo/sessions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	newRouterWith(&bytes.Buffer{}, h, demo).ServeHTTP(rec, req)
	return rec
}

func TestDemoDisabled_SG102_AC1(t *testing.T) {
	f := &fakeSessions{forbid: t}
	rec := post(f, false, `{"role":"OWNER","locale":"vi"}`)
	status, code := problemOf(t, rec)
	if rec.Code != 404 || status != 404 || code != "DEMO_DISABLED" {
		t.Fatalf("code=%d problem=%d/%s", rec.Code, status, code)
	}
	if f.calls != 0 {
		t.Fatalf("use case reached %d times with demo mode off", f.calls)
	}
}

func TestCreateDemoHandler_SG102_AC1(t *testing.T) {
	f := &fakeSessions{}
	rec := post(f, true, `{"role":"RECEPTIONIST","locale":"en"}`)
	var s struct {
		AccessToken string `json:"accessToken"`
		TenantID    string `json:"tenantId"`
		User        struct{ Role, Locale string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil || rec.Code != 201 {
		t.Fatalf("code=%d body=%q err=%v", rec.Code, rec.Body.String(), err)
	}
	if s.AccessToken != goodToken || s.TenantID != "tn_a" || s.User.Role != "RECEPTIONIST" || s.User.Locale != "en" {
		t.Fatalf("unexpected session: %+v", s)
	}
}

func TestMeAndLocaleHandlers_SG102_AC3(t *testing.T) {
	f := &fakeSessions{}
	h := newRouterWith(&bytes.Buffer{}, f, false)
	rec := do(h, "GET", "/v1/me", "Bearer "+goodToken)
	var me struct {
		BuildingAccess []struct{ BuildingId, Level string }
		Tenant         struct{ Id, Currency string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &me); err != nil || rec.Code != 200 {
		t.Fatalf("code=%d body=%q", rec.Code, rec.Body.String())
	}
	if len(me.BuildingAccess) != 2 || me.BuildingAccess[0].BuildingId != "b1" || me.BuildingAccess[0].Level != "EDIT" ||
		me.BuildingAccess[1].Level != "NONE" || me.Tenant.Id != "tn_a" || me.Tenant.Currency != "VND" {
		t.Fatalf("unexpected me: %s", rec.Body.String())
	}
	req := httptest.NewRequest("PUT", "/v1/me/locale", strings.NewReader(`{"locale":"en"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+goodToken)
	put := httptest.NewRecorder()
	h.ServeHTTP(put, req)
	if put.Code != 204 || f.locale != "en" {
		t.Fatalf("code=%d locale=%q", put.Code, f.locale)
	}
}

func TestProblemMapping_SG102_AC1(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{app.ErrDemoDisabled, 404, "DEMO_DISABLED"},
		{app.ErrTrialNotFound, 404, "TRIAL_NOT_FOUND"},
		{app.ErrUnauthenticated, 401, "UNAUTHENTICATED"},
		{app.ErrSessionExpired, 401, "SESSION_EXPIRED"},
		{&app.ValidationError{Field: "locale", Reason: "client-input-xyz"}, 422, "VALIDATION_FAILED"},
		{app.ErrConflict, 409, "CONFLICT"},
		{errBoom, 500, "INTERNAL"},
	}
	for _, tc := range cases {
		var logs bytes.Buffer
		rec := httptest.NewRecorder()
		wrapped := errors.Join(errors.New("ctx"), tc.err) // mapping must use errors.Is/As
		problemResponder(slog.New(slog.NewJSONHandler(&logs, nil)))(rec, httptest.NewRequest("GET", "/", nil), wrapped)
		status, code := problemOf(t, rec)
		if rec.Code != tc.status || status != tc.status || code != tc.code {
			t.Errorf("%v: code=%d problem=%d/%s", tc.err, rec.Code, status, code)
		}
		if strings.Contains(rec.Body.String(), "client-input-xyz") {
			t.Errorf("client input echoed: %s", rec.Body.String())
		}
		if (tc.status == 401) != (rec.Header().Get("WWW-Authenticate") == "Bearer") {
			t.Errorf("%v: WWW-Authenticate=%q", tc.err, rec.Header().Get("WWW-Authenticate"))
		}
	}
}

func TestCreateDemoValidation_SG102_AC1(t *testing.T) {
	// The fake accepts anything; the real use case rejects. Use the real error mapping through the router.
	for name, tc := range map[string]struct {
		body   string
		err    error
		status int
		code   string
	}{
		"bad role":      {`{"role":"ADMIN-secret","locale":"vi"}`, &app.ValidationError{Field: "role", Reason: "unknown role"}, 422, "VALIDATION_FAILED"},
		"bad locale":    {`{"role":"OWNER","locale":"fr-secret"}`, &app.ValidationError{Field: "locale", Reason: "must be vi or en"}, 422, "VALIDATION_FAILED"},
		"malformed":     {`{not json`, nil, 400, "BAD_REQUEST"},
		"unknown trial": {`{"role":"OWNER","locale":"vi","tenantId":"tn_x"}`, app.ErrTrialNotFound, 404, "TRIAL_NOT_FOUND"},
	} {
		rec := post(&fakeSessions{demoErr: tc.err}, true, tc.body)
		status, code := problemOf(t, rec)
		if rec.Code != tc.status || status != tc.status || code != tc.code {
			t.Errorf("%s: code=%d problem=%d/%s", name, rec.Code, status, code)
		}
		if strings.Contains(rec.Body.String(), "secret") {
			t.Errorf("%s: client input echoed: %s", name, rec.Body.String())
		}
	}
}
