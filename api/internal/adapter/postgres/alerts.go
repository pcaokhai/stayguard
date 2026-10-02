package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres/sqlcgen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// AlertWriter implements app.AlertWriter: insert only, in the caller's transaction.
type AlertWriter struct{}

var _ app.AlertWriter = AlertWriter{}

func (AlertWriter) Raise(ctx context.Context, tx app.Tx, a app.AlertDraft) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	details := a.Details
	if details == nil {
		details = map[string]string{}
	}
	raw, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("encode alert details: %w", err)
	}
	arg := sqlcgen.InsertAlertParams{ID: a.ID, TenantID: t.tenant, Kind: a.Kind, RoomCode: optText(a.RoomCode),
		ShiftID: optText(a.ShiftID), StayID: optText(a.StayID), ActorID: optText(a.By), Details: raw}
	if a.Amount != nil {
		arg.Amount = pgtype.Int8{Int64: *a.Amount, Valid: true}
	}
	return writeFailure("insert alert", sqlcgen.New(t).InsertAlert(ctx, arg))
}
