package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres/sqlcgen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// StayHistoryRepo implements app.StayHistoryRepo. The tenant always comes from the Tx.
type StayHistoryRepo struct{}

var _ app.StayHistoryRepo = StayHistoryRepo{}

func (StayHistoryRepo) Timezone(ctx context.Context, tx app.Tx) (string, error) {
	return RoomRepo{}.Timezone(ctx, tx)
}

func (StayHistoryRepo) BuildingIDs(ctx context.Context, tx app.Tx) ([]string, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	ids, err := sqlcgen.New(t).ListBuildingIDs(ctx, t.tenant)
	return ids, wrap("list buildings", err)
}

func (StayHistoryRepo) Stays(ctx context.Context, tx app.Tx, f app.StayFilter) ([]app.StayListRow, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	arg := sqlcgen.ListStayHistoryParams{TenantID: t.tenant, FromAt: ts(f.From), ToAt: ts(f.To), BuildingIds: f.BuildingIDs,
		BuildingID: optText(f.BuildingID), State: optText(f.State), RowLimit: int32(f.Limit), // #nosec G115 -- page size constant
		Pattern: optText(likePattern(f.Query))}
	if f.CursorAt != nil {
		arg.CursorAt, arg.CursorID = ts(*f.CursorAt), pgtype.Text{String: f.CursorID, Valid: true}
	}
	rows, err := sqlcgen.New(t).ListStayHistory(ctx, arg)
	if err != nil {
		return nil, wrap("list stay history", err)
	}
	out := make([]app.StayListRow, len(rows))
	for i, r := range rows {
		out[i] = app.StayListRow{ID: r.ID, RoomCode: r.RoomCode, GuestName: r.GuestName, RentalType: r.RentalType,
			Status: r.Status, State: r.State, FrontDeskName: r.FrontDeskName, CheckInAt: r.CheckInAt.Time.UTC(),
			PaymentMethod: r.PaymentMethod}
		if r.CheckOutAt.Valid {
			at := r.CheckOutAt.Time.UTC()
			out[i].CheckOutAt = &at
		}
		if r.Total.Valid {
			total := r.Total.Int64
			out[i].Total = &total
		}
	}
	return out, nil
}

// likePattern makes a contains-pattern for ILIKE ... ESCAPE '\'; empty means no filter.
func likePattern(q string) string {
	if q == "" {
		return ""
	}
	esc := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q)
	return "%" + esc + "%"
}

func (StayHistoryRepo) StayBuilding(ctx context.Context, tx app.Tx, stayID string) (string, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return "", false, err
	}
	id, err := sqlcgen.New(t).GetStayBuilding(ctx, sqlcgen.GetStayBuildingParams{TenantID: t.tenant, StayID: stayID})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, wrap("select stay building", err)
	}
	return id, true, nil
}

func (StayHistoryRepo) Timeline(ctx context.Context, tx app.Tx, stayID string) ([]app.TimelineRow, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListStayTimeline(ctx, sqlcgen.ListStayTimelineParams{TenantID: t.tenant, StayID: stayID})
	if err != nil {
		return nil, wrap("list stay timeline", err)
	}
	out := make([]app.TimelineRow, len(rows))
	for i, r := range rows {
		d := map[string]string{}
		if err := json.Unmarshal(r.Details, &d); err != nil {
			return nil, fmt.Errorf("decode timeline details of kind %s: %w", r.Kind, err)
		}
		out[i] = app.TimelineRow{At: r.At.Time.UTC(), Kind: r.Kind, ActorName: r.ActorName, Details: d}
	}
	return out, nil
}

func (StayHistoryRepo) Receipt(ctx context.Context, tx app.Tx, invoiceID string) (app.ReceiptRecord, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.ReceiptRecord{}, false, err
	}
	q := sqlcgen.New(t)
	r, err := q.GetReceiptInvoice(ctx, sqlcgen.GetReceiptInvoiceParams{TenantID: t.tenant, InvoiceID: invoiceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ReceiptRecord{}, false, nil
	}
	if err != nil {
		return app.ReceiptRecord{}, false, wrap("select receipt invoice", err)
	}
	stayRec, ok, err := BillingRepo{}.StayByID(ctx, tx, r.StayID)
	if err != nil || !ok {
		return app.ReceiptRecord{}, false, wrap("select receipt stay", err)
	}
	pays, err := q.ListReceiptPayments(ctx, sqlcgen.ListReceiptPaymentsParams{TenantID: t.tenant, InvoiceID: invoiceID})
	if err != nil {
		return app.ReceiptRecord{}, false, wrap("select receipt payments", err)
	}
	rec := app.ReceiptRecord{BuildingID: r.BuildingID, PropertyName: r.PropertyName, BillCode: r.BillCode, RoomCode: r.RoomCode,
		CheckInAt: r.CheckInAt.Time, CheckOutAt: r.CheckOutAt.Time, Quote: r.Quote, Extras: stayRec.Extras}
	for _, p := range pays {
		rec.Payments = append(rec.Payments, app.PaidPayment{ID: p.ID, Method: p.Method, Amount: p.Amount, At: p.PaidAt.Time})
	}
	return rec, true, nil
}
