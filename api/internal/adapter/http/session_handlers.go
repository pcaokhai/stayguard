package httpadapter

import (
	"context"
	"sort"

	"github.com/pcaokhai/stayguard/api/internal/adapter/http/gen"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

// sessionService is what the session handlers need from the application layer.
type sessionService interface {
	CreateDemo(ctx context.Context, role access.Role, locale, tenantID string) (app.DemoSession, error)
	Me(ctx context.Context, c app.Caller) (app.MeView, error)
	SetLocale(ctx context.Context, c app.Caller, locale string) error
}

func (s Server) CreateDemoSession(ctx context.Context, req gen.CreateDemoSessionRequestObject) (gen.CreateDemoSessionResponseObject, error) {
	// Checked here too so a disabled demo never reaches the use case or the database.
	if !s.demoEnabled {
		return nil, app.ErrDemoDisabled
	}
	if req.Body == nil {
		return nil, &app.ValidationError{Field: "body", Reason: "required"}
	}
	tenantID := ""
	if req.Body.TenantId != nil {
		tenantID = *req.Body.TenantId
	}
	d, err := s.sessions.CreateDemo(ctx, access.Role(req.Body.Role), string(req.Body.Locale), tenantID)
	if err != nil {
		return nil, err
	}
	return demoWithCookie{gen.CreateDemoSession201JSONResponse(gen.Session{
		AccessToken: d.Token, ExpiresAt: d.ExpiresAt, TenantId: d.TenantID, User: toUser(d.User),
	}), sessionCookieHeader(ctx, d.Token)}, nil
}

func (s Server) GetMe(ctx context.Context, _ gen.GetMeRequestObject) (gen.GetMeResponseObject, error) {
	c, ok := app.CallerFrom(ctx)
	if !ok {
		return nil, app.ErrUnauthenticated
	}
	v, err := s.sessions.Me(ctx, c)
	if err != nil {
		return nil, err
	}
	me := gen.Me{User: toUser(v.User), BuildingAccess: toBuildingAccess(v.BuildingAccess)}
	me.Tenant.Id, me.Tenant.Name, me.Tenant.Timezone = v.Tenant.ID, v.Tenant.Name, v.Tenant.Timezone
	me.Tenant.Currency = gen.MeTenantCurrency(v.Tenant.Currency)
	return gen.GetMe200JSONResponse(me), nil
}

func (s Server) SetMyLocale(ctx context.Context, req gen.SetMyLocaleRequestObject) (gen.SetMyLocaleResponseObject, error) {
	c, ok := app.CallerFrom(ctx)
	if !ok {
		return nil, app.ErrUnauthenticated
	}
	if req.Body == nil {
		return nil, &app.ValidationError{Field: "body", Reason: "required"}
	}
	if err := s.sessions.SetLocale(ctx, c, string(req.Body.Locale)); err != nil {
		return nil, err
	}
	return gen.SetMyLocale204Response{}, nil
}

func toUser(u app.User) gen.User {
	return gen.User{Id: u.ID, Name: u.Name, Role: gen.Role(u.Role), Locale: gen.Locale(u.Locale)}
}

// toBuildingAccess sorts by building id so the response is stable.
func toBuildingAccess(m map[string]access.Level) []gen.BuildingAccess {
	out := make([]gen.BuildingAccess, 0, len(m))
	for id, l := range m {
		out = append(out, gen.BuildingAccess{BuildingId: id, Level: permissionLevel(l)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].BuildingId < out[j].BuildingId })
	return out
}

func permissionLevel(l access.Level) gen.PermissionLevel {
	switch l {
	case access.EDIT:
		return gen.PermissionLevelEDIT
	case access.VIEW:
		return gen.PermissionLevelVIEW
	}
	return gen.PermissionLevelNONE // unknown levels fail closed
}
