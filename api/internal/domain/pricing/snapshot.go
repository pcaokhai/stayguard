package pricing

import "encoding/json"

// ParseRatePlan is the only supported decode path; Snapshot is meant for plans it returned.
//
// Snapshot is the canonical JSON copied onto a stay at check-in: fixed field order, no
// whitespace, so parse then Snapshot is byte-stable.
func (p RatePlan) Snapshot() []byte {
	// Marshal cannot fail for this plain struct of integers and strings.
	b, _ := json.Marshal(p)
	return b
}
