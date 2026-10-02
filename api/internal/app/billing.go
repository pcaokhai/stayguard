package app

import (
	"context"
	"encoding/json"
	"fmt"
)

const (
	// statusOK is stored with the replayable response (app cannot import net/http).
	statusOK        = 200
	extraIDPrefix   = "sx"
	invoiceIDPrefix = "iv"
	entityInvoice   = "invoice"
)

// Billing holds the extras and check-out use cases (SG-205). It does no I/O of its own: everything goes
// through ports. It is a separate type from Stays so the check-in constructor and its wiring stay untouched.
type Billing struct {
	uow      UnitOfWork
	stays    BillingStayRepo
	services ServiceRepo
	enc      Encryptor
	idem     IdempotencyStore
	audit    AuditWriter
	ids      IDGenerator
	clock    Clock
	guard
}

func NewBilling(uow UnitOfWork, stays BillingStayRepo, services ServiceRepo, levels BuildingLevels, enc Encryptor,
	idem IdempotencyStore, audit AuditWriter, ids IDGenerator, clock Clock) *Billing {
	return &Billing{uow: uow, stays: stays, services: services, enc: enc, idem: idem, audit: audit, ids: ids,
		clock: clock, guard: guard{levels: levels}}
}

// checkKey bounds the idempotency key before it reaches the store.
func checkKey(key string) error {
	if key == "" || len(key) > maxIdempotencyKeyBytes {
		return ErrInvalidIdempotencyKey
	}
	return nil
}

// begin finds the stay (a foreign or unknown id is a 404 before it can be a 403), checks EDIT on its
// building, then asks the idempotency store; the outcome has Replay set when the key was used before.
func (b *Billing) begin(ctx context.Context, tx Tx, op string, c Caller, stayID, route, key, hash string) (IdempotencyOutcome, error) {
	rec, ok, err := b.stays.StayByID(ctx, tx, stayID)
	if err != nil {
		return IdempotencyOutcome{}, fmt.Errorf("stay: %w", err)
	}
	if !ok {
		return IdempotencyOutcome{}, ErrNotFound
	}
	if err := b.checkBuilding(ctx, op, c, rec.BuildingID); err != nil {
		return IdempotencyOutcome{}, err
	}
	return b.idem.Begin(ctx, tx, route, key, hash)
}

// complete stores the answer under the key so a retry replays it.
func (b *Billing) complete(ctx context.Context, tx Tx, route, key string, status int, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	return b.idem.Complete(ctx, tx, route, key, status, body)
}

func (b *Billing) auditAppend(ctx context.Context, tx Tx, c Caller, action, entityType, entityID string, after any) error {
	raw, err := json.Marshal(after)
	if err != nil {
		return fmt.Errorf("encode audit: %w", err)
	}
	e := AuditEntry{ID: b.ids.New(auditIDPrefix), ActorID: c.UserID, Action: action, EntityType: entityType, EntityID: entityID, After: raw}
	if err := b.audit.Append(ctx, tx, e); err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	return nil
}

// detail builds the stay view as of now (ACTIVE) or of the check-out (CHECKED_OUT), like getStay.
func (b *Billing) detail(ctx context.Context, tx Tx, c Caller, rec StayRecord) (StayDetail, error) {
	loc, err := loadZone(ctx, tx, b.stays)
	if err != nil {
		return StayDetail{}, err
	}
	asOf, err := quoteInstant(rec, b.clock.Now())
	if err != nil {
		return StayDetail{}, err
	}
	return detailFor(rec, asOf, loc)
}
