package app

import "strings"

// auditCategories maps the action codes written to audit_logs to the activity-log categories of the contract.
// The prefixes do not overlap, so the category of an action and the filter for a category agree.
// A new audit action needs a prefix here (TestAuditCategories_* fail otherwise); without one it shows under RATES_SETTINGS and no category filter finds it.
var auditCategories = []struct{ prefix, category string }{
	{"payment.", "MONEY"}, {"expense.", "MONEY"}, {"payroll.", "MONEY"}, {"stay.checked_out", "MONEY"}, {"stay.extras_added", "MONEY"},
	{"stay.check_in", "STAY_TIME"}, {"stay.moved", "STAY_TIME"},
	{"shift.", "SHIFT"},
	{"SERVICE_", "STOCK"}, {"STOCK_", "STOCK"}, {"STOCKTAKE_", "STOCK"},
	{"room.cleaned", "MAINTENANCE"}, {"ticket.", "MAINTENANCE"},
	{"STAFF_", "ACCESS_STAFF"}, {"BUILDING_PERMISSION_SET", "ACCESS_STAFF"}, {"ACCOUNT_LOCKED", "ACCESS_STAFF"}, {"roster.", "ACCESS_STAFF"}, {"leave.", "ACCESS_STAFF"},
	{"BUILDING_CREATED", "RATES_SETTINGS"}, {"BUILDING_RENAMED", "RATES_SETTINGS"}, {"FLOOR_", "RATES_SETTINGS"}, {"ROOMS_CREATED", "RATES_SETTINGS"},
	{"ROOM_UPDATED", "RATES_SETTINGS"}, {"RATE_PLAN_", "RATES_SETTINGS"}, {"PROPERTY_", "RATES_SETTINGS"},
	{"INSTALLER_", "INSTALLER"}, {"BANK_ACCOUNT_", "INSTALLER"},
	{"GUEST_ID_", "GUEST_ID"},
}

const defaultAuditCategory = "RATES_SETTINGS"

// auditCategoryOf is the category of an action code.
func auditCategoryOf(action string) string {
	for _, c := range auditCategories {
		if strings.HasPrefix(action, c.prefix) {
			return c.category
		}
	}
	return defaultAuditCategory
}

// auditPrefixesOf lists the LIKE patterns of a category; ok is false for an unknown category.
func auditPrefixesOf(category string) (patterns []string, ok bool) {
	for _, c := range auditCategories {
		if c.category == category {
			patterns = append(patterns, c.prefix+"%")
		}
	}
	return patterns, len(patterns) > 0
}
