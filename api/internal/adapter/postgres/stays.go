package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres/sqlcgen"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
)

// oneActiveStayIndex is the partial unique index that backs "one ACTIVE stay per room".
const oneActiveStayIndex = "stays_one_active_per_unit"

// StayRepo implements app.StayRepo. The tenant always comes from the Tx, never from callers.
type StayRepo struct{}

var _ app.StayRepo = StayRepo{}

func (StayRepo) Room(ctx context.Context, tx app.Tx, roomID string, forUpdate bool) (app.CheckInRoom, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.CheckInRoom{}, false, err
	}
	q := sqlcgen.New(t)
	arg := sqlcgen.GetCheckInRoomParams{TenantID: t.tenant, UnitID: roomID}
	var r sqlcgen.GetCheckInRoomRow
	if forUpdate {
		var locked sqlcgen.LockCheckInRoomRow
		locked, err = q.LockCheckInRoom(ctx, sqlcgen.LockCheckInRoomParams(arg))
		r = sqlcgen.GetCheckInRoomRow(locked)
	} else {
		r, err = q.GetCheckInRoom(ctx, arg)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return app.CheckInRoom{}, false, nil
	}
	if err != nil {
		return app.CheckInRoom{}, false, wrap("select check-in room", err)
	}
	return app.CheckInRoom{ID: r.ID, Code: r.Code, BuildingID: r.BuildingID, StoredStatus: r.Status,
		RatePlan: r.RatePlan, RatePlanVersion: int(r.RatePlanVersion)}, true, nil
}

// InsertStay maps a violation of the one-active-stay index to room.ErrNotVacant: the backstop
// behind the room row lock.
func (StayRepo) InsertStay(ctx context.Context, tx app.Tx, s app.NewStay) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	if s.RatePlanSchema < 0 || s.RatePlanSchema > math.MaxInt16 { // the column is a smallint
		return fmt.Errorf("insert stay: rate plan schema %d out of range", s.RatePlanSchema)
	}
	err = sqlcgen.New(t).InsertStay(ctx, sqlcgen.InsertStayParams{
		ID: s.ID, TenantID: t.tenant, UnitID: s.RoomID, RentalType: s.RentalType, Deposit: s.Deposit,
		CheckInAt: pgtype.Timestamptz{Time: s.CheckInAt, Valid: true}, RatePlanSnapshot: s.RatePlanSnapshot,
		RatePlanSchema: int16(s.RatePlanSchema), GuestName: s.GuestName, GuestPhone: s.GuestPhone,
		CreatedBy: optText(s.CreatedBy),
	})
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == pgUniqueViolation && pe.ConstraintName == oneActiveStayIndex {
		return room.ErrNotVacant
	}
	if err != nil {
		return insertFailure(err)
	}
	if len(s.IDNumberEnc) > 0 { // collected at check-in under the stay-declaration duty
		return GuestIDRepo{}.putNumber(ctx, t, s.ID, s.IDNumberEnc, s.CheckInAt, s.CreatedBy)
	}
	return nil
}

// insertFailure keeps what an operator needs (SQLSTATE and constraint name) and drops the rest: the
// pg error text and detail quote the row's values.
func insertFailure(err error) error {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return fmt.Errorf("insert stay failed: sqlstate %s constraint %q", pe.Code, pe.ConstraintName)
	}
	return errors.New("insert stay failed")
}

func (StayRepo) MarkRoomOccupied(ctx context.Context, tx app.Tx, roomID string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	n, err := sqlcgen.New(t).OccupyRoom(ctx, sqlcgen.OccupyRoomParams{TenantID: t.tenant, UnitID: roomID})
	if err != nil {
		return wrap("occupy room", err)
	}
	if n != 1 {
		return room.ErrNotVacant
	}
	return nil
}

func (StayRepo) StayByID(ctx context.Context, tx app.Tx, stayID string) (app.StayRecord, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.StayRecord{}, false, err
	}
	q := sqlcgen.New(t)
	r, err := q.GetStayByID(ctx, sqlcgen.GetStayByIDParams{TenantID: t.tenant, StayID: stayID})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.StayRecord{}, false, nil
	}
	if err != nil {
		return app.StayRecord{}, false, wrap("select stay", err)
	}
	extras, err := q.ListStayExtras(ctx, sqlcgen.ListStayExtrasParams{TenantID: t.tenant, StayID: stayID})
	if err != nil {
		return app.StayRecord{}, false, wrap("select stay extras", err)
	}
	rec, err := toStayRecord(r, extras)
	return rec, err == nil, err
}

func (StayRepo) Timezone(ctx context.Context, tx app.Tx) (string, error) {
	return RoomRepo{}.Timezone(ctx, tx)
}

func toStayRecord(r sqlcgen.GetStayByIDRow, extras []sqlcgen.ListStayExtrasRow) (app.StayRecord, error) {
	rec := app.StayRecord{
		ID: r.ID, RoomID: r.UnitID, RoomCode: r.RoomCode, BuildingID: r.BuildingID, RentalType: r.RentalType,
		Status: r.Status, GuestName: r.GuestName, GuestPhone: r.GuestPhone,
		GuestID: app.GuestIDIndicators{HasIDNumber: r.HasIDNumber, HasFrontPhoto: r.HasFrontPhoto, HasBackPhoto: r.HasBackPhoto},
		Deposit: r.Deposit, CheckInAt: r.CheckInAt.Time, RatePlanSnapshot: r.RatePlanSnapshot,
	}
	if r.CheckOutAt.Valid {
		out := r.CheckOutAt.Time
		rec.CheckOutAt = &out
	}
	for _, e := range extras {
		var name struct{ VI, EN string }
		if err := json.Unmarshal(e.ServiceName, &name); err != nil {
			// The raw value could hold tenant data; report the service code only.
			return app.StayRecord{}, fmt.Errorf("decode name of service %s: %w", e.ServiceCode, err)
		}
		rec.Extras = append(rec.Extras, app.ExtraRecord{ServiceCode: e.ServiceCode, Name: app.LocalizedName(name),
			Quantity: int64(e.Quantity), UnitAmount: e.UnitAmount})
	}
	return rec, nil
}

func (StayRepo) PendingPayment(ctx context.Context, tx app.Tx, stayID string) (*app.PendingPayment, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	r, err := sqlcgen.New(t).GetStayPendingPayment(ctx, sqlcgen.GetStayPendingPaymentParams{TenantID: t.tenant, StayID: stayID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, wrap("stay pending payment", err)
	}
	return app.NewPendingPayment(r.PaymentID, r.Total, r.Deposit, r.Received, r.RefundDue, r.CreatedAt.Time), nil
}
