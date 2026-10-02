package app

import (
	"context"
	"time"
)

// StayFilter is a history query. From is inclusive and To exclusive (instants, already cut from tenant-zone days);
// BuildingIDs is the buildings the caller may view. Cursor is the last row of the previous page.
type StayFilter struct {
	From, To    time.Time
	BuildingIDs []string
	BuildingID  string // optional, one of BuildingIDs
	Query       string // optional room, guest name or phone
	State       string // optional StayListItem state
	CursorAt    *time.Time
	CursorID    string
	Limit       int
}

// StayListRow is one stay of the history. Total and PaymentMethod are empty until an invoice or payment exists.
type StayListRow struct {
	ID, RoomCode, GuestName, RentalType, Status, State, FrontDeskName string
	CheckInAt                                                         time.Time
	CheckOutAt                                                        *time.Time
	Total                                                             *int64
	PaymentMethod                                                     string
	InvoiceID, BillCode                                               string // empty until the stay is checked out
}

// TimelineRow is one event of a stay; Details hold display values only.
type TimelineRow struct {
	At        time.Time
	Kind      string
	ActorName string
	Details   map[string]string
}

// PaidPayment is a PAID payment of an invoice.
type PaidPayment struct {
	ID, Method string
	Amount     int64
	At         time.Time
}

// ReceiptRecord is what a receipt prints; Quote is the frozen invoice quote JSON.
type ReceiptRecord struct {
	BuildingID, PropertyName, BillCode, RoomCode string
	CheckInAt, CheckOutAt                        time.Time
	Quote                                        []byte
	Extras                                       []ExtraRecord
	Payments                                     []PaidPayment
}

// StayHistoryRepo filters by the tenant of the Tx.
type StayHistoryRepo interface {
	Timezone(ctx context.Context, tx Tx) (string, error)
	// FrontDeskDays is the property setting: how many days back a receptionist may look.
	FrontDeskDays(ctx context.Context, tx Tx) (int, error)
	BuildingIDs(ctx context.Context, tx Tx) ([]string, error)
	Stays(ctx context.Context, tx Tx, f StayFilter) ([]StayListRow, error)
	// StayBuilding reports the building of a stay; false when the stay is unknown.
	StayBuilding(ctx context.Context, tx Tx, stayID string) (string, bool, error)
	Timeline(ctx context.Context, tx Tx, stayID string) ([]TimelineRow, error)
	Receipt(ctx context.Context, tx Tx, invoiceID string) (ReceiptRecord, bool, error)
}
