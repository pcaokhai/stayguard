package app

import "strings"

// auditCategories maps the action codes written to audit_logs to the activity-log categories of the contract.
// The prefixes do not overlap, so the category of an action and the filter for a category agree.
// A new audit action needs a prefix here; until then it shows under RATES_SETTINGS and no category filter finds it.
var auditCategories = []struct{ prefix, category string }{
	{"payment.", "MONEY"}, {"invoice.", "MONEY"}, {"stay.checked_out", "MONEY"}, {"stay.extras_added", "MONEY"},
	{"stay.check_in", "STAY_TIME"}, {"stay.moved", "STAY_TIME"},
	{"shift.", "SHIFT"},
	{"stock.", "STOCK"}, {"service.", "STOCK"},
	{"room.cleaned", "MAINTENANCE"}, {"ticket.", "MAINTENANCE"}, {"damage.", "MAINTENANCE"},
	{"staff.", "ACCESS_STAFF"}, {"user.", "ACCESS_STAFF"}, {"pin.", "ACCESS_STAFF"}, {"auth.", "ACCESS_STAFF"},
	{"permission.", "ACCESS_STAFF"}, {"ACCOUNT_LOCKED", "ACCESS_STAFF"},
	{"rate.", "RATES_SETTINGS"}, {"settings.", "RATES_SETTINGS"}, {"property.", "RATES_SETTINGS"}, {"building.", "RATES_SETTINGS"},
	{"floor.", "RATES_SETTINGS"}, {"room.created", "RATES_SETTINGS"}, {"room.updated", "RATES_SETTINGS"},
	{"sepay.", "INSTALLER"}, {"bank.", "INSTALLER"}, {"tenant.", "INSTALLER"},
	{"guest_id.", "GUEST_ID"},
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
