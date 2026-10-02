package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

type fakeStaffRepo struct {
	rows      map[string]StaffRow
	levels    map[string]map[string]access.Level
	sessions  map[string]int // deleted sessions per user
	buildings []string
	usernames map[string]bool
	roles     map[string]access.Role
}

func newFakeStaffRepo() *fakeStaffRepo {
	return &fakeStaffRepo{rows: map[string]StaffRow{}, levels: map[string]map[string]access.Level{}, sessions: map[string]int{},
		buildings: []string{"b1", "b2"}, usernames: map[string]bool{}, roles: map[string]access.Role{"us_owner": access.RoleOwner}}
}

func (r *fakeStaffRepo) ListStaff(_ context.Context, _ Tx, position, userID *string) ([]StaffRow, error) {
	var out []StaffRow
	for _, row := range r.rows {
		if (position == nil || *position == row.Position) && (userID == nil || *userID == row.ID) {
			out = append(out, row)
		}
	}
	return out, nil
}

func (r *fakeStaffRepo) StaffLevels(context.Context, Tx) (map[string]map[string]access.Level, error) {
	return r.levels, nil
}

func (r *fakeStaffRepo) InsertStaff(_ context.Context, _ Tx, s StaffInsert) error {
	if s.Username != nil {
		if r.usernames[*s.Username] {
			return ErrConflict
		}
		r.usernames[*s.Username] = true
	}
	r.rows[s.ID] = StaffRow{ID: s.ID, Name: s.Name, Role: s.Role, AppAccess: s.AppAccess, Status: "ACTIVE", Username: s.Username,
		Phone: s.Phone, Position: s.Position, Contract: s.Contract}
	r.roles[s.ID] = access.Role(s.Role)
	return nil
}

func (r *fakeStaffRepo) UpdateStaff(_ context.Context, _ Tx, id string, p StaffPatch) (bool, error) {
	row, ok := r.rows[id]
	if !ok {
		return false, nil
	}
	if p.Position != nil {
		row.Position = *p.Position
	}
	if p.AppAccess != nil {
		row.AppAccess = *p.AppAccess
	}
	if p.Name != nil {
		row.Name = *p.Name
	}
	r.rows[id] = row
	return true, nil
}

func (r *fakeStaffRepo) SetStatus(_ context.Context, _ Tx, id, status string) (bool, error) {
	row, ok := r.rows[id]
	if !ok || row.Status == "REMOVED" {
		return false, nil
	}
	row.Status = status
	r.rows[id] = row
	return true, nil
}

func (r *fakeStaffRepo) DeleteUserSessions(_ context.Context, _ Tx, id string) error {
	r.sessions[id]++
	return nil
}

func (r *fakeStaffRepo) SetBuildingLevel(_ context.Context, _ Tx, u, b string, l access.Level, _ time.Time) error {
	if r.levels[u] == nil {
		r.levels[u] = map[string]access.Level{}
	}
	r.levels[u][b] = l
	return nil
}

func (r *fakeStaffRepo) BuildingLevel(_ context.Context, _ Tx, u, b string) (access.Level, error) {
	return r.levels[u][b], nil
}

func (r *fakeStaffRepo) BuildingExists(_ context.Context, _ Tx, id string) (bool, error) {
	for _, b := range r.buildings {
		if b == id {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeStaffRepo) BuildingIDs(context.Context, Tx) ([]string, error) { return r.buildings, nil }

func (r *fakeStaffRepo) UserRole(_ context.Context, _ Tx, id string) (access.Role, string, bool, error) {
	role, ok := r.roles[id]
	if row, staff := r.rows[id]; staff {
		return role, row.Status, ok, nil
	}
	return role, "ACTIVE", ok, nil
}

func (r *fakeStaffRepo) PermissionUsers(context.Context, Tx) ([]User, error) {
	var out []User
	for id, row := range r.rows {
		if row.AppAccess != "NONE" && row.Status != "REMOVED" {
			out = append(out, User{ID: id, Name: row.Name, Role: access.Role(row.Role)})
		}
	}
	return out, nil
}

type fixedPin struct{ pin string }

func (f fixedPin) New() (string, error) { return f.pin, nil }

type fakeShifts struct{ open map[string]bool }

func (f fakeShifts) HasOpenShift(_ context.Context, _ Tx, id string) (bool, error) {
	return f.open[id], nil
}

type staffRig struct {
	s      *Staff
	repo   *fakeStaffRepo
	auth   authRig
	audit  *fakeAudit
	idem   *fakeIdem
	shifts fakeShifts
	owner  Caller
	mgr    Caller
}

const (
	ownerPin = "482915"
	issued   = "905731"
)

func newStaffRig(t *testing.T) staffRig {
	t.Helper()
	ar := newAuthRig(t, allowAll{}, allowAll{})
	// The owner is "ann" of tenant tn_a in the auth rig (PIN 482915).
	repo := newFakeStaffRepo()
	audit, idem, shifts := &fakeAudit{}, &fakeIdem{m: map[string]idemState{}}, fakeShifts{open: map[string]bool{}}
	s := NewStaff(ar.a.sessions.uow, repo, ar.a, ar.repo, fixedPin{issued}, idem, audit, shifts, &seqIDs{}, ar.clock)
	return staffRig{s: s, repo: repo, auth: ar, audit: audit, idem: idem, shifts: shifts,
		owner: Caller{TenantID: "tn_a", UserID: "us_ann", Role: access.RoleOwner},
		mgr:   Caller{TenantID: "tn_a", UserID: "us_mgr", Role: access.RoleManager}}
}

func okInput() StaffInput {
	u := "linh"
	return StaffInput{Name: "Linh", Position: "FRONT_DESK", AppAccess: "RECEPTIONIST", Username: &u,
		Contract:       Contract{PayType: "MONTHLY", Rate: 7_000_000, StandardShifts: 26, StartDate: t0, AnnualLeaveDays: 12},
		BuildingAccess: []BuildingLevel{{"b1", access.EDIT}, {"b2", access.VIEW}}}
}

func TestCreateStaff_OneTimePinOnce_SG1101_AC1(t *testing.T) {
	r := newStaffRig(t)
	ctx := context.Background()
	v, pin, err := r.s.CreateStaff(ctx, r.owner, "k1", okInput())
	if err != nil || pin == nil || pin.Pin != issued || !pin.ExpiresAt.Equal(t0.Add(OneTimePinTTL)) {
		t.Fatalf("create: %+v %v %v", v, pin, err)
	}
	if v.Status != "ACTIVE" || v.AppAccess != "RECEPTIONIST" || v.Username == nil || len(v.BuildingAccess) != 2 ||
		v.BuildingAccess[0].Level != access.EDIT || v.BuildingAccess[1].Level != access.VIEW {
		t.Fatalf("view: %+v", v)
	}
	// The PIN is stored hashed with must_change and a 24 h expiry.
	cred := r.auth.repo.users[authKey{"tn_a", v.ID}].Pin
	if cred.Hash == issued || !cred.MustChange || cred.OneTimeExpiresAt == nil || !cred.OneTimeExpiresAt.Equal(t0.Add(OneTimePinTTL)) {
		t.Fatalf("credential: %+v", cred)
	}
	// The same key and body replays without the PIN; the stored response never holds it.
	v2, pin2, err := r.s.CreateStaff(ctx, r.owner, "k1", okInput())
	if err != nil || pin2 != nil || v2.ID != v.ID || len(r.repo.rows) != 1 {
		t.Fatalf("replay: %+v %v %v rows=%d", v2, pin2, err, len(r.repo.rows))
	}
	for _, st := range r.idem.snapshot() {
		if strings.Contains(string(st.body), issued) {
			t.Fatal("PIN in the idempotency store")
		}
	}
	// A different body under the same key is refused.
	other := okInput()
	other.Name = "Other"
	if _, _, err = r.s.CreateStaff(ctx, r.owner, "k1", other); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Fatalf("reused key: %v", err)
	}
}

func TestCreateStaff_NoAppAccessHasNoPinOrUsername_SG1101_AC1(t *testing.T) {
	r := newStaffRig(t)
	in := okInput()
	in.AppAccess, in.Position = "NONE", "SECURITY"
	v, pin, err := r.s.CreateStaff(context.Background(), r.owner, "k1", in)
	if err != nil || pin != nil || v.Username != nil || len(r.auth.repo.users) != 2 { // the two tenants' users of the rig only
		t.Fatalf("%+v %v %v", v, pin, err)
	}
	if len(r.repo.levels[v.ID]) != 0 {
		t.Fatal("building access applies only when app access is not NONE")
	}
}

func TestCreateStaff_Validation_SG1101(t *testing.T) {
	r := newStaffRig(t)
	bad := map[string]func(*StaffInput){
		"no username":      func(i *StaffInput) { i.Username = nil },
		"bad username":     func(i *StaffInput) { u := "Linh Nguyen"; i.Username = &u },
		"empty name":       func(i *StaffInput) { i.Name = "" },
		"position":         func(i *StaffInput) { i.Position = "BOSS" },
		"access":           func(i *StaffInput) { i.AppAccess = "OWNER" },
		"pay type":         func(i *StaffInput) { i.Contract.PayType = "WEEKLY" },
		"negative rate":    func(i *StaffInput) { i.Contract.Rate = -1 },
		"unknown level":    func(i *StaffInput) { i.BuildingAccess = []BuildingLevel{{"b1", access.Level(7)}} },
		"unknown building": func(i *StaffInput) { i.BuildingAccess = []BuildingLevel{{"nope", access.EDIT}} },
	}
	for name, mut := range bad {
		in := okInput()
		mut(&in)
		var ve *ValidationError
		if _, _, err := r.s.CreateStaff(context.Background(), r.owner, "k-"+name, in); !errors.As(err, &ve) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if len(r.repo.rows) != 0 {
		t.Fatal("nothing may be stored")
	}
	_, _, _ = r.s.CreateStaff(context.Background(), r.owner, "ok1", okInput())
	if _, _, err := r.s.CreateStaff(context.Background(), r.owner, "ok2", okInput()); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate username: %v", err)
	}
}

func TestStaff_RoleOnlyOperations_SG1101_AC2(t *testing.T) {
	r := newStaffRig(t)
	ctx := context.Background()
	v, _, _ := r.s.CreateStaff(ctx, r.owner, "k1", okInput())
	// MANAGER may reset PINs and lock; it may not create, update or remove (owner only).
	if _, err := r.s.ResetPin(ctx, r.mgr, v.ID); err != nil {
		t.Errorf("manager reset: %v", err)
	}
	if _, err := r.s.LockStaff(ctx, r.mgr, v.ID); err != nil {
		t.Errorf("manager lock: %v", err)
	}
	for name, err := range map[string]error{
		"create": func() error { _, _, err := r.s.CreateStaff(ctx, r.mgr, "k2", okInput()); return err }(),
		"update": func() error { _, err := r.s.UpdateStaff(ctx, r.mgr, v.ID, StaffUpdate{}); return err }(),
		"remove": r.s.RemoveStaff(ctx, r.mgr, v.ID, ownerPin),
		"list":   func() error { _, err := r.s.List(ctx, r.mgr, nil); return err }(),
		"perms":  func() error { _, err := r.s.ListPermissions(ctx, r.mgr); return err }(),
		"set":    func() error { _, err := r.s.SetBuildingPermission(ctx, r.mgr, v.ID, "b1", access.EDIT); return err }(),
	} {
		if !errors.Is(err, access.ErrRoleForbidden) {
			t.Errorf("manager %s: %v", name, err)
		}
	}
	rec := Caller{TenantID: "tn_a", UserID: "us_rec", Role: access.RoleReceptionist}
	if _, err := r.s.ResetPin(ctx, rec, v.ID); !errors.Is(err, access.ErrRoleForbidden) {
		t.Errorf("receptionist reset: %v", err)
	}
}

func TestResetPin_SG1101(t *testing.T) {
	r := newStaffRig(t)
	ctx := context.Background()
	v, _, _ := r.s.CreateStaff(ctx, r.owner, "k1", okInput())
	pin, err := r.s.ResetPin(ctx, r.owner, v.ID)
	if err != nil || pin.Pin != issued || r.repo.sessions[v.ID] != 1 {
		t.Fatalf("%v %v sessions=%d", pin, err, r.repo.sessions[v.ID])
	}
	if strings.Contains(pin.String(), issued) {
		t.Fatal("OneTimePin must not print its PIN")
	}
	in := okInput()
	in.AppAccess, in.Username = "NONE", nil
	g, _, _ := r.s.CreateStaff(ctx, r.owner, "k2", in)
	if _, err = r.s.ResetPin(ctx, r.owner, g.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("no app access: %v", err)
	}
	if _, err = r.s.ResetPin(ctx, r.owner, "nobody"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}
}

func TestLockUnlock_SG1101(t *testing.T) {
	r := newStaffRig(t)
	ctx := context.Background()
	v, _, _ := r.s.CreateStaff(ctx, r.owner, "k1", okInput())
	l, err := r.s.LockStaff(ctx, r.owner, v.ID)
	if err != nil || l.Status != "LOCKED" || r.repo.sessions[v.ID] != 1 {
		t.Fatalf("lock: %+v %v", l, err)
	}
	// A wrong-PIN lock also shows as LOCKED until unlocked.
	k := authKey{"tn_a", v.ID}
	until := t0.Add(10 * time.Minute)
	u := r.auth.repo.users[k]
	u.Pin.LockedUntil, u.Pin.FailedCount = &until, 0
	r.auth.repo.users[k] = u
	if u, err := r.s.UnlockStaff(ctx, r.owner, v.ID); err != nil || u.Status != "ACTIVE" || u.LockedUntil != nil {
		t.Fatalf("unlock: %+v %v", u, err)
	}
	if r.auth.repo.users[k].Pin.LockedUntil != nil {
		t.Fatal("unlock must clear the wrong-PIN lock")
	}
	if _, err = r.s.LockStaff(ctx, r.owner, "nobody"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}
}

func TestRemoveStaff_NeedsOwnerPin_SG1101_AC3(t *testing.T) {
	r := newStaffRig(t)
	ctx := context.Background()
	v, _, _ := r.s.CreateStaff(ctx, r.owner, "k1", okInput())
	if err := r.s.RemoveStaff(ctx, r.owner, v.ID, pinWrong); !errors.Is(err, ErrOwnerPinInvalid) {
		t.Fatalf("wrong owner PIN: %v", err)
	}
	if got := r.auth.repo.users[authKey{"tn_a", "us_ann"}].Pin.FailedCount; got != 1 {
		t.Fatalf("a wrong owner PIN counts towards the lock, got %d", got)
	}
	if r.repo.rows[v.ID].Status != "ACTIVE" {
		t.Fatal("not removed")
	}
	r.shifts.open[v.ID] = true
	if err := r.s.RemoveStaff(ctx, r.owner, v.ID, ownerPin); !errors.Is(err, ErrShiftOpen) {
		t.Fatalf("open shift: %v", err)
	}
	delete(r.shifts.open, v.ID)
	if err := r.s.RemoveStaff(ctx, r.owner, v.ID, ownerPin); err != nil {
		t.Fatal(err)
	}
	if row := r.repo.rows[v.ID]; row.Status != "REMOVED" || r.repo.sessions[v.ID] != 1 {
		t.Fatalf("removed row %+v sessions=%d", row, r.repo.sessions[v.ID])
	}
	if err := r.s.RemoveStaff(ctx, r.owner, v.ID, ownerPin); !errors.Is(err, ErrNotFound) {
		t.Fatalf("remove twice: %v", err)
	}
	var ve *ValidationError
	if err := r.s.RemoveStaff(ctx, r.owner, v.ID, "12"); !errors.As(err, &ve) {
		t.Fatalf("bad pin shape: %v", err)
	}
}

func TestRemoveStaff_FiveWrongOwnerPinsLock_SG1101_AC3(t *testing.T) {
	r := newStaffRig(t)
	v, _, _ := r.s.CreateStaff(context.Background(), r.owner, "k1", okInput())
	var err error
	for i := 0; i < 5; i++ {
		err = r.s.RemoveStaff(context.Background(), r.owner, v.ID, pinWrong)
	}
	var locked *AccountLockedError
	if !errors.As(err, &locked) || r.repo.rows[v.ID].Status != "ACTIVE" {
		t.Fatalf("%v", err)
	}
}

func TestUpdateStaff_SG1101(t *testing.T) {
	r := newStaffRig(t)
	ctx := context.Background()
	v, _, _ := r.s.CreateStaff(ctx, r.owner, "k1", okInput())
	pos, acc := "MANAGER", "MANAGER"
	u, err := r.s.UpdateStaff(ctx, r.owner, v.ID, StaffUpdate{Position: &pos, AppAccess: &acc})
	if err != nil || u.Position != "MANAGER" || u.AppAccess != "MANAGER" {
		t.Fatalf("%+v %v", u, err)
	}
	none := "NONE"
	if _, err = r.s.UpdateStaff(ctx, r.owner, v.ID, StaffUpdate{AppAccess: &none}); err != nil || r.repo.sessions[v.ID] != 1 {
		t.Fatalf("NONE must end sessions: %v %d", err, r.repo.sessions[v.ID])
	}
	// A person created without a username cannot be given access.
	in := okInput()
	in.AppAccess, in.Username, in.Position = "NONE", nil, "SECURITY"
	g, _, _ := r.s.CreateStaff(ctx, r.owner, "k2", in)
	rec := "RECEPTIONIST"
	var ve *ValidationError
	if _, err = r.s.UpdateStaff(ctx, r.owner, g.ID, StaffUpdate{AppAccess: &rec}); !errors.As(err, &ve) {
		t.Fatalf("no username: %v", err)
	}
	bad := "BOSS"
	if _, err = r.s.UpdateStaff(ctx, r.owner, v.ID, StaffUpdate{Position: &bad}); !errors.As(err, &ve) {
		t.Fatalf("bad position: %v", err)
	}
	if _, err = r.s.UpdateStaff(ctx, r.owner, "nobody", StaffUpdate{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}
}

func TestSetBuildingPermission_SG501(t *testing.T) {
	r := newStaffRig(t)
	ctx := context.Background()
	v, _, _ := r.s.CreateStaff(ctx, r.owner, "k1", okInput())
	got, err := r.s.SetBuildingPermission(ctx, r.owner, v.ID, "b1", access.VIEW)
	if err != nil || got.Level != access.VIEW || r.repo.levels[v.ID]["b1"] != access.VIEW {
		t.Fatalf("%+v %v", got, err)
	}
	last := r.audit.entries[len(r.audit.entries)-1]
	if last.Action != "BUILDING_PERMISSION_SET" || last.ActorID != "us_ann" || last.EntityID != v.ID ||
		!strings.Contains(string(last.Before), `"EDIT"`) || !strings.Contains(string(last.After), `"VIEW"`) {
		t.Fatalf("audit must hold who, whom, building, old and new level: %+v %s %s", last, last.Before, last.After)
	}
	var ve *ValidationError
	if _, err = r.s.SetBuildingPermission(ctx, r.owner, "us_owner", "b1", access.NONE); !errors.As(err, &ve) {
		t.Fatalf("owner lock-out: %v", err)
	}
	if _, err = r.s.SetBuildingPermission(ctx, r.owner, v.ID, "nope", access.EDIT); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown building: %v", err)
	}
	if _, err = r.s.SetBuildingPermission(ctx, r.owner, "nobody", "b1", access.EDIT); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown user: %v", err)
	}
	if _, err = r.s.SetBuildingPermission(ctx, r.owner, v.ID, "b1", access.Level(9)); !errors.As(err, &ve) {
		t.Fatalf("bad level: %v", err)
	}
	list, err := r.s.ListPermissions(ctx, r.owner)
	if err != nil || len(list) != 1 || list[0].Access[0].Level != access.VIEW || list[0].Access[1].Level != access.VIEW {
		t.Fatalf("%+v %v", list, err)
	}
}

func TestStaffAudit_HoldsNoPersonalData_SG1101(t *testing.T) {
	r := newStaffRig(t)
	in := okInput()
	phone := "0912345678"
	in.Phone = &phone
	if _, _, err := r.s.CreateStaff(context.Background(), r.owner, "k1", in); err != nil {
		t.Fatal(err)
	}
	for _, e := range r.audit.entries {
		for _, secret := range []string{issued, phone, "Linh", "linh"} {
			if strings.Contains(string(e.Before)+string(e.After), secret) {
				t.Fatalf("%q in audit entry %s", secret, e.After)
			}
		}
	}
}
