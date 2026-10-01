package httpadapter

import (
	"context"
	"errors"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

const (
	goodToken    = "good-token-value"
	expiredToken = "expired-token-value"
	fakeUserName = "receptionist-jane"
)

// fakeSessions answers Authenticate by token and records every call. With failAll set, any use is a
// test failure signal (calls > 0) so a handler that should not reach the use case is caught.
type fakeSessions struct {
	calls   int
	failAll bool
	demoErr error
	locale  string
}

func (f *fakeSessions) Authenticate(_ context.Context, token string) (app.Caller, error) {
	f.calls++
	switch token {
	case goodToken:
		return app.Caller{TenantID: "tn_a", UserID: "us_1", Role: access.RoleReceptionist, Locale: "vi"}, nil
	case expiredToken:
		return app.Caller{}, app.ErrSessionExpired
	}
	return app.Caller{}, app.ErrUnauthenticated
}

func (f *fakeSessions) CreateDemo(_ context.Context, role access.Role, locale, _ string) (app.DemoSession, error) {
	f.calls++
	if f.demoErr != nil {
		return app.DemoSession{}, f.demoErr
	}
	u := app.User{ID: "us_1", Name: fakeUserName, Role: role, Locale: locale}
	return app.DemoSession{Token: goodToken, ExpiresAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), TenantID: "tn_a", User: u}, nil
}

func (f *fakeSessions) Me(_ context.Context, c app.Caller) (app.MeView, error) {
	f.calls++
	return app.MeView{
		User:           app.User{ID: c.UserID, Name: fakeUserName, Role: c.Role, Locale: c.Locale},
		Tenant:         app.TenantInfo{ID: c.TenantID, Name: "T", Timezone: "Asia/Ho_Chi_Minh", Currency: "VND"},
		BuildingAccess: map[string]access.Level{"b2": access.NONE, "b1": access.EDIT},
	}, nil
}

func (f *fakeSessions) SetLocale(_ context.Context, _ app.Caller, locale string) error {
	f.calls++
	f.locale = locale
	return nil
}

var errBoom = errors.New("boom")
