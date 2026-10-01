package app

import (
	"encoding/json"
	"fmt"
	"time"
)

// InvoiceView is the check-out answer. It is also the stored idempotency response body, so it must
// round-trip through JSON. It holds no guest data. Quote is the frozen quote: never recomputed.
type InvoiceView struct {
	ID        string
	StayID    string
	RoomCode  string
	BillCode  string
	Status    string
	CreatedAt time.Time
	Quote     QuoteView
}

// invoiceViewOf rebuilds the view from the stored row; the quote comes from the frozen JSON.
func invoiceViewOf(inv InvoiceRecord, roomCode string) (InvoiceView, error) {
	var q QuoteView
	if err := json.Unmarshal(inv.Quote, &q); err != nil {
		return InvoiceView{}, fmt.Errorf("stored invoice quote: %w", err)
	}
	return InvoiceView{ID: inv.ID, StayID: inv.StayID, RoomCode: roomCode, BillCode: inv.BillCode, Status: inv.Status,
		CreatedAt: inv.CreatedAt.UTC(), Quote: q}, nil
}
