package access

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"
)

// want is a compact, independent restatement of docs/04 section 2.3.
// roles: O owner, R receptionist, H housekeeping. need: "-" no building check, "V" VIEW, "E" EDIT.
var want = map[string]struct{ roles, need string }{
	"getMe":                    {"ORH", "-"},
	"setMyLocale":              {"ORH", "-"},
	"listBuildings":            {"ORH", "-"},
	"listRooms":                {"ORH", "V"},
	"getRoom":                  {"ORH", "V"},
	"listServices":             {"OR", "-"},
	"createStay":               {"OR", "E"},
	"addStayExtras":            {"OR", "E"},
	"checkoutStay":             {"OR", "E"},
	"getStay":                  {"OR", "V"},
	"createPayment":            {"OR", "E"},
	"simulatePaymentReceived":  {"OR", "E"},
	"getPayment":               {"OR", "V"},
	"streamPaymentEvents":      {"OR", "V"},
	"listHousekeepingTasks":    {"ORH", "-"},
	"completeHousekeepingTask": {"OH", "E"},
	"reportRoomUsage":          {"ORH", "E"},
	"getCurrentShift":          {"OR", "E"},
	"closeShift":               {"OR", "E"},
	"recordCashPayout":         {"OR", "E"},
	"getOwnerOverview":         {"O", "-"},
	"listStaffPermissions":     {"O", "-"},
	"setBuildingPermission":    {"O", "-"},
	"getShiftReview":           {"O", "-"},
}

var public = []string{"createDemoSession", "getHealth", "getReadiness", "receiveBankWebhook"}

var roleLetter = map[Role]string{RoleOwner: "O", RoleReceptionist: "R", RoleHousekeeping: "H"}

func TestAuthorizer_SG102_AC5_Matrix(t *testing.T) {
	a := Authorizer{}
	for op, w := range want {
		for role, letter := range roleLetter {
			for level := NONE; level <= EDIT; level++ {
				var exp error
				switch {
				case !strings.Contains(w.roles, letter):
					exp = ErrRoleForbidden
				case w.need == "V" && level < VIEW, w.need == "E" && level < EDIT:
					exp = ErrBuildingForbidden
				}
				if got := a.Check(op, role, level); !errors.Is(got, exp) {
					t.Errorf("%s role=%s level=%d: got %v want %v", op, role, level, got, exp)
				}
			}
		}
	}
}

func TestAuthorizer_SG102_AC5_PublicOps(t *testing.T) {
	for _, op := range public {
		if err := (Authorizer{}).Check(op, Role(""), NONE); err != nil {
			t.Errorf("%s: %v", op, err)
		}
	}
}

func TestAuthorizer_SG102_AC5_FailClosed(t *testing.T) {
	a := Authorizer{}
	if err := a.Check("nope", RoleOwner, EDIT); !errors.Is(err, ErrUnknownOperation) {
		t.Errorf("unknown op: %v", err)
	}
	if err := a.Check("getMe", Role("ADMIN"), EDIT); !errors.Is(err, ErrRoleForbidden) {
		t.Errorf("unknown role: %v", err)
	}
	for _, lv := range []Level{-1, 3, 99} {
		if err := a.Check("createStay", RoleOwner, lv); !errors.Is(err, ErrBuildingForbidden) {
			t.Errorf("level %d: %v", lv, err)
		}
	}
	if _, err := ParseRole("ADMIN"); err == nil {
		t.Error("ParseRole accepted unknown")
	}
	if _, err := ParseLevel("ROOT"); err == nil {
		t.Error("ParseLevel accepted unknown")
	}
	if r, err := ParseRole("OWNER"); err != nil || r != RoleOwner {
		t.Errorf("ParseRole OWNER: %v %v", r, err)
	}
	if l, err := ParseLevel("VIEW"); err != nil || l != VIEW {
		t.Errorf("ParseLevel VIEW: %v %v", l, err)
	}
}

func TestAuthorizer_SG102_AC5_OwnerImplicitEdit(t *testing.T) {
	for _, stored := range []Level{NONE, VIEW, EDIT} {
		if got := EffectiveLevel(RoleOwner, stored); got != EDIT {
			t.Errorf("owner stored=%d: %d", stored, got)
		}
		if got := EffectiveLevel(RoleReceptionist, stored); got != stored {
			t.Errorf("receptionist stored=%d: %d", stored, got)
		}
	}
}

func TestAuthorizer_SG102_AC5_ContractParity(t *testing.T) {
	raw, err := os.ReadFile("../../../../contracts/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	inContract := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^\s*operationId:\s*(\S+)`).FindAllStringSubmatch(string(raw), -1) {
		inContract[m[1]] = true
		if _, ok := rules[m[1]]; !ok {
			t.Errorf("operationId %s in contract but not in rule table", m[1])
		}
	}
	for op := range rules {
		if !inContract[op] {
			t.Errorf("rule %s not in contract", op)
		}
	}
}
