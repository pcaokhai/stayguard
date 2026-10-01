package app

import (
	"context"
	"encoding/json"
)

// AuditEntry is one append-only audit record. The tenant comes from the transaction.
// Before and After must hold only ids, statuses and amounts: never names, phone or ID numbers,
// bank accounts or secrets (CLAUDE.md §6 rule 10). There is no free-form text field on purpose.
type AuditEntry struct {
	ID         string
	ActorID    string // empty for system actions
	Action     string
	EntityType string
	EntityID   string
	Before     json.RawMessage // nil for creations
	After      json.RawMessage // nil for deletions
	TraceID    string
}

// AuditWriter inserts only, in the same transaction as the command it records.
type AuditWriter interface {
	Append(ctx context.Context, tx Tx, e AuditEntry) error
}
