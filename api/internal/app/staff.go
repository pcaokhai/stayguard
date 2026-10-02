package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

const (
	statusLocked  = "LOCKED"
	statusRemoved = "REMOVED"
	staffRoute    = "POST /v1/owner/staff"
	maxNameLen    = 80
)

var (
	positions = map[string]bool{"FRONT_DESK": true, "HOUSEKEEPING": true, "SECURITY": true, "MANAGER": true, "MAINTENANCE": true, "OTHER": true}
	payTypes  = map[string]bool{"MONTHLY": true, "PER_SHIFT": true, "HOURLY": true}
	// appAccessRole is the role a person signs in with; NONE keeps the least-privileged role and cannot sign in.
	appAccessRole = map[string]access.Role{
		"MANAGER": access.RoleManager, "RECEPTIONIST": access.RoleReceptionist, "HOUSEKEEPING": access.RoleHousekeeping,
		accessNone: access.RoleHousekeeping,
	}
)

// Staff is the owner's staff management: people, one-time PINs, locks, removal and building access.
type Staff struct {
	uow    UnitOfWork
	repo   StaffRepo
	auth   *Auth
	authDB AuthRepo
	pins   PinGenerator
	idem   IdempotencyStore
	audit  AuditWriter
	shifts OpenShifts
	ids    IDGenerator
	clock  Clock
	authz  access.Authorizer
}

func NewStaff(uow UnitOfWork, repo StaffRepo, auth *Auth, authDB AuthRepo, pins PinGenerator, idem IdempotencyStore,
	audit AuditWriter, shifts OpenShifts, ids IDGenerator, clock Clock) *Staff {
	return &Staff{uow: uow, repo: repo, auth: auth, authDB: authDB, pins: pins, idem: idem, audit: audit, shifts: shifts, ids: ids, clock: clock}
}

// StaffInput is the body of createStaff.
type StaffInput struct {
	Name, Position, AppAccess string
	Phone, Username           *string
	Contract                  Contract
	BuildingAccess            []BuildingLevel
}

// StaffUpdate changes the given fields only; Contract replaces the whole contract.
type StaffUpdate struct {
	Name, Phone, Position, AppAccess *string
	Contract                         *Contract
}

func (s *Staff) List(ctx context.Context, c Caller, position *string) ([]StaffView, error) {
	if err := s.authz.Check("listStaff", c.Role, access.EDIT); err != nil {
		return nil, err
	}
	if position != nil && !positions[*position] {
		return nil, &ValidationError{"position", "unknown position"}
	}
	var out []StaffView
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		var err error
		out, err = s.views(ctx, tx, position, nil)
		return err
	})
	return out, err
}

// views loads staff with building access; one of position and userID narrows the list.
func (s *Staff) views(ctx context.Context, tx Tx, position, userID *string) ([]StaffView, error) {
	rows, err := s.repo.ListStaff(ctx, tx, position, userID)
	if err != nil {
		return nil, err
	}
	levels, err := s.repo.StaffLevels(ctx, tx)
	if err != nil {
		return nil, err
	}
	buildings, err := s.repo.BuildingIDs(ctx, tx)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	out := make([]StaffView, len(rows))
	for i, r := range rows {
		out[i] = staffView(r, buildings, levels[r.ID], now)
	}
	return out, nil
}

func staffView(r StaffRow, buildings []string, levels map[string]access.Level, now time.Time) StaffView {
	v := StaffView{ID: r.ID, Name: r.Name, Phone: r.Phone, Position: r.Position, AppAccess: r.AppAccess, Status: r.Status,
		Username: r.Username, LastActivityAt: r.LastActivityAt, Contract: r.Contract, BuildingAccess: make([]BuildingLevel, len(buildings))}
	if r.Status == statusActive && r.PinLockedUntil != nil && now.Before(*r.PinLockedUntil) {
		v.Status, v.LockedUntil = statusLocked, r.PinLockedUntil
	}
	for i, id := range buildings {
		v.BuildingAccess[i] = BuildingLevel{BuildingID: id, Level: levels[id]}
	}
	return v
}

func validateContract(k Contract) error {
	switch {
	case !payTypes[k.PayType]:
		return &ValidationError{"contract.payType", "unknown pay type"}
	case k.Rate < 0 || k.FixedAllowance < 0:
		return &ValidationError{"contract", "amounts must not be negative"}
	case k.StandardShifts < 0 || k.AnnualLeaveDays < 0:
		return &ValidationError{"contract", "counts must not be negative"}
	}
	return nil
}

func validateName(name string) error {
	if name == "" || len(name) > maxNameLen {
		return &ValidationError{"name", "required, at most 80 characters"}
	}
	return nil
}

func validateStaffInput(in StaffInput) error {
	if err := validateName(in.Name); err != nil {
		return err
	}
	if !positions[in.Position] {
		return &ValidationError{"position", "unknown position"}
	}
	if _, ok := appAccessRole[in.AppAccess]; !ok {
		return &ValidationError{"appAccess", "unknown app access"}
	}
	if in.AppAccess != accessNone && (in.Username == nil || !validUsername(*in.Username)) {
		return &ValidationError{"username", "required, 2 to 32 of a-z 0-9 . _"}
	}
	for _, b := range in.BuildingAccess {
		if !b.Level.Valid() {
			return &ValidationError{"buildingAccess", "unknown level"}
		}
	}
	return validateContract(in.Contract)
}

// validUsername matches the contract pattern ^[a-z0-9_.]{2,32}$.
func validUsername(u string) bool {
	if len(u) < 2 || len(u) > 32 {
		return false
	}
	for i := 0; i < len(u); i++ {
		c := u[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' && c != '.' {
			return false
		}
	}
	return true
}

// staffCreated is what the idempotency store keeps: the staff member, never the PIN.
type staffCreated struct {
	Staff StaffView `json:"staff"`
}

// CreateStaff adds a person. The one-time PIN comes back only on the first answer; a replay of the same key
// returns the staff member without it (it was shown once), and resetStaffPin issues a new one.
func (s *Staff) CreateStaff(ctx context.Context, c Caller, key string, in StaffInput) (StaffView, *OneTimePin, error) {
	if err := s.authz.Check("createStaff", c.Role, access.EDIT); err != nil {
		return StaffView{}, nil, err
	}
	if key == "" || len(key) > maxIdempotencyKeyBytes {
		return StaffView{}, nil, ErrInvalidIdempotencyKey
	}
	if err := validateStaffInput(in); err != nil {
		return StaffView{}, nil, err
	}
	var pin *OneTimePin
	var hash string
	if in.AppAccess != accessNone {
		var err error
		if pin, hash, err = s.newOneTimePin(); err != nil {
			return StaffView{}, nil, err
		}
	}
	body, err := json.Marshal(in) // no secret in the input: the PIN is generated here
	if err != nil {
		return StaffView{}, nil, fmt.Errorf("encode staff request: %w", err)
	}
	var view StaffView
	var issued *OneTimePin
	err = s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		oc, err := s.idem.Begin(ctx, tx, staffRoute, key, RequestHash(body))
		if err != nil {
			return err
		}
		if oc.Replay {
			var stored staffCreated
			if err = json.Unmarshal(oc.Body, &stored); err != nil {
				return fmt.Errorf("stored staff response: %w", err)
			}
			view = stored.Staff
			return nil
		}
		if view, err = s.insert(ctx, tx, c, in, hash, pin); err != nil {
			return err
		}
		issued = pin
		stored, err := json.Marshal(staffCreated{Staff: view})
		if err != nil {
			return fmt.Errorf("encode staff response: %w", err)
		}
		return s.idem.Complete(ctx, tx, staffRoute, key, statusCreated, stored)
	})
	if err != nil {
		return StaffView{}, nil, err
	}
	return view, issued, nil
}

// newOneTimePin returns a PIN and its hash; the PIN is valid for OneTimePinTTL from now.
func (s *Staff) newOneTimePin() (*OneTimePin, string, error) {
	pin, err := s.pins.New()
	if err != nil {
		return nil, "", fmt.Errorf("generate pin: %w", err)
	}
	hash, err := s.auth.hasher.Hash(pin)
	if err != nil {
		return nil, "", fmt.Errorf("hash pin: %w", err)
	}
	return &OneTimePin{Pin: pin, ExpiresAt: s.clock.Now().Add(OneTimePinTTL)}, hash, nil
}

func (s *Staff) insert(ctx context.Context, tx Tx, c Caller, in StaffInput, hash string, pin *OneTimePin) (StaffView, error) {
	now := s.clock.Now()
	id := s.ids.New(userPrefix)
	username := in.Username
	if in.AppAccess == accessNone {
		username = nil
	}
	if pin != nil {
		if err := s.checkBuildings(ctx, tx, in.BuildingAccess); err != nil {
			return StaffView{}, err
		}
	}
	err := s.repo.InsertStaff(ctx, tx, StaffInsert{ID: id, Name: in.Name, Role: string(appAccessRole[in.AppAccess]), AppAccess: in.AppAccess,
		Username: username, Phone: in.Phone, Position: in.Position, Contract: in.Contract})
	if err != nil {
		return StaffView{}, err
	}
	if pin != nil {
		exp := pin.ExpiresAt
		if err = s.authDB.SetPin(ctx, tx, id, hash, true, &exp, now); err != nil {
			return StaffView{}, err
		}
		if err = s.grantLevels(ctx, tx, id, in.BuildingAccess, now); err != nil {
			return StaffView{}, err
		}
	}
	if err = s.record(ctx, tx, c, "STAFF_CREATED", id, nil, map[string]any{"position": in.Position, "appAccess": in.AppAccess}); err != nil {
		return StaffView{}, err
	}
	return s.view(ctx, tx, id)
}

func (s *Staff) checkBuildings(ctx context.Context, tx Tx, levels []BuildingLevel) error {
	for _, b := range levels {
		ok, err := s.repo.BuildingExists(ctx, tx, b.BuildingID)
		if err != nil {
			return err
		}
		if !ok {
			return &ValidationError{"buildingAccess", "unknown building"}
		}
	}
	return nil
}

func (s *Staff) grantLevels(ctx context.Context, tx Tx, userID string, levels []BuildingLevel, now time.Time) error {
	for _, b := range levels {
		if err := s.repo.SetBuildingLevel(ctx, tx, userID, b.BuildingID, b.Level, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Staff) view(ctx context.Context, tx Tx, id string) (StaffView, error) {
	vs, err := s.views(ctx, tx, nil, &id)
	if err != nil {
		return StaffView{}, err
	}
	if len(vs) == 0 {
		return StaffView{}, ErrNotFound
	}
	return vs[0], nil
}

// record appends an audit row with ids, positions and levels only: no names, phones or PINs.
func (s *Staff) record(ctx context.Context, tx Tx, c Caller, action, entityID string, before, after map[string]any) error {
	e := AuditEntry{ID: s.ids.New(auditPrefix), ActorID: c.UserID, Action: action, EntityType: "user", EntityID: entityID}
	var err error
	if before != nil {
		if e.Before, err = json.Marshal(before); err != nil {
			return err
		}
	}
	if after != nil {
		if e.After, err = json.Marshal(after); err != nil {
			return err
		}
	}
	return s.audit.Append(ctx, tx, e)
}

func (s *Staff) UpdateStaff(ctx context.Context, c Caller, userID string, in StaffUpdate) (StaffView, error) {
	if err := s.authz.Check("updateStaff", c.Role, access.EDIT); err != nil {
		return StaffView{}, err
	}
	patch, err := s.patchOf(in)
	if err != nil {
		return StaffView{}, err
	}
	var out StaffView
	err = s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		cur, err := s.view(ctx, tx, userID)
		if err != nil || cur.Status == statusRemoved {
			return errOr(err, ErrNotFound)
		}
		if err = checkAccessChange(cur, in.AppAccess); err != nil {
			return err
		}
		if ok, uerr := s.repo.UpdateStaff(ctx, tx, userID, patch); uerr != nil || !ok {
			return errOr(uerr, ErrNotFound)
		}
		if in.AppAccess != nil && *in.AppAccess == accessNone {
			if err = s.repo.DeleteUserSessions(ctx, tx, userID); err != nil {
				return err
			}
		}
		if out, err = s.view(ctx, tx, userID); err != nil {
			return err
		}
		return s.record(ctx, tx, c, "STAFF_UPDATED", userID,
			map[string]any{"position": cur.Position, "appAccess": cur.AppAccess},
			map[string]any{"position": out.Position, "appAccess": out.AppAccess})
	})
	return out, err
}

// checkAccessChange: sign-in needs a username, and a person created without one (appAccess NONE) cannot be given access.
func checkAccessChange(cur StaffView, next *string) error {
	if next != nil && *next != accessNone && cur.Username == nil {
		return &ValidationError{"appAccess", "this person has no sign-in name; add them again with one"}
	}
	return nil
}

func (s *Staff) patchOf(in StaffUpdate) (StaffPatch, error) {
	p := StaffPatch{Name: in.Name, Phone: in.Phone, Position: in.Position, AppAccess: in.AppAccess}
	if in.Name != nil {
		if err := validateName(*in.Name); err != nil {
			return p, err
		}
	}
	if in.Position != nil && !positions[*in.Position] {
		return p, &ValidationError{"position", "unknown position"}
	}
	if in.AppAccess != nil {
		role, ok := appAccessRole[*in.AppAccess]
		if !ok {
			return p, &ValidationError{"appAccess", "unknown app access"}
		}
		if *in.AppAccess != accessNone { // NONE keeps the old role: it cannot sign in anyway
			r := string(role)
			p.Role = &r
		}
	}
	if k := in.Contract; k != nil {
		if err := validateContract(*k); err != nil {
			return p, err
		}
		p.PayType, p.Rate, p.FixedAllowance = &k.PayType, &k.Rate, &k.FixedAllowance
		p.StandardShifts, p.AnnualLeaveDays, p.StartDate = &k.StandardShifts, &k.AnnualLeaveDays, &k.StartDate
	}
	return p, nil
}

// ResetPin issues a new one-time PIN and ends the person's sessions. Unlike createStaff it takes no
// idempotency key: a retry simply issues another PIN and the earlier one stops working.
func (s *Staff) ResetPin(ctx context.Context, c Caller, userID string) (OneTimePin, error) {
	if err := s.authz.Check("resetStaffPin", c.Role, access.EDIT); err != nil {
		return OneTimePin{}, err
	}
	pin, hash, err := s.newOneTimePin()
	if err != nil {
		return OneTimePin{}, err
	}
	err = s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		cur, err := s.view(ctx, tx, userID)
		if err != nil || cur.Status == statusRemoved {
			return errOr(err, ErrNotFound)
		}
		if cur.AppAccess == accessNone {
			return ErrConflict // no sign-in, no PIN
		}
		exp := pin.ExpiresAt
		if err = s.authDB.SetPin(ctx, tx, userID, hash, true, &exp, s.clock.Now()); err != nil {
			return err
		}
		if err = s.repo.DeleteUserSessions(ctx, tx, userID); err != nil {
			return err
		}
		return s.record(ctx, tx, c, "STAFF_PIN_RESET", userID, nil, nil)
	})
	if err != nil {
		return OneTimePin{}, err
	}
	return *pin, nil
}

func (s *Staff) LockStaff(ctx context.Context, c Caller, userID string) (StaffView, error) {
	return s.setLock(ctx, c, "lockStaff", userID, statusLocked, "STAFF_LOCKED")
}

func (s *Staff) UnlockStaff(ctx context.Context, c Caller, userID string) (StaffView, error) {
	return s.setLock(ctx, c, "unlockStaff", userID, statusActive, "STAFF_UNLOCKED")
}

// setLock changes the status; locking ends the sessions, unlocking also clears a wrong-PIN lock.
func (s *Staff) setLock(ctx context.Context, c Caller, op, userID, status, action string) (StaffView, error) {
	if err := s.authz.Check(op, c.Role, access.EDIT); err != nil {
		return StaffView{}, err
	}
	var out StaffView
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		if ok, err := s.repo.SetStatus(ctx, tx, userID, status); err != nil || !ok {
			return errOr(err, ErrNotFound)
		}
		if status == statusLocked {
			if err := s.repo.DeleteUserSessions(ctx, tx, userID); err != nil {
				return err
			}
		} else if _, ok, err := s.authDB.PinState(ctx, tx, userID); err != nil {
			return err
		} else if ok {
			if err = s.authDB.SetPinFailures(ctx, tx, userID, 0, nil, nil); err != nil {
				return err
			}
		}
		var err error
		if out, err = s.view(ctx, tx, userID); err != nil {
			return err
		}
		return s.record(ctx, tx, c, action, userID, nil, nil)
	})
	return out, err
}

// RemoveStaff deactivates a person after the owner re-enters their PIN. History stays; sign-in ends at once.
func (s *Staff) RemoveStaff(ctx context.Context, c Caller, userID, ownerPin string) error {
	if err := s.authz.Check("removeStaff", c.Role, access.EDIT); err != nil {
		return err
	}
	if access.ValidatePinFormat(ownerPin) != nil {
		return &ValidationError{"ownerPin", "must be six digits"}
	}
	var outcome error
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		failure, err := s.auth.requireOwnerPin(ctx, tx, c.UserID, ownerPin, s.clock.Now())
		if err != nil || failure != nil {
			outcome = failure
			return err // a wrong owner PIN commits its count
		}
		cur, err := s.view(ctx, tx, userID)
		if err != nil || cur.Status == statusRemoved {
			return errOr(err, ErrNotFound)
		}
		if open, err := s.shifts.HasOpenShift(ctx, tx, userID); err != nil {
			return err
		} else if open {
			return ErrShiftOpen
		}
		if ok, err := s.repo.SetStatus(ctx, tx, userID, statusRemoved); err != nil || !ok {
			return errOr(err, ErrNotFound)
		}
		if err = s.repo.DeleteUserSessions(ctx, tx, userID); err != nil {
			return err
		}
		return s.record(ctx, tx, c, "STAFF_REMOVED", userID, nil, nil)
	})
	return errOr(err, outcome)
}

// ListPermissions is every signed-in staff member with their level per building.
func (s *Staff) ListPermissions(ctx context.Context, c Caller) ([]StaffPermissionView, error) {
	if err := s.authz.Check("listStaffPermissions", c.Role, access.EDIT); err != nil {
		return nil, err
	}
	var out []StaffPermissionView
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		users, err := s.repo.PermissionUsers(ctx, tx)
		if err != nil {
			return err
		}
		levels, err := s.repo.StaffLevels(ctx, tx)
		if err != nil {
			return err
		}
		buildings, err := s.repo.BuildingIDs(ctx, tx)
		if err != nil {
			return err
		}
		out = make([]StaffPermissionView, len(users))
		for i, u := range users {
			v := StaffPermissionView{UserID: u.ID, Name: u.Name, Role: u.Role, Access: make([]BuildingLevel, len(buildings))}
			for j, b := range buildings {
				v.Access[j] = BuildingLevel{BuildingID: b, Level: levels[u.ID][b]}
			}
			out[i] = v
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
		return nil
	})
	return out, err
}

// SetBuildingPermission sets one person's level on one building and audits old and new.
// The owner needs no row (implicit EDIT), so setting one for the owner is refused: no self-lock-out.
func (s *Staff) SetBuildingPermission(ctx context.Context, c Caller, userID, buildingID string, level access.Level) (BuildingLevel, error) {
	if err := s.authz.Check("setBuildingPermission", c.Role, access.EDIT); err != nil {
		return BuildingLevel{}, err
	}
	if !level.Valid() {
		return BuildingLevel{}, &ValidationError{"level", "must be NONE, VIEW or EDIT"}
	}
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		role, status, ok, err := s.repo.UserRole(ctx, tx, userID)
		if err != nil || !ok || status == statusRemoved {
			return errOr(err, ErrNotFound)
		}
		if role == access.RoleOwner {
			return &ValidationError{"userId", "the owner has full access"}
		}
		if exists, err := s.repo.BuildingExists(ctx, tx, buildingID); err != nil || !exists {
			return errOr(err, ErrNotFound)
		}
		old, err := s.repo.BuildingLevel(ctx, tx, userID, buildingID)
		if err != nil {
			return err
		}
		if err = s.repo.SetBuildingLevel(ctx, tx, userID, buildingID, level, s.clock.Now()); err != nil {
			return err
		}
		return s.recordPermission(ctx, tx, c, userID, buildingID, old, level)
	})
	return BuildingLevel{BuildingID: buildingID, Level: level}, err
}

func (s *Staff) recordPermission(ctx context.Context, tx Tx, c Caller, userID, buildingID string, old, level access.Level) error {
	return s.record(ctx, tx, c, "BUILDING_PERMISSION_SET", userID,
		map[string]any{"buildingId": buildingID, "level": old.String()}, map[string]any{"buildingId": buildingID, "level": level.String()})
}
