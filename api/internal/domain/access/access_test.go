package access

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"
)

// want is a compact, independent restatement of docs/04 section 2.3 and docs/15 section 2.
// roles: O owner, M manager, R receptionist, H housekeeping. need: "-" no building check, "V" VIEW, "E" EDIT.
var want = map[string]struct{ roles, need string }{
	"getMe": {"MORH", "-"}, "setMyLocale": {"MORH", "-"}, "signOut": {"MORH", "-"}, "changeMyPin": {"MORH", "-"},
	"listBuildings": {"MORH", "-"}, "listRooms": {"MORH", "V"}, "getRoom": {"MORH", "V"}, "listServices": {"MOR", "-"},
	"createStay": {"MOR", "E"}, "addStayExtras": {"MOR", "E"}, "checkoutStay": {"MOR", "E"}, "getStay": {"MOR", "V"},
	"createPayment": {"MOR", "E"}, "simulatePaymentReceived": {"MOR", "E"}, "getPayment": {"MOR", "V"}, "streamPaymentEvents": {"MOR", "V"},
	"listHousekeepingTasks": {"MORH", "-"},
	// ponytail: receptionist and manager may clean too (docs/15 rule 6); L-B4 widens this operation.
	"completeHousekeepingTask": {"MORH", "E"},
	"reportRoomUsage":          {"MORH", "E"}, "reportDamage": {"MORH", "E"},
	"getCurrentShift": {"MOR", "E"}, "closeShift": {"MOR", "E"}, "recordCashPayout": {"MOR", "E"},
	"editCheckInTime": {"MOR", "E"}, "moveStay": {"MOR", "E"}, "listStays": {"MOR", "-"}, "getReceipt": {"MOR", "V"},
	"setGuestIdNumber": {"MOR", "E"}, "uploadGuestIdPhoto": {"MOR", "E"}, "createStocktake": {"MOR", "E"},

	// Own data of any signed-in person.
	"getMyRoster": {"MORH", "-"}, "listMyLeaveRequests": {"MORH", "-"}, "createLeaveRequest": {"MORH", "-"}, "cancelMyLeave": {"MORH", "-"},

	// Owner or manager.
	"resetStaffPin": {"OM", "-"}, "lockStaff": {"OM", "-"}, "unlockStaff": {"OM", "-"}, "getRoster": {"OM", "-"}, "putRoster": {"OM", "-"},
	"copyRosterWeek": {"OM", "-"}, "listLeaveRequests": {"OM", "-"}, "approveLeave": {"OM", "-"}, "declineLeave": {"OM", "-"},
	"listTickets": {"OM", "-"}, "getTicket": {"OM", "-"}, "updateTicket": {"OM", "-"}, "getProperty": {"OM", "-"}, "getSepayStatus": {"OM", "-"},
	"updateRoom": {"OM", "-"}, "listRatePlans": {"OM", "-"}, "previewPrice": {"OM", "-"}, "createService": {"OM", "-"}, "updateService": {"OM", "-"},
	"restockService": {"OM", "-"}, "listStockMovements": {"OM", "-"}, "getStayTimeline": {"OM", "-"}, "listTransactions": {"OM", "-"},
	"listAlerts": {"OM", "-"}, "markAlertRead": {"OM", "-"}, "listClosedShifts": {"OM", "-"}, "getGuestIdRecord": {"OM", "-"},
	"revealGuestIdNumber": {"OM", "-"}, "getGuestIdPhoto": {"OM", "-"}, "deleteGuestIdPhoto": {"OM", "-"}, "deleteGuestIdNumber": {"OM", "-"},

	// Owner only: bank accounts, removing staff, pay, expense amounts, linking transfers (docs/15 section 2).
	"getOwnerOverview": {"O", "-"}, "listStaffPermissions": {"O", "-"}, "setBuildingPermission": {"O", "-"}, "getShiftReview": {"O", "-"},
	"listStaff": {"O", "-"}, "createStaff": {"O", "-"}, "updateStaff": {"O", "-"}, "removeStaff": {"O", "-"},
	"getPayroll": {"O", "-"}, "updatePayrollLine": {"O", "-"}, "markPayrollPaid": {"O", "-"},
	"getExpenseMonth": {"O", "-"}, "createExpense": {"O", "-"}, "updateExpense": {"O", "-"}, "deleteExpense": {"O", "-"}, "getIncomeCostReport": {"O", "-"},
	"updateProperty": {"O", "-"}, "listBankAccounts": {"O", "-"}, "createBankAccount": {"O", "-"}, "makeDefaultBankAccount": {"O", "-"},
	"removeBankAccount": {"O", "-"}, "createBuilding": {"O", "-"}, "updateBuilding": {"O", "-"}, "createFloor": {"O", "-"}, "createRooms": {"O", "-"},
	"updateRatePlan": {"O", "-"}, "removeService": {"O", "-"}, "linkTransferToInvoice": {"O", "-"}, "listInvoices": {"O", "-"}, "listAuditLogs": {"O", "-"},
}

var public = []string{"createDemoSession", "getHealth", "getReadiness", "receiveBankWebhook", "receiveBankWebhookLegacy", "signIn"}

var roleLetter = map[Role]string{RoleOwner: "O", RoleManager: "M", RoleReceptionist: "R", RoleHousekeeping: "H"}

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

// contractAccess reads the x-access of every operation of the contract.
func contractAccess(t *testing.T) map[string]string {
	t.Helper()
	raw, err := os.ReadFile("../../../../contracts/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	idx := regexp.MustCompile(`(?m)^\s*operationId:\s*(\S+)`).FindAllStringSubmatchIndex(text, -1)
	out := map[string]string{}
	for n, m := range idx {
		end := len(text)
		if n+1 < len(idx) {
			end = idx[n+1][0]
		}
		a := regexp.MustCompile(`(?m)^\s*x-access:\s*([A-Z_]+)`).FindStringSubmatch(text[m[1]:end])
		out[text[m[2]:m[3]]] = ""
		if a != nil {
			out[text[m[2]:m[3]]] = a[1]
		}
	}
	return out
}

// Every operation of the contract is in the table once, and its x-access agrees with the roles in the table:
// the table cannot drift from the contract, and a new operation cannot ship without a decision.
func TestAuthorizer_SG1101_AC2_TableMatchesContract(t *testing.T) {
	classes := map[string]string{"OWNER": "O", "OWNER_OR_MANAGER": "OM", "ANY": "MORH", "ANY_STAFF": "MORH"}
	for op, xa := range contractAccess(t) {
		w, inTable := want[op]
		isPublic := false
		for _, p := range public {
			isPublic = isPublic || p == op
		}
		switch {
		case isPublic && inTable:
			t.Errorf("%s is public and in the table", op)
		case !isPublic && !inTable:
			t.Errorf("%s is neither public nor in the role table", op)
		}
		if exp, ok := classes[xa]; ok && inTable && w.roles != exp {
			t.Errorf("%s: x-access %s wants roles %s, table has %s", op, xa, exp, w.roles)
		}
		if (xa == "EDIT" || xa == "EDIT_ANY") && inTable && w.need != "E" {
			t.Errorf("%s: x-access %s needs EDIT, table has %s", op, xa, w.need)
		}
		if xa == "VIEW" && inTable && w.need == "E" {
			t.Errorf("%s: x-access VIEW but the table needs EDIT", op)
		}
	}
	for op := range want {
		if _, ok := contractAccess(t)[op]; !ok {
			t.Errorf("table row %s is not in the contract", op)
		}
	}
}

// MANAGER keeps the exclusions of docs/15 section 2: bank accounts, removing staff, pay, expense amounts and linking transfers.
func TestAuthorizer_SG1101_AC2_ManagerExclusions(t *testing.T) {
	for _, op := range []string{"listBankAccounts", "createBankAccount", "makeDefaultBankAccount", "removeBankAccount", "removeStaff",
		"getPayroll", "updatePayrollLine", "markPayrollPaid", "getExpenseMonth", "createExpense", "updateExpense", "deleteExpense",
		"linkTransferToInvoice", "updateStaff", "createStaff"} {
		if err := (Authorizer{}).Check(op, RoleManager, EDIT); !errors.Is(err, ErrRoleForbidden) {
			t.Errorf("%s: manager got %v, want ROLE_FORBIDDEN", op, err)
		}
	}
	for _, op := range []string{"resetStaffPin", "lockStaff", "getRoster", "updateRoom", "restockService", "listAlerts"} {
		if err := (Authorizer{}).Check(op, RoleManager, NONE); err != nil {
			t.Errorf("%s: manager got %v, want allowed", op, err)
		}
	}
}
