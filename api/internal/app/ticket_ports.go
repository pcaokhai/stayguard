package app

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrRoomOccupied: the room has a guest in it, so it cannot be locked (HTTP 409 ROOM_OCCUPIED).
	ErrRoomOccupied = errors.New("room is occupied")
	// ErrTicketDone: a finished ticket can no longer change (HTTP 409 TICKET_DONE).
	ErrTicketDone = errors.New("ticket is done")
	// ErrTicketStatus: a ticket only moves forward (HTTP 409 TICKET_STATUS).
	ErrTicketStatus = errors.New("ticket status cannot go back")
)

// TicketRecord is a stored ticket with its room.
type TicketRecord struct {
	ID, Code, RoomID, RoomCode, BuildingID, RoomStatus string
	Category, Description, Status, ReportedBy          string
	RoomLocked                                         bool
	ReportedAt                                         time.Time
	ExpectedDoneOn                                     *time.Time
	PartsCost, LabourCost                              *int64
	Repairer, Note                                     *string
	CompletedAt                                        *time.Time
}

type NewTicket struct {
	ID, Code, RoomID, Category, Description, ReportedBy string
	RoomLocked                                          bool
	ReportedAt                                          time.Time
}

// TicketChange is every editable column of a ticket after an update.
type TicketChange struct {
	ID, Status, CompletedBy string
	RoomLocked              bool
	ExpectedDoneOn          *time.Time
	PartsCost, LabourCost   *int64
	Repairer, Note          *string
	CompletedAt             *time.Time
}

// RoomLock is a room row locked for the transaction.
type RoomLock struct{ ID, Code, BuildingID, Status string }

// TicketRepo filters by the tenant of the Tx.
type TicketRepo interface {
	Timezone(ctx context.Context, tx Tx) (string, error)
	NextSeq(ctx context.Context, tx Tx) (int, error)
	Insert(ctx context.Context, tx Tx, t NewTicket) error
	// Lock takes the ticket row lock; false when there is none.
	Lock(ctx context.Context, tx Tx, ticketID string) (bool, error)
	ByID(ctx context.Context, tx Tx, ticketID string) (TicketRecord, bool, error)
	List(ctx context.Context, tx Tx, status string) ([]TicketRecord, error)
	Update(ctx context.Context, tx Tx, c TicketChange) error
	LockRoom(ctx context.Context, tx Tx, roomID string) (RoomLock, bool, error)
	// SetMaintenance and Reopen move the room status; SetMaintenance returns ErrRoomOccupied when a guest is in the room (the backstop).
	SetMaintenance(ctx context.Context, tx Tx, roomID string) error
	Reopen(ctx context.Context, tx Tx, roomID string) error
	// OtherLocks counts the open tickets of the room, apart from ticketID, that keep it locked.
	OtherLocks(ctx context.Context, tx Tx, roomID, ticketID string) (int, error)
}
