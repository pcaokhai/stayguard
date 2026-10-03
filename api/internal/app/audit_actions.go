package app

// Audit action codes written by the setup, staff, bank and stock use cases. The other codes are constants next to the
// use case that writes them. Every code needs a category in audit_categories.go and an entry in contracts/audit-actions.json.
const (
	auditPropertyUpdated    = "PROPERTY_UPDATED"
	auditBankAccountAdded   = "BANK_ACCOUNT_ADDED"
	auditBankAccountDefault = "BANK_ACCOUNT_DEFAULT"
	auditBankAccountRemoved = "BANK_ACCOUNT_REMOVED"
	auditBuildingCreated    = "BUILDING_CREATED"
	auditBuildingRenamed    = "BUILDING_RENAMED"
	auditFloorCreated       = "FLOOR_CREATED"
	auditRoomsCreated       = "ROOMS_CREATED"
	auditRoomUpdated        = "ROOM_UPDATED"
	auditRatePlanUpdated    = "RATE_PLAN_UPDATED"
	auditServiceCreated     = "SERVICE_CREATED"
	auditServiceUpdated     = "SERVICE_UPDATED"
	auditServiceRemoved     = "SERVICE_REMOVED"
	auditStockIn            = "STOCK_IN"
	auditStocktakeCreated   = "STOCKTAKE_CREATED"
	auditStaffCreated       = "STAFF_CREATED"
	auditStaffUpdated       = "STAFF_UPDATED"
	auditStaffPinReset      = "STAFF_PIN_RESET"
	auditStaffRemoved       = "STAFF_REMOVED"
	auditStaffLocked        = "STAFF_LOCKED"
	auditStaffUnlocked      = "STAFF_UNLOCKED"
	auditBuildingPermission = "BUILDING_PERMISSION_SET"
)
