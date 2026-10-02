package app

import (
	"context"
	"errors"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/money"
)

var (
	// ErrShiftOpen: a person with an open shift cannot be removed (HTTP 409 SHIFT_OPEN).
	ErrShiftOpen = errors.New("staff member has an open shift")
	// ErrOwnerPinInvalid: the owner PIN re-entered for a sensitive change is wrong (HTTP 403 OWNER_PIN_INVALID).
	ErrOwnerPinInvalid = errors.New("owner pin invalid")
)

// Contract is the pay terms of a staff member.
type Contract struct {
	PayType         string
	Rate            money.Vnd
	FixedAllowance  money.Vnd
	StandardShifts  int
	StartDate       time.Time // a calendar date, midnight UTC
	AnnualLeaveDays int
}

// BuildingLevel is one building and the stored access level of a person to it.
type BuildingLevel struct {
	BuildingID string
	Level      access.Level
}

// StaffView is a staff member as the owner sees them.
type StaffView struct {
	ID, Name       string
	Phone          *string
	Position       string
	AppAccess      string
	Status         string // ACTIVE, LOCKED or REMOVED (a sign-in lock from wrong PINs shows as LOCKED)
	Username       *string
	LockedUntil    *time.Time
	LastActivityAt *time.Time
	Contract       Contract
	BuildingAccess []BuildingLevel
}

// StaffRow is a stored staff member without building access.
type StaffRow struct {
	ID, Name, Role, AppAccess, Status string
	Username, Phone                   *string
	Position                          string
	Contract                          Contract
	PinLockedUntil, LastActivityAt    *time.Time
}

// StaffInsert is a staff member to store; Role follows AppAccess.
type StaffInsert struct {
	ID, Name, Role, AppAccess string
	Username, Phone           *string
	Position                  string
	Contract                  Contract
}

// StaffPatch changes the given fields only.
type StaffPatch struct {
	Name, Phone, Position, AppAccess, Role, PayType *string
	Rate, FixedAllowance                            *money.Vnd
	StandardShifts, AnnualLeaveDays                 *int
	StartDate                                       *time.Time
}

// StaffPermissionView is one signed-in person and their stored level per building.
type StaffPermissionView struct {
	UserID, Name string
	Role         access.Role
	Access       []BuildingLevel
}

// StaffRepo is tenant-scoped like IdentityRepo.
type StaffRepo interface {
	ListStaff(ctx context.Context, tx Tx, position, userID *string) ([]StaffRow, error)
	// StaffLevels is every stored level by user id.
	StaffLevels(ctx context.Context, tx Tx) (map[string]map[string]access.Level, error)
	InsertStaff(ctx context.Context, tx Tx, s StaffInsert) error
	// UpdateStaff reports false when the person is not a staff member or is removed.
	UpdateStaff(ctx context.Context, tx Tx, id string, p StaffPatch) (bool, error)
	SetStatus(ctx context.Context, tx Tx, id, status string) (bool, error)
	DeleteUserSessions(ctx context.Context, tx Tx, userID string) error
	SetBuildingLevel(ctx context.Context, tx Tx, userID, buildingID string, level access.Level, now time.Time) error
	BuildingLevel(ctx context.Context, tx Tx, userID, buildingID string) (access.Level, error)
	BuildingExists(ctx context.Context, tx Tx, id string) (bool, error)
	BuildingIDs(ctx context.Context, tx Tx) ([]string, error)
	// UserRole is the role and status of any user (owner included); false when unknown.
	UserRole(ctx context.Context, tx Tx, id string) (role access.Role, status string, ok bool, err error)
	PermissionUsers(ctx context.Context, tx Tx) ([]User, error)
}

// OpenShifts says whether a person has a shift open. L-B2 (shifts) provides the real answer.
type OpenShifts interface {
	HasOpenShift(ctx context.Context, tx Tx, userID string) (bool, error)
}

// PinGenerator makes a random one-time PIN that passes access.ValidateNewPin.
type PinGenerator interface{ New() (string, error) }

// OneTimePin is shown once to the person who issues it.
type OneTimePin struct {
	Pin       string
	ExpiresAt time.Time
}

// String and GoString keep the PIN out of %v and %+v output.
func (p OneTimePin) String() string   { return "OneTimePin{" + redacted + "}" }
func (p OneTimePin) GoString() string { return p.String() }
