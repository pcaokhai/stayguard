package app

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrInvoiceNotOpen: the invoice was already paid.
	ErrInvoiceNotOpen = errors.New("invoice is not open")
	// ErrNoBankAccount: the tenant has no bank account to receive transfers.
	ErrNoBankAccount = errors.New("tenant has no bank account")
	// ErrPaymentNotPending is the backstop of the settle and mismatch updates (zero rows changed).
	ErrPaymentNotPending = errors.New("payment is not pending")
	// ErrLinkAmount: the event does not equal the invoice's remaining balance (HTTP 409 LINK_AMOUNT_MISMATCH).
	ErrLinkAmount = errors.New("event amount is not the remaining balance")
	// ErrEventNotLinkable: the bank event was already settled or linked, so linking it again is refused (HTTP 409).
	ErrEventNotLinkable = errors.New("payment event is not an unmatched transfer")
)

// PayInvoice is the invoice as payment needs it, locked for the transaction.
type PayInvoice struct {
	ID, Status, BillCode, StayID, BuildingID string
	Quote                                    []byte
}

// PaymentRecord is a stored payment with its invoice's bill code and building.
type PaymentRecord struct {
	ID, InvoiceID, Method, Status, BillCode, BuildingID string
	Amount                                              int64
	ReceivedAmount                                      *int64
	PaidAt                                              *time.Time
	TransactionID                                       *string
}

// PendingTransfer is an open transfer the payment-event handler may match.
type PendingTransfer struct {
	PaymentID, InvoiceID, StayID, BillCode, RoomCode string
	Amount                                           int64
}

// NewPayment is a payment row to insert; a TRANSFER is always created PENDING by the adapter.
type NewPayment struct {
	ID, InvoiceID, BillCode string
	Amount                  int64
	At                      time.Time
}

// PaymentEvent is a normalised bank event: the simulator and every provider adapter produce this.
type PaymentEvent struct {
	TenantID, Provider, ExternalID, Content string
	Amount                                  int64
	ReceivedAt                              time.Time
}

// StoredEvent is a payment event row as stored; Result is a payment.Result value.
type StoredEvent struct {
	ID, Result string
	Event      PaymentEvent
}

// PaymentRepo filters by the tenant of the Tx. SettleTransfer is called only by the payment-event handler.
type PaymentRepo interface {
	LockInvoice(ctx context.Context, tx Tx, invoiceID string) (PayInvoice, bool, error)
	PaymentByID(ctx context.Context, tx Tx, id string) (PaymentRecord, bool, error)
	PendingForInvoice(ctx context.Context, tx Tx, invoiceID string) (PaymentRecord, bool, error)
	PendingTransfers(ctx context.Context, tx Tx) ([]PendingTransfer, error)
	InsertPendingTransfer(ctx context.Context, tx Tx, p NewPayment) error
	InsertCashPayment(ctx context.Context, tx Tx, p NewPayment) error
	ExpirePending(ctx context.Context, tx Tx, invoiceID string) error
	// InsertEvent reports false when (provider, external id) was seen before.
	InsertEvent(ctx context.Context, tx Tx, id string, ev PaymentEvent) (bool, error)
	SetEventResult(ctx context.Context, tx Tx, ev PaymentEvent, result string) error
	// LockEvent takes the row lock of a stored event of this tenant; false when there is none.
	LockEvent(ctx context.Context, tx Tx, eventID string) (StoredEvent, bool, error)
	// ReceivedForInvoice is the bank money matched to the invoice that did not close it (PARTIAL events).
	ReceivedForInvoice(ctx context.Context, tx Tx, invoiceID string) (int64, error)
	// SetEventMatched ties an event to an invoice with a result (PARTIAL or SETTLED).
	SetEventMatched(ctx context.Context, tx Tx, ev PaymentEvent, invoiceID, result string) error
	// SettleInvoiceEvents turns the PARTIAL events of a paid invoice into SETTLED.
	SettleInvoiceEvents(ctx context.Context, tx Tx, invoiceID string) error
	// StalePartials lists open invoices with bank money whose first partial event is at or before the cutoff and that
	// have no PAYMENT_PARTIAL alert yet.
	StalePartials(ctx context.Context, tx Tx, before time.Time) ([]StalePartial, error)
	// SetTransferReceived records the cumulative bank money on a pending transfer; it stays PENDING.
	SetTransferReceived(ctx context.Context, tx Tx, paymentID string, received int64) error
	// SettleTransfer and MarkMismatch return ErrPaymentNotPending when no pending transfer was updated.
	SettleTransfer(ctx context.Context, tx Tx, paymentID string, paidAt time.Time, received int64, transactionID string) error
	MarkMismatch(ctx context.Context, tx Tx, paymentID string, received int64, transactionID string) error
	// CloseInvoice marks the invoice PAID and its occupied room TO_CLEAN.
	CloseInvoice(ctx context.Context, tx Tx, invoiceID, stayID string, at time.Time) error
	// DefaultBankAccount is the tenant's default, SePay-connected receiving account (empty id when there is none):
	// the one account a QR may pay.
	DefaultBankAccount(ctx context.Context, tx Tx) (id string, enc []byte, err error)
}

// StalePartial is an invoice that got part of its money and is still not paid.
type StalePartial struct {
	InvoiceID, BillCode, StayID, RoomCode string
	Received, Remaining                   int64
	FirstAt                               time.Time
}
