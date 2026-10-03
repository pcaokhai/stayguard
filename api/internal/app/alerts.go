package app

import (
	"context"
	"time"
)

// Alert kinds; they match the contract's AlertKind and the alerts.kind CHECK.
const (
	AlertAccountLocked       = "ACCOUNT_LOCKED"
	AlertCashOver            = "CASH_OVER"
	AlertCashShort           = "CASH_SHORT"
	AlertDamageReported      = "DAMAGE_REPORTED"
	AlertLeaveRequested      = "LEAVE_REQUESTED"
	AlertPaymentMismatch     = "PAYMENT_MISMATCH"
	AlertOverpaid            = "OVERPAID"
	AlertPaymentPartial      = "PAYMENT_PARTIAL"
	AlertPaymentUnpaid       = "PAYMENT_UNPAID"
	AlertRefundPending       = "REFUND_PENDING"
	AlertSepayUpdated        = "SEPAY_UPDATED"
	AlertStayTimeEdited      = "STAY_TIME_EDITED"
	AlertStocktakeDifference = "STOCKTAKE_DIFFERENCE"
	AlertUnmatchedTransfer   = "UNMATCHED_TRANSFER"
	AlertUnusedRoomReport    = "UNUSED_ROOM_REPORT"
	alertIDPrefix            = "al"
)

// AlertDraft is one alert to raise. Details hold short display values (times, codes), never ID numbers,
// phones, PINs or account numbers (CLAUDE.md §4 rule 9).
type AlertDraft struct {
	ID, Kind                      string
	RoomCode, ShiftID, StayID, By string // empty means none
	Amount                        *int64
	Details                       map[string]string
}

// AlertWriter inserts in the transaction of the command that raises the alert, so both commit or neither does.
type AlertWriter interface {
	Raise(ctx context.Context, tx Tx, a AlertDraft) error
}

// Resolutions of an alert: how it stopped needing the owner.
const (
	ResolutionPaid      = "PAID"
	ResolutionRefunded  = "REFUNDED"
	ResolutionLinked    = "LINKED"
	ResolutionDismissed = "DISMISSED"
)

// moneyAlertKinds are the alerts about one stay's invoice; they resolve when the invoice is settled.
var moneyAlertKinds = []string{AlertPaymentMismatch, AlertPaymentPartial, AlertPaymentUnpaid, AlertRefundPending}

// AlertResolve closes open alerts, in the transaction of the command that settled the money. It resolves the money alerts of a stay
// and, for a linked bank event, the unmatched-transfer alert of that event (found by event id, or by amount and note for an alert
// raised before event ids were kept). History stays.
type AlertResolve struct {
	StayID, Resolution string
	Kinds              []string
	EventID, EventNote string
	EventAmount        int64
	At                 time.Time
}

// AlertResolver is implemented by the alert adapter next to AlertWriter.
type AlertResolver interface {
	Resolve(ctx context.Context, tx Tx, r AlertResolve) error
}
