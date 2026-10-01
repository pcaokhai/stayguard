package httpadapter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

const (
	goodToken    = "good-token-value"
	expiredToken = "expired-token-value"
	fakeUserName = "receptionist-jane"
)

// fakeSessions answers Authenticate by token and records every call. When forbid is set, any call
// fails that test immediately, so a path that must not reach the use case is caught.
type fakeSessions struct {
	calls   int
	forbid  *testing.T
	demoErr error
	locale  string
}

func (f *fakeSessions) Authenticate(_ context.Context, token string) (app.Caller, error) {
	f.touch()
	switch token {
	case goodToken:
		return app.Caller{TenantID: "tn_a", UserID: "us_1", Role: access.RoleReceptionist, Locale: "vi"}, nil
	case expiredToken:
		return app.Caller{}, app.ErrSessionExpired
	}
	return app.Caller{}, app.ErrUnauthenticated
}

func (f *fakeSessions) CreateDemo(_ context.Context, role access.Role, locale, _ string) (app.DemoSession, error) {
	f.touch()
	if f.demoErr != nil {
		return app.DemoSession{}, f.demoErr
	}
	u := app.User{ID: "us_1", Name: fakeUserName, Role: role, Locale: locale}
	return app.DemoSession{Token: goodToken, ExpiresAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), TenantID: "tn_a", User: u}, nil
}

func (f *fakeSessions) Me(_ context.Context, c app.Caller) (app.MeView, error) {
	f.touch()
	return app.MeView{
		User:           app.User{ID: c.UserID, Name: fakeUserName, Role: c.Role, Locale: c.Locale},
		Tenant:         app.TenantInfo{ID: c.TenantID, Name: "T", Timezone: "Asia/Ho_Chi_Minh", Currency: "VND"},
		BuildingAccess: map[string]access.Level{"b2": access.NONE, "b1": access.EDIT},
	}, nil
}

func (f *fakeSessions) SetLocale(_ context.Context, _ app.Caller, locale string) error {
	f.touch()
	f.locale = locale
	return nil
}

var errBoom = errors.New("boom")

func (f *fakeSessions) touch() {
	f.calls++
	if f.forbid != nil {
		f.forbid.Errorf("session use case called although the request must be rejected earlier")
	}
}
