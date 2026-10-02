package httpadapter

import (
	"context"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/pcaokhai/stayguard/api/internal/adapter/http/gen"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/money"
)

// StaffService is the staff management surface the handlers need.
type StaffService interface {
	List(ctx context.Context, c app.Caller, position *string) ([]app.StaffView, error)
	CreateStaff(ctx context.Context, c app.Caller, key string, in app.StaffInput) (app.StaffView, *app.OneTimePin, error)
	UpdateStaff(ctx context.Context, c app.Caller, userID string, in app.StaffUpdate) (app.StaffView, error)
	ResetPin(ctx context.Context, c app.Caller, userID string) (app.OneTimePin, error)
	LockStaff(ctx context.Context, c app.Caller, userID string) (app.StaffView, error)
	UnlockStaff(ctx context.Context, c app.Caller, userID string) (app.StaffView, error)
	RemoveStaff(ctx context.Context, c app.Caller, userID, ownerPin string) error
	ListPermissions(ctx context.Context, c app.Caller) ([]app.StaffPermissionView, error)
	SetBuildingPermission(ctx context.Context, c app.Caller, userID, buildingID string, level access.Level) (app.BuildingLevel, error)
}

func callerOf(ctx context.Context) (app.Caller, error) {
	c, ok := app.CallerFrom(ctx)
	if !ok {
		return app.Caller{}, app.ErrUnauthenticated
	}
	return c, nil
}

func (s Server) ListStaff(ctx context.Context, req gen.ListStaffRequestObject) (gen.ListStaffResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	var pos *string
	if req.Params.Position != nil {
		p := string(*req.Params.Position)
		pos = &p
	}
	vs, err := s.staff.List(ctx, c, pos)
	if err != nil {
		return nil, err
	}
	items := make([]gen.Staff, len(vs))
	for i, v := range vs {
		items[i] = toStaff(v)
	}
	return gen.ListStaff200JSONResponse{Items: items}, nil
}

func (s Server) CreateStaff(ctx context.Context, req gen.CreateStaffRequestObject) (gen.CreateStaffResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, &app.ValidationError{Field: "body", Reason: "required"}
	}
	b := req.Body
	in := app.StaffInput{Name: b.Name, Position: string(b.Position), AppAccess: string(b.AppAccess), Phone: b.Phone,
		Username: b.Username, Contract: fromContract(b.Contract)}
	if b.BuildingAccess != nil {
		for _, a := range *b.BuildingAccess {
			in.BuildingAccess = append(in.BuildingAccess, app.BuildingLevel{BuildingID: a.BuildingId, Level: fromPermissionLevel(a.Level)})
		}
	}
	v, pin, err := s.staff.CreateStaff(ctx, c, req.Params.IdempotencyKey.String(), in)
	if err != nil {
		return nil, err
	}
	out := gen.CreateStaffResponse{Staff: toStaff(v)}
	if pin != nil {
		out.OneTimePin = &gen.OneTimePin{Pin: pin.Pin, ExpiresAt: pin.ExpiresAt}
	}
	return gen.CreateStaff201JSONResponse(out), nil
}

func (s Server) UpdateStaff(ctx context.Context, req gen.UpdateStaffRequestObject) (gen.UpdateStaffResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, &app.ValidationError{Field: "body", Reason: "required"}
	}
	b := req.Body
	in := app.StaffUpdate{Name: b.Name, Phone: b.Phone}
	if b.Position != nil {
		p := string(*b.Position)
		in.Position = &p
	}
	if b.AppAccess != nil {
		a := string(*b.AppAccess)
		in.AppAccess = &a
	}
	if b.Contract != nil {
		k := fromContract(*b.Contract)
		in.Contract = &k
	}
	v, err := s.staff.UpdateStaff(ctx, c, req.UserId, in)
	if err != nil {
		return nil, err
	}
	return gen.UpdateStaff200JSONResponse(toStaff(v)), nil
}

func (s Server) ResetStaffPin(ctx context.Context, req gen.ResetStaffPinRequestObject) (gen.ResetStaffPinResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	pin, err := s.staff.ResetPin(ctx, c, req.UserId)
	if err != nil {
		return nil, err
	}
	return gen.ResetStaffPin200JSONResponse{Pin: pin.Pin, ExpiresAt: pin.ExpiresAt}, nil
}

func (s Server) LockStaff(ctx context.Context, req gen.LockStaffRequestObject) (gen.LockStaffResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.staff.LockStaff(ctx, c, req.UserId)
	if err != nil {
		return nil, err
	}
	return gen.LockStaff200JSONResponse(toStaff(v)), nil
}

func (s Server) UnlockStaff(ctx context.Context, req gen.UnlockStaffRequestObject) (gen.UnlockStaffResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.staff.UnlockStaff(ctx, c, req.UserId)
	if err != nil {
		return nil, err
	}
	return gen.UnlockStaff200JSONResponse(toStaff(v)), nil
}

func (s Server) RemoveStaff(ctx context.Context, req gen.RemoveStaffRequestObject) (gen.RemoveStaffResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil || req.Body.OwnerPin == nil {
		return nil, &app.ValidationError{Field: "ownerPin", Reason: "required"}
	}
	if err = s.staff.RemoveStaff(ctx, c, req.UserId, *req.Body.OwnerPin); err != nil {
		return nil, err
	}
	return gen.RemoveStaff204Response{}, nil
}

func (s Server) ListStaffPermissions(ctx context.Context, _ gen.ListStaffPermissionsRequestObject) (gen.ListStaffPermissionsResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	vs, err := s.staff.ListPermissions(ctx, c)
	if err != nil {
		return nil, err
	}
	items := make([]gen.StaffPermission, len(vs))
	for i, v := range vs {
		items[i] = gen.StaffPermission{UserId: v.UserID, Name: v.Name, Role: gen.Role(v.Role), Access: toBuildingLevels(v.Access)}
	}
	return gen.ListStaffPermissions200JSONResponse{Items: items}, nil
}

func (s Server) SetBuildingPermission(ctx context.Context, req gen.SetBuildingPermissionRequestObject) (gen.SetBuildingPermissionResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, &app.ValidationError{Field: "body", Reason: "required"}
	}
	b, err := s.staff.SetBuildingPermission(ctx, c, req.UserId, req.BuildingId, fromPermissionLevel(req.Body.Level))
	if err != nil {
		return nil, err
	}
	return gen.SetBuildingPermission200JSONResponse(gen.BuildingAccess{BuildingId: b.BuildingID, Level: permissionLevel(b.Level)}), nil
}

// fromPermissionLevel fails closed: an unknown level becomes an invalid Level that the use case rejects.
func fromPermissionLevel(l gen.PermissionLevel) access.Level {
	if lv, err := access.ParseLevel(string(l)); err == nil {
		return lv
	}
	return access.Level(-1)
}

func toBuildingLevels(in []app.BuildingLevel) []gen.BuildingAccess {
	out := make([]gen.BuildingAccess, len(in))
	for i, b := range in {
		out[i] = gen.BuildingAccess{BuildingId: b.BuildingID, Level: permissionLevel(b.Level)}
	}
	return out
}

func fromContract(k gen.Contract) app.Contract {
	return app.Contract{PayType: string(k.PayType), Rate: money.Vnd(k.Rate), FixedAllowance: money.Vnd(k.FixedAllowance),
		StandardShifts: k.StandardShifts, StartDate: k.StartDate.Time, AnnualLeaveDays: k.AnnualLeaveDays}
}

func toStaff(v app.StaffView) gen.Staff {
	return gen.Staff{
		Id: v.ID, Name: v.Name, Phone: v.Phone, Position: gen.Position(v.Position), AppAccess: gen.AppAccess(v.AppAccess),
		Username: v.Username, Status: gen.StaffStatus(v.Status), LockedUntil: v.LockedUntil, LastActivityAt: v.LastActivityAt,
		BuildingAccess: toBuildingLevels(v.BuildingAccess),
		Contract: gen.Contract{PayType: gen.PayType(v.Contract.PayType), Rate: v.Contract.Rate.Int64(),
			FixedAllowance: v.Contract.FixedAllowance.Int64(), StandardShifts: v.Contract.StandardShifts,
			StartDate: openapi_types.Date{Time: v.Contract.StartDate}, AnnualLeaveDays: v.Contract.AnnualLeaveDays},
	}
}
