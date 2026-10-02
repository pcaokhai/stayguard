package app

import "context"

// Alert kinds; they match the contract's AlertKind and the alerts.kind CHECK.
const (
	AlertAccountLocked       = "ACCOUNT_LOCKED"
	AlertCashOver            = "CASH_OVER"
	AlertCashShort           = "CASH_SHORT"
	AlertDamageReported      = "DAMAGE_REPORTED"
	AlertLeaveRequested      = "LEAVE_REQUESTED"
	AlertPaymentMismatch     = "PAYMENT_MISMATCH"
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
