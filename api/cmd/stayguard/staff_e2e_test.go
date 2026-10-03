//go:build integration

package main

import (
	"context"
	"strings"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/adapter/crypto"
	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

const (
	staffTenant = "tn_staff"
	staffCode   = "staffco"
	ownerPIN    = "482915"
)

// seedOwner creates a real (non-trial) guesthouse with one owner who signs in with ownerPIN.
func (e *env) seedOwner() {
	e.t.Helper()
	e.exec(`INSERT INTO app.tenants (id, name, guesthouse_code) VALUES ($1, 'Staff Co', $2)`, staffTenant, staffCode)
	e.exec(`INSERT INTO app.users (id, tenant_id, name, role, app_access, username) VALUES ('us_owner', $1, 'Owner', 'OWNER', 'OWNER', 'owner1')`, staffTenant)
	hash, err := crypto.NewPinHasher(testDataKey).Hash(ownerPIN)
	if err != nil {
		e.t.Fatal(err)
	}
	err = postgres.NewUnitOfWork(e.pool).Do(context.Background(), staffTenant, func(ctx context.Context, tx app.Tx) error {
		return postgres.NewAuthRepo().SetPin(ctx, tx, "us_owner", hash, false, nil, e.start)
	})
	if err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) signIn(user, pin string) reply {
	return e.call("POST", "/v1/auth/sign-in", "", map[string]any{"guesthouseCode": staffCode, "username": user, "pin": pin})
}

func staffBody() map[string]any {
	return map[string]any{
		"name": "Linh", "position": "FRONT_DESK", "appAccess": "RECEPTIONIST", "username": "linh",
		"contract":       map[string]any{"payType": "MONTHLY", "rate": 7000000, "fixedAllowance": 0, "standardShifts": 26, "startDate": "2026-10-01", "annualLeaveDays": 12},
		"buildingAccess": []map[string]any{{"buildingId": stayBuilding, "level": "EDIT"}},
	}
}

func level(r reply, building string) string {
	items, _ := r.body["items"].([]any)
	for _, it := range items {
		m, _ := it.(map[string]any)
		if m["id"] == building {
			s, _ := m["level"].(string)
			return s
		}
	}
	return "-"
}

// The whole story of L-A2 on the real stack: the owner adds a receptionist, the receptionist signs in with the
// one-time PIN and must change it, building access is stored, and revoking it applies on the next request.
func TestStaffLifecycle_SG1101_SG501(t *testing.T) {
	e := newEnv(t)
	e.seedOwner()
	e.seedStayTenant(staffTenant, 1)
	ow := e.signIn("owner1", ownerPIN)
	if ow.status != 200 {
		t.Fatalf("owner sign-in: %d %v", ow.status, ow.body)
	}
	owner := ow.str("accessToken")

	key := newKey()
	st, raw := e.send("POST", "/v1/owner/staff", owner, key, staffBody())
	created := parse(raw)
	pin, _ := created["oneTimePin"].(map[string]any)["pin"].(string)
	id := created["staff"].(map[string]any)["id"].(string)
	if st != 201 || len(pin) != 6 || id == "" {
		t.Fatalf("create staff: %d %s", st, raw)
	}
	// A replay returns the staff member and no PIN; the PIN is nowhere in the database or the logs in clear.
	st, raw = e.send("POST", "/v1/owner/staff", owner, key, staffBody())
	if st != 201 || parse(raw)["oneTimePin"] != nil || parse(raw)["staff"].(map[string]any)["id"] != id {
		t.Fatalf("replay: %d %s", st, raw)
	}
	for _, table := range []string{"idempotency_keys", "audit_logs", "pin_credentials", "users", "staff_profiles"} {
		if strings.Contains(e.dumpTable(table), pin) {
			t.Errorf("one-time PIN found in %s", table)
		}
	}
	if strings.Contains(e.logs.String(), pin) || strings.Contains(e.logs.String(), ownerPIN) {
		t.Error("a PIN reached the logs")
	}

	// First sign-in: only changeMyPin until the PIN is replaced.
	in := e.signIn("linh", pin)
	if in.status != 200 || in.body["mustChangePin"] != true {
		t.Fatalf("staff sign-in: %d %v", in.status, in.body)
	}
	staff := in.str("accessToken")
	if r := e.call("GET", "/v1/buildings", staff, nil); r.status != 403 || r.str("code") != "PIN_CHANGE_REQUIRED" {
		t.Fatalf("limited session: %d %v", r.status, r.body)
	}
	if r := e.call("PUT", "/v1/me/pin", staff, map[string]any{"currentPin": pin, "newPin": "123456"}); r.status != 422 || r.str("code") != "PIN_TOO_SIMPLE" {
		t.Fatalf("run: %d %v", r.status, r.body)
	}
	if r := e.call("PUT", "/v1/me/pin", staff, map[string]any{"currentPin": pin, "newPin": "260814"}); r.status != 204 {
		t.Fatalf("change pin: %d %v", r.status, r.body)
	}
	if r := e.signIn("linh", pin); r.status != 401 || r.str("code") != "PIN_INVALID" {
		t.Fatalf("old PIN: %d %v", r.status, r.body)
	}

	// Stored building access: EDIT lets the receptionist check in, VIEW does not, NONE hides the building,
	// each change applying to the same token on the next request.
	setLevel := func(l string) {
		t.Helper()
		if r := e.call("PUT", "/v1/owner/staff/"+id+"/building-permissions/"+stayBuilding, owner, map[string]any{"level": l}); r.status != 200 || r.str("level") != l {
			t.Fatalf("set %s: %d %v", l, r.status, r.body)
		}
	}
	if got := level(e.call("GET", "/v1/buildings", staff, nil), stayBuilding); got != "EDIT" {
		t.Fatalf("EDIT level, got %s", got)
	}
	setLevel("VIEW")
	if got := level(e.call("GET", "/v1/buildings", staff, nil), stayBuilding); got != "VIEW" {
		t.Fatalf("VIEW level, got %s", got)
	}
	if st, raw = e.checkIn(staff, 1, newKey(), stayBody(nil)); st != 403 || parse(raw)["code"] != "BUILDING_FORBIDDEN" {
		t.Fatalf("check-in with VIEW: %d %s", st, raw)
	}
	setLevel("EDIT")
	if st, raw = e.checkIn(staff, 1, newKey(), stayBody(nil)); st != 201 {
		t.Fatalf("check-in with EDIT: %d %s", st, raw)
	}
	setLevel("NONE")
	if got := level(e.call("GET", "/v1/buildings", staff, nil), stayBuilding); got != "-" {
		t.Fatalf("revoked building must disappear, got %s", got)
	}
	if n := e.count(`SELECT count(*) FROM app.audit_logs WHERE action = 'BUILDING_PERMISSION_SET' AND entity_id = $1`, id); n != 3 {
		t.Fatalf("permission audit rows = %d", n)
	}

	// Roles: the receptionist cannot manage staff.
	if r := e.call("GET", "/v1/owner/staff", staff, nil); r.status != 403 || r.str("code") != "ROLE_FORBIDDEN" {
		t.Fatalf("receptionist lists staff: %d %v", r.status, r.body)
	}
	// Staff list shows the new person; the owner cannot give themselves a building row.
	if r := e.call("GET", "/v1/owner/staff", owner, nil); r.status != 200 || len(r.body["items"].([]any)) != 1 {
		t.Fatalf("list: %d %v", r.status, r.body)
	}
	if r := e.call("PUT", "/v1/owner/staff/us_owner/building-permissions/"+stayBuilding, owner, map[string]any{"level": "NONE"}); r.status != 422 {
		t.Fatalf("owner row: %d %v", r.status, r.body)
	}

	// Lock ends the session at once and unlock restores sign-in; the PIN reset issues a new one-time PIN.
	if r := e.call("PATCH", "/v1/owner/staff/"+id, owner, map[string]any{"phone": "0901234567"}); r.status != 200 {
		t.Fatalf("update staff: %d %v", r.status, r.body)
	}
	if r := e.call("POST", "/v1/owner/staff/"+id+"/lock", owner, nil); r.status != 200 || r.str("status") != "LOCKED" {
		t.Fatalf("lock: %d %v", r.status, r.body)
	}
	if r := e.call("GET", "/v1/me", staff, nil); r.status != 401 {
		t.Fatalf("locked session: %d", r.status)
	}
	if r := e.signIn("linh", "260814"); r.status != 401 {
		t.Fatalf("locked sign-in: %d", r.status)
	}
	if r := e.call("POST", "/v1/owner/staff/"+id+"/unlock", owner, nil); r.status != 200 || r.str("status") != "ACTIVE" {
		t.Fatalf("unlock: %d %v", r.status, r.body)
	}
	if st, raw = e.send("POST", "/v1/owner/staff/"+id+"/pin-reset", owner, newKey(), nil); st != 200 || len(parse(raw)["pin"].(string)) != 6 {
		t.Fatalf("reset: %d %s", st, raw)
	}

	// Removing needs the owner PIN, and an open shift must be closed first (docs/15 rule 13).
	if r := e.call("POST", "/v1/owner/staff/"+id+"/remove", owner, map[string]any{"ownerPin": ownerPIN}); r.status != 409 || r.str("code") != "SHIFT_OPEN" {
		t.Fatalf("remove with an open shift: %d %v", r.status, r.body)
	}
	// The person's sessions ended with the PIN reset, so the shift is closed directly (the close use case is tested in shifts_e2e_test.go).
	e.exec(`UPDATE app.shifts SET status = 'CLOSED', closed_at = now(), expected_cash = 0, counted_cash = 0, difference = 0, float_left = 0
		WHERE user_id = $1 AND status = 'OPEN'`, id)
	// Afterwards the person cannot sign in.
	if r := e.call("POST", "/v1/owner/staff/"+id+"/remove", owner, map[string]any{"ownerPin": "159357"}); r.status != 403 || r.str("code") != "OWNER_PIN_INVALID" {
		t.Fatalf("wrong owner PIN: %d %v", r.status, r.body)
	}
	if r := e.call("POST", "/v1/owner/staff/"+id+"/remove", owner, map[string]any{"ownerPin": ownerPIN}); r.status != 204 {
		t.Fatalf("remove: %d %v", r.status, r.body)
	}
	if r := e.signIn("linh", "260814"); r.status != 401 {
		t.Fatalf("removed sign-in: %d", r.status)
	}
	if n := e.count(`SELECT count(*) FROM app.users WHERE id = $1 AND status = 'REMOVED'`, id); n != 1 {
		t.Fatal("history must be kept: the user row stays")
	}
}

func TestSignInLockout_SG701_AC2_E2E(t *testing.T) {
	e := newEnv(t)
	e.seedOwner()
	var last reply
	for i := 0; i < 5; i++ {
		last = e.signIn("owner1", "159357")
	}
	if last.status != 401 || last.str("code") != "ACCOUNT_LOCKED" || last.str("lockedUntil") == "" {
		t.Fatalf("fifth wrong PIN: %d %v", last.status, last.body)
	}
	if r := e.signIn("owner1", ownerPIN); r.str("code") != "ACCOUNT_LOCKED" {
		t.Fatalf("right PIN while locked: %d %v", r.status, r.body)
	}
	if n := e.count(`SELECT count(*) FROM app.audit_logs WHERE action = 'ACCOUNT_LOCKED' AND tenant_id = $1`, staffTenant); n != 1 {
		t.Fatalf("ACCOUNT_LOCKED audit rows = %d", n)
	}
	// Same answer for an unknown guesthouse, user or PIN.
	for _, body := range []map[string]any{
		{"guesthouseCode": "nowhere", "username": "owner1", "pin": ownerPIN},
		{"guesthouseCode": staffCode, "username": "ghost", "pin": ownerPIN},
	} {
		if r := e.call("POST", "/v1/auth/sign-in", "", body); r.status != 401 || r.str("code") != "PIN_INVALID" {
			t.Fatalf("%v: %d %v", body, r.status, r.body)
		}
	}
}
