//go:build integration

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/platform/config"
)

const (
	importAccountNo = "0011223344"
	sepaySecret     = "whsec_Example-Secret-123"
)

const importJSON = `{
  "guesthouseCode": "riverside",
  "name": "Riverside Guesthouse",
  "property": {"name": "Riverside", "address": "1 River Rd", "phone": "0900000000"},
  "bankAccount": {"bankBin": "970436", "accountNo": "` + importAccountNo + `", "accountName": "RIVERSIDE"},
  "unitTypes": [
    {"code": "STANDARD", "name": {"vi": "Thuong", "en": "Standard"}, "ratePlan": {"version": 1, "currency": "VND", "graceMinutes": 15,
      "hourly": {"firstHour": 80000, "extraHour": 20000}, "overnight": {"price": 200000, "windowStart": "21:00", "windowEnd": "12:00"},
      "daily": {"price": 300000, "windowStart": "14:00", "windowEnd": "12:00"}}}
  ],
  "buildings": [{"code": "A", "name": "Building A", "floors": 2, "roomsPerFloor": 3}],
  "services": [{"code": "WATER", "name": {"vi": "Nuoc", "en": "Water"}, "price": 10000, "stock": 20}],
  "owner": {"name": "Mai", "username": "mai"},
  "staff": [
    {"name": "Linh", "username": "linh", "position": "FRONT_DESK", "appAccess": "RECEPTIONIST",
     "contract": {"payType": "MONTHLY", "rate": 7000000, "fixedAllowance": 0, "standardShifts": 26, "startDate": "2026-10-01", "annualLeaveDays": 12},
     "buildingAccess": {"A": "EDIT"}},
    {"name": "Bao", "position": "SECURITY", "appAccess": "NONE",
     "contract": {"payType": "PER_SHIFT", "rate": 300000, "fixedAllowance": 0, "standardShifts": 20, "startDate": "2026-10-01", "annualLeaveDays": 12}}
  ]
}`

func (e *env) installer() *app.Installer {
	e.t.Helper()
	inst, err := newInstaller(config.Config{DataEncryptionKey: testDataKey}, e.pool, e.clock)
	if err != nil {
		e.t.Fatal(err)
	}
	return inst
}

func (e *env) cli(args []string, secret string) (string, error) {
	var out bytes.Buffer
	err := execInstaller(context.Background(), e.installer(), args, &out, func() (string, error) { return secret, nil })
	return out.String(), err
}

// Import, webhook address, secret and status follow docs/runbooks/sepay-handover.md; the secret is write-only
// and the receiving account becomes the QR default only once SePay is connected.
func TestInstallerCLI_ImportAndSepay_SG703(t *testing.T) {
	e := newEnv(t)
	file := filepath.Join(t.TempDir(), "import.json")
	if err := os.WriteFile(file, []byte(importJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := e.cli([]string{"tenant", "import", "--file", file}, "")
	if err != nil {
		t.Fatalf("import: %v\n%s", err, out)
	}
	pins := regexp.MustCompile(`(?m)^\s+(mai|linh)\s+(\d{6})\s`).FindAllStringSubmatch(out, -1)
	if len(pins) != 2 || strings.Contains(out, "bao") {
		t.Fatalf("one-time PINs for the owner and the receptionist only:\n%s", out)
	}
	if strings.Contains(out, importAccountNo) {
		t.Fatalf("the account number must not be printed:\n%s", out)
	}
	if n := e.count(`SELECT count(*) FROM app.tenants WHERE guesthouse_code = 'riverside'`); n != 1 {
		t.Fatal("tenant missing")
	}
	if n := e.count(`SELECT count(*) FROM app.units`); n != 6 {
		t.Fatalf("rooms: %d", n)
	}
	if n := e.count(`SELECT count(*) FROM app.audit_logs WHERE action = 'INSTALLER_TENANT_IMPORTED'`); n != 1 {
		t.Fatal("import audit entry missing")
	}
	// The imported owner signs in with the one-time PIN and must change it; the receptionist has building access A.
	owner := e.callAs("riverside", "mai", pins[0][2])
	if owner.status != 200 || owner.body["mustChangePin"] != true {
		t.Fatalf("owner sign-in: %d %v", owner.status, owner.body)
	}
	token := owner.str("accessToken")
	if r := e.call("PUT", "/v1/me/pin", token, map[string]any{"currentPin": pins[0][2], "newPin": "482915"}); r.status != 204 {
		t.Fatalf("change pin: %d %v", r.status, r.body)
	}
	if n := e.count(`SELECT count(*) FROM app.building_permissions p JOIN app.users u ON u.id = p.user_id WHERE u.username = 'linh' AND p.level = 'EDIT'`); n != 1 {
		t.Fatal("receptionist building access")
	}

	// Property settings.
	if r := e.call("GET", "/v1/owner/property", token, nil); r.status != 200 || r.str("guesthouseCode") != "riverside" || r.str("address") != "1 River Rd" || r.body["qrExpiryMinutes"] != float64(30) {
		t.Fatalf("property: %d %v", r.status, r.body)
	}
	if r := e.call("PATCH", "/v1/owner/property", token, map[string]any{"qrExpiryMinutes": 45}); r.status != 200 || r.body["qrExpiryMinutes"] != float64(45) {
		t.Fatalf("update property: %d %v", r.status, r.body)
	}
	if r := e.call("PATCH", "/v1/owner/property", token, map[string]any{"qrExpiryMinutes": 3}); r.status != 422 {
		t.Fatalf("range: %d", r.status)
	}

	// Before SePay is connected the QR has no account.
	if id := e.defaultAccount("riverside"); id != "" {
		t.Fatalf("a pending account must not be the QR account: %s", id)
	}
	list := e.call("GET", "/v1/owner/bank-accounts", token, nil)
	items, _ := list.body["items"].([]any)
	if list.status != 200 || len(items) != 1 || items[0].(map[string]any)["sepayStatus"] != "PENDING" || items[0].(map[string]any)["accountNoMasked"] != "******3344" {
		t.Fatalf("list: %d %v", list.status, list.body)
	}
	pending := items[0].(map[string]any)["id"].(string)
	if r := e.call("POST", "/v1/owner/bank-accounts/"+pending+"/make-default", token, map[string]any{"ownerPin": "482915"}); r.status != 409 {
		t.Fatalf("pending as default: %d %v", r.status, r.body)
	}
	if s := e.call("GET", "/v1/owner/sepay-status", token, nil); s.str("status") != "NOT_CONNECTED" {
		t.Fatalf("status: %v", s.body)
	}

	// Webhook address and secret.
	hook, err := e.cli([]string{"sepay", "webhook", "--tenant", "riverside", "--base-url", "https://stay.example.vn"}, "")
	if err != nil || !regexp.MustCompile(`^https://stay\.example\.vn/v1/webhooks/bank/[A-Za-z0-9_-]{20,}\n$`).MatchString(hook) {
		t.Fatalf("webhook: %q %v", hook, err)
	}
	if again, _ := e.cli([]string{"sepay", "webhook", "--tenant", "riverside", "--base-url", "https://stay.example.vn"}, ""); again != hook {
		t.Fatal("the address is stable")
	}
	setOut, err := e.cli([]string{"sepay", "set-secret", "--tenant", "riverside"}, sepaySecret)
	if err != nil || strings.Contains(setOut, sepaySecret) {
		t.Fatalf("set-secret: %q %v", setOut, err)
	}
	status, _ := e.cli([]string{"sepay", "status", "--tenant", "riverside"}, "")
	if !strings.Contains(status, "connection: CONNECTED") || !strings.Contains(status, "secret stored: true") || strings.Contains(status, sepaySecret) || strings.Contains(status, importAccountNo) {
		t.Fatalf("status:\n%s", status)
	}
	// The secret is in no clear text anywhere; the owner sees the installer's change.
	for _, table := range []string{"tenants", "audit_logs", "alerts", "bank_accounts"} {
		d := e.dumpTable(table)
		if strings.Contains(d, sepaySecret) || strings.Contains(d, importAccountNo) {
			t.Errorf("secret or account number in clear in %s", table)
		}
	}
	if strings.Contains(e.logs.String(), sepaySecret) {
		t.Error("secret in the logs")
	}
	if n := e.count(`SELECT count(*) FROM app.alerts WHERE kind = 'SEPAY_UPDATED'`); n != 1 {
		t.Fatalf("SEPAY_UPDATED alerts: %d", n)
	}
	if n := e.count(`SELECT count(*) FROM app.audit_logs WHERE action = 'INSTALLER_SEPAY_UPDATED' AND actor_id IS NULL`); n != 1 {
		t.Fatal("installer audit entry missing")
	}
	// Now the account is connected and the QR default.
	if id := e.defaultAccount("riverside"); id != pending {
		t.Fatalf("QR account = %q, want the connected default %q", id, pending)
	}
	if s := e.call("GET", "/v1/owner/sepay-status", token, nil); s.str("status") != "CONNECTED" {
		t.Fatalf("status after connect: %v", s.body)
	}

	// A second account stays PENDING; it cannot take the QR until connected; a request cannot choose the QR account.
	st, raw := e.send("POST", "/v1/owner/bank-accounts", token, newKey(), map[string]any{"ownerPin": "482915", "bankBin": "970415", "accountNo": "5566778899", "accountName": "OTHER"})
	if st != 201 || parse(raw)["sepayStatus"] != "PENDING" || parse(raw)["isDefault"] != false {
		t.Fatalf("second account: %d %s", st, raw)
	}
	if id := e.defaultAccount("riverside"); id != pending {
		t.Fatalf("QR account changed to %q", id)
	}
	if r := e.call("POST", "/v1/owner/bank-accounts/"+pending+"/remove", token, map[string]any{"ownerPin": "482915"}); r.status != 409 {
		t.Fatalf("remove default: %d", r.status)
	}
	if r := e.call("POST", "/v1/owner/bank-accounts/"+parse(raw)["id"].(string)+"/remove", token, map[string]any{"ownerPin": "159357"}); r.status != 403 || r.str("code") != "OWNER_PIN_INVALID" {
		t.Fatalf("remove with a wrong owner PIN: %d %v", r.status, r.body)
	}

	// Errors name the field, never the value; an unknown guesthouse is refused; a second import of the code conflicts.
	if _, err = e.cli([]string{"sepay", "status", "--tenant", "nowhere"}, ""); !errors.Is(err, app.ErrTenantUnknown) {
		t.Fatalf("unknown tenant: %v", err)
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	_ = os.WriteFile(bad, []byte(strings.Replace(importJSON, `"riverside"`, `"x"`, 1)), 0o600)
	if _, err = e.cli([]string{"tenant", "import", "--file", bad}, ""); err == nil || !strings.Contains(err.Error(), "guesthouseCode") || strings.Contains(err.Error(), importAccountNo) {
		t.Fatalf("bad code: %v", err)
	}
	if _, err = e.cli([]string{"tenant", "import", "--file", file}, ""); !errors.Is(err, app.ErrConflict) {
		t.Fatalf("second import: %v", err)
	}
	if r := e.call("POST", "/v1/webhooks/bank", "", map[string]any{"externalId": "x", "amount": 1, "content": "x", "receivedAt": "2026-10-02T00:00:00Z"}); r.status != 410 {
		t.Fatalf("legacy webhook: %d %v", r.status, r.body)
	}
}

func (e *env) callAs(code, user, pin string) reply {
	return e.call("POST", "/v1/auth/sign-in", "", map[string]any{"guesthouseCode": code, "username": user, "pin": pin})
}

// defaultAccount is the id the QR would pay ("" when none), read the way the payment use case reads it.
func (e *env) defaultAccount(code string) string {
	e.t.Helper()
	tenant, found, err := postgres.NewTenantResolver(e.pool).TenantByCode(context.Background(), code)
	if err != nil || !found {
		e.t.Fatalf("tenant %s: %v %v", code, found, err)
	}
	var id string
	err = postgres.NewUnitOfWork(e.pool).Do(context.Background(), tenant, func(ctx context.Context, tx app.Tx) error {
		var derr error
		id, _, derr = postgres.PaymentRepo{}.DefaultBankAccount(ctx, tx)
		return derr
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}
