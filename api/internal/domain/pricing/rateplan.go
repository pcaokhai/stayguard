// Package pricing holds the rate plan model and, later, the pure pricing rules.
package pricing

import (
	"github.com/pcaokhai/stayguard/api/internal/domain/money"
)

const (
	CurrencyVND     = "VND"
	MinVersion      = 1
	MinGraceMinutes = 0
	MaxGraceMinutes = 60
)

// RatePlan mirrors contracts/pricing/rate-plan.schema.json; field order here is the snapshot order.
type RatePlan struct {
	Version      int64  `json:"version"`
	Currency     string `json:"currency"`
	GraceMinutes int    `json:"graceMinutes"`
	Hourly       Hourly `json:"hourly"`
	Overnight    Window `json:"overnight"`
	Daily        Window `json:"daily"`
}

type Hourly struct {
	FirstHour money.Vnd `json:"firstHour"`
	ExtraHour money.Vnd `json:"extraHour"`
}

// Window is a price with the tenant-local clock times that bound it.
type Window struct {
	Price money.Vnd `json:"price"`
	Start Clock     `json:"windowStart"`
	End   Clock     `json:"windowEnd"`
}
