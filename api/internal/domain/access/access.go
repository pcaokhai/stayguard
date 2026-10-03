// Package access holds the pure authorization rules of docs/04 section 2.3.
package access

import (
	"errors"
	"fmt"
)

// Role is a signed-in user's role.
type Role string

const (
	RoleOwner        Role = "OWNER"
	RoleManager      Role = "MANAGER"
	RoleReceptionist Role = "RECEPTIONIST"
	RoleHousekeeping Role = "HOUSEKEEPING"
)

// ParseRole fails closed on unknown values.
func ParseRole(s string) (Role, error) {
	switch r := Role(s); r {
	case RoleOwner, RoleManager, RoleReceptionist, RoleHousekeeping:
		return r, nil
	}
	return "", fmt.Errorf("unknown role %q", s)
}

// Level is building access; the order NONE < VIEW < EDIT is meaningful.
type Level int

const (
	NONE Level = iota
	VIEW
	EDIT
)

// ParseLevel fails closed on unknown values.
func ParseLevel(s string) (Level, error) {
	switch s {
	case "NONE":
		return NONE, nil
	case "VIEW":
		return VIEW, nil
	case "EDIT":
		return EDIT, nil
	}
	return NONE, fmt.Errorf("unknown access level %q", s)
}

// Valid reports whether l is one of NONE, VIEW and EDIT.
func (l Level) Valid() bool { return l >= NONE && l <= EDIT }

func (l Level) String() string {
	switch l {
	case VIEW:
		return "VIEW"
	case EDIT:
		return "EDIT"
	}
	return "NONE"
}

// EffectiveLevel gives the owner implicit EDIT on every building (docs/04 section 2.3).
func EffectiveLevel(role Role, stored Level) Level {
	if role == RoleOwner {
		return EDIT
	}
	return stored
}

// Scope says which building level, if any, the caller must pass to Check.
type Scope int

const (
	// ScopeNone: no building check (role only).
	ScopeNone Scope = iota
	// ScopeBuilding: level of the building being touched.
	ScopeBuilding
	// ScopeAnyEditable: caller passes the maximum level across all buildings.
	ScopeAnyEditable
	// ScopeFilter: role check only; callers filter results by the level per building.
	ScopeFilter
	// ScopeOwnerOnly: owner role, no building check.
	ScopeOwnerOnly
	// ScopePublic: no session (health, demo session, signed webhook).
	ScopePublic
)

type rule struct {
	roles []Role
	min   Level
	scope Scope
}

var (
	ErrRoleForbidden     = errors.New("role forbidden")
	ErrBuildingForbidden = errors.New("building forbidden")
	ErrUnknownOperation  = errors.New("unknown operation")
)

var (
	anyRole      = []Role{RoleOwner, RoleManager, RoleReceptionist, RoleHousekeeping}
	front        = []Role{RoleOwner, RoleManager, RoleReceptionist}
	owner        = []Role{RoleOwner}
	ownerManager = []Role{RoleOwner, RoleManager}
)

// rules is keyed by OpenAPI operationId; a test keeps it in step with contracts/openapi.yaml.
var rules = map[string]rule{
	"createDemoSession": {scope: ScopePublic},
	"getHealth":         {scope: ScopePublic},
	"getReadiness":      {scope: ScopePublic},
	// The bank webhook is authenticated by signature, not by session.
	"receiveBankWebhook": {scope: ScopePublic},

	"getMe":       {roles: anyRole, scope: ScopeNone},
	"setMyLocale": {roles: anyRole, scope: ScopeNone},

	"listBuildings":         {roles: anyRole, scope: ScopeFilter},
	"listHousekeepingTasks": {roles: anyRole, scope: ScopeFilter},

	"listRooms": {roles: anyRole, min: VIEW, scope: ScopeBuilding},
	"getRoom":   {roles: anyRole, min: VIEW, scope: ScopeBuilding},

	"listServices": {roles: front, scope: ScopeNone},

	"createStay":              {roles: front, min: EDIT, scope: ScopeBuilding},
	"addStayExtras":           {roles: front, min: EDIT, scope: ScopeBuilding},
	"checkoutStay":            {roles: front, min: EDIT, scope: ScopeBuilding},
	"getStay":                 {roles: front, min: VIEW, scope: ScopeBuilding},
	"createPayment":           {roles: front, min: EDIT, scope: ScopeBuilding},
	"simulatePaymentReceived": {roles: front, min: EDIT, scope: ScopeBuilding},
	"getPayment":              {roles: front, min: VIEW, scope: ScopeBuilding},
	"streamPaymentEvents":     {roles: front, min: VIEW, scope: ScopeBuilding},

	"completeHousekeepingTask": {roles: anyRole, min: EDIT, scope: ScopeBuilding},
	"reportRoomUsage":          {roles: anyRole, min: EDIT, scope: ScopeBuilding},

	"getCurrentShift":  {roles: front, min: EDIT, scope: ScopeAnyEditable},
	"closeShift":       {roles: front, min: EDIT, scope: ScopeAnyEditable},
	"recordCashPayout": {roles: front, min: EDIT, scope: ScopeAnyEditable},

	"getOwnerOverview":      {roles: owner, scope: ScopeOwnerOnly},
	"listStaffPermissions":  {roles: owner, scope: ScopeOwnerOnly},
	"setBuildingPermission": {roles: owner, scope: ScopeOwnerOnly},
	"getShiftReview":        {roles: owner, scope: ScopeOwnerOnly},

	// Contract 1.1.0 operations, from x-access: OWNER, OWNER_OR_MANAGER (role only), ANY and ANY_STAFF (own data), then building rules.
	"listStaff":                {roles: owner, scope: ScopeOwnerOnly},
	"createStaff":              {roles: owner, scope: ScopeOwnerOnly},
	"updateStaff":              {roles: owner, scope: ScopeOwnerOnly},
	"removeStaff":              {roles: owner, scope: ScopeOwnerOnly},
	"getPayroll":               {roles: owner, scope: ScopeOwnerOnly},
	"updatePayrollLine":        {roles: owner, scope: ScopeOwnerOnly},
	"markPayrollPaid":          {roles: owner, scope: ScopeOwnerOnly},
	"getExpenseMonth":          {roles: owner, scope: ScopeOwnerOnly},
	"createExpense":            {roles: owner, scope: ScopeOwnerOnly},
	"updateExpense":            {roles: owner, scope: ScopeOwnerOnly},
	"deleteExpense":            {roles: owner, scope: ScopeOwnerOnly},
	"getIncomeCostReport":      {roles: owner, scope: ScopeOwnerOnly},
	"updateProperty":           {roles: owner, scope: ScopeOwnerOnly},
	"listBankAccounts":         {roles: owner, scope: ScopeOwnerOnly},
	"createBankAccount":        {roles: owner, scope: ScopeOwnerOnly},
	"makeDefaultBankAccount":   {roles: owner, scope: ScopeOwnerOnly},
	"removeBankAccount":        {roles: owner, scope: ScopeOwnerOnly},
	"createBuilding":           {roles: owner, scope: ScopeOwnerOnly},
	"updateBuilding":           {roles: owner, scope: ScopeOwnerOnly},
	"createFloor":              {roles: owner, scope: ScopeOwnerOnly},
	"createRooms":              {roles: owner, scope: ScopeOwnerOnly},
	"updateRatePlan":           {roles: owner, scope: ScopeOwnerOnly},
	"removeService":            {roles: owner, scope: ScopeOwnerOnly},
	"listInvoices":             {roles: owner, scope: ScopeOwnerOnly},
	"linkTransferToInvoice":    {roles: owner, scope: ScopeOwnerOnly},
	"dismissUnmatchedTransfer": {roles: owner, scope: ScopeOwnerOnly},
	"listAuditLogs":            {roles: owner, scope: ScopeOwnerOnly},
	"resetStaffPin":            {roles: ownerManager, scope: ScopeNone},
	"lockStaff":                {roles: ownerManager, scope: ScopeNone},
	"unlockStaff":              {roles: ownerManager, scope: ScopeNone},
	"getRoster":                {roles: ownerManager, scope: ScopeNone},
	"putRoster":                {roles: ownerManager, scope: ScopeNone},
	"copyRosterWeek":           {roles: ownerManager, scope: ScopeNone},
	"listLeaveRequests":        {roles: ownerManager, scope: ScopeNone},
	"approveLeave":             {roles: ownerManager, scope: ScopeNone},
	"declineLeave":             {roles: ownerManager, scope: ScopeNone},
	"listTickets":              {roles: ownerManager, scope: ScopeNone},
	"getTicket":                {roles: ownerManager, scope: ScopeNone},
	"updateTicket":             {roles: ownerManager, scope: ScopeNone},
	"getProperty":              {roles: ownerManager, scope: ScopeNone},
	"getSepayStatus":           {roles: ownerManager, scope: ScopeNone},
	"updateRoom":               {roles: ownerManager, scope: ScopeNone},
	"listRatePlans":            {roles: ownerManager, scope: ScopeNone},
	"previewPrice":             {roles: ownerManager, scope: ScopeNone},
	"createService":            {roles: ownerManager, scope: ScopeNone},
	"updateService":            {roles: ownerManager, scope: ScopeNone},
	"restockService":           {roles: ownerManager, scope: ScopeNone},
	"listStockMovements":       {roles: ownerManager, scope: ScopeNone},
	"getStayTimeline":          {roles: ownerManager, scope: ScopeNone},
	"listTransactions":         {roles: ownerManager, scope: ScopeNone},
	"listAlerts":               {roles: ownerManager, scope: ScopeNone},
	"markAlertRead":            {roles: ownerManager, scope: ScopeNone},
	"listClosedShifts":         {roles: ownerManager, scope: ScopeNone},
	"getGuestIdRecord":         {roles: ownerManager, scope: ScopeNone},
	"revealGuestIdNumber":      {roles: ownerManager, scope: ScopeNone},
	"getGuestIdPhoto":          {roles: ownerManager, scope: ScopeNone},
	"deleteGuestIdPhoto":       {roles: ownerManager, scope: ScopeNone},
	"deleteGuestIdNumber":      {roles: ownerManager, scope: ScopeNone},
	"getMyRoster":              {roles: anyRole, scope: ScopeNone},
	"listMyLeaveRequests":      {roles: anyRole, scope: ScopeNone},
	"createLeaveRequest":       {roles: anyRole, scope: ScopeNone},
	"cancelMyLeave":            {roles: anyRole, scope: ScopeNone},
	"signOut":                  {roles: anyRole, scope: ScopeNone},
	"changeMyPin":              {roles: anyRole, scope: ScopeNone},
	"reportDamage":             {roles: anyRole, min: EDIT, scope: ScopeBuilding},
	"editCheckInTime":          {roles: front, min: EDIT, scope: ScopeBuilding},
	"moveStay":                 {roles: front, min: EDIT, scope: ScopeBuilding},
	"setGuestIdNumber":         {roles: front, min: EDIT, scope: ScopeBuilding},
	"uploadGuestIdPhoto":       {roles: front, min: EDIT, scope: ScopeBuilding},
	"getReceipt":               {roles: front, min: VIEW, scope: ScopeBuilding},
	"listStays":                {roles: front, scope: ScopeFilter},
	"createStocktake":          {roles: front, min: EDIT, scope: ScopeAnyEditable},
	"signIn":                   {scope: ScopePublic},
	"receiveBankWebhookLegacy": {scope: ScopePublic},
}

// Authorizer applies the rule table. It holds no state.
type Authorizer struct{}

// Check returns nil, ErrRoleForbidden, ErrBuildingForbidden or ErrUnknownOperation.
// level must already be the effective level (see EffectiveLevel); for ScopeAnyEditable it is the
// maximum across buildings. Role is checked first, then the building level.
func (Authorizer) Check(op string, role Role, level Level) error {
	r, ok := rules[op]
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownOperation, op)
	}
	if r.scope == ScopePublic {
		return nil
	}
	if !hasRole(r.roles, role) {
		return fmt.Errorf("%w: %s on %s", ErrRoleForbidden, role, op)
	}
	if r.scope != ScopeBuilding && r.scope != ScopeAnyEditable {
		return nil
	}
	if !level.Valid() || level < r.min {
		return fmt.Errorf("%w: %s", ErrBuildingForbidden, op)
	}
	return nil
}

func hasRole(allowed []Role, role Role) bool {
	for _, a := range allowed {
		if a == role {
			return true
		}
	}
	return false
}
