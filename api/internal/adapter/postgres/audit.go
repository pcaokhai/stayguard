package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres/sqlcgen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// AuditWriter implements app.AuditWriter: insert only, in the caller's transaction.
type AuditWriter struct{}

var _ app.AuditWriter = AuditWriter{}

// NewAuditWriter returns the writer; it holds no state.
func NewAuditWriter() AuditWriter { return AuditWriter{} }

// Append inserts one audit row under the transaction's tenant.
func (AuditWriter) Append(ctx context.Context, tx app.Tx, e app.AuditEntry) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	err = sqlcgen.New(t).InsertAuditLog(ctx, sqlcgen.InsertAuditLogParams{
		ID: e.ID, TenantID: t.tenant, ActorID: optText(e.ActorID), Action: e.Action,
		EntityType: e.EntityType, EntityID: e.EntityID, Before: e.Before, After: e.After, TraceID: optText(e.TraceID),
	})
	if err != nil {
		return fmt.Errorf("append audit log: %w", err)
	}
	return nil
}

func optText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
