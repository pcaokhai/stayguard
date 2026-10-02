package postgres

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres/sqlcgen"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

// StayEditRepo implements app.StayEditRepo. The tenant always comes from the Tx.
type StayEditRepo struct{}

var _ app.StayEditRepo = StayEditRepo{}

func (StayEditRepo) LockStay(ctx context.Context, tx app.Tx, stayID string) (app.StayRecord, bool, error) {
	return BillingRepo{}.LockStay(ctx, tx, stayID)
}

func (StayEditRepo) Room(ctx context.Context, tx app.Tx, roomID string, forUpdate bool) (app.CheckInRoom, bool, error) {
	return StayRepo{}.Room(ctx, tx, roomID, forUpdate)
}

func (StayEditRepo) Timezone(ctx context.Context, tx app.Tx) (string, error) {
	return RoomRepo{}.Timezone(ctx, tx)
}

func (StayEditRepo) MarkRoomOccupied(ctx context.Context, tx app.Tx, roomID string) error {
	return StayRepo{}.MarkRoomOccupied(ctx, tx, roomID)
}

func (StayEditRepo) FirstRecordedCheckIn(ctx context.Context, tx app.Tx, stayID string) (time.Time, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return time.Time{}, false, err
	}
	at, err := sqlcgen.New(t).FirstRecordedCheckIn(ctx, sqlcgen.FirstRecordedCheckInParams{TenantID: t.tenant, StayID: stayID})
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, wrap("select first check-in", err)
	}
	return at.Time, at.Valid, nil
}

func (StayEditRepo) UpdateCheckIn(ctx context.Context, tx app.Tx, stayID string, at time.Time) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	n, err := sqlcgen.New(t).UpdateStayCheckIn(ctx, sqlcgen.UpdateStayCheckInParams{TenantID: t.tenant, StayID: stayID,
		CheckInAt: pgtype.Timestamptz{Time: at, Valid: true}})
	if err != nil {
		return wrap("update check-in", err)
	}
	if n != 1 {
		return stay.ErrNotActive
	}
	return nil
}

func (StayEditRepo) MoveStay(ctx context.Context, tx app.Tx, m app.MovedStay) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	if m.RatePlanSchema < 0 || m.RatePlanSchema > math.MaxInt16 { // the column is a smallint
		return errors.New("move stay: rate plan schema out of range")
	}
	n, err := sqlcgen.New(t).MoveStay(ctx, sqlcgen.MoveStayParams{TenantID: t.tenant, StayID: m.StayID, UnitID: m.ToRoomID,
		RentalType: m.RentalType, RatePlanSnapshot: m.RatePlanSnapshot, RatePlanSchema: int16(m.RatePlanSchema)})
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == pgUniqueViolation && pe.ConstraintName == oneActiveStayIndex {
		return room.ErrNotVacant
	}
	if err != nil {
		return writeFailure("move stay", err)
	}
	if n != 1 {
		return stay.ErrNotActive
	}
	return nil
}

func (StayEditRepo) MarkRoomToClean(ctx context.Context, tx app.Tx, roomID string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	n, err := sqlcgen.New(t).ReleaseRoom(ctx, sqlcgen.ReleaseRoomParams{TenantID: t.tenant, UnitID: roomID})
	if err != nil {
		return wrap("release room", err)
	}
	if n != 1 {
		return room.ErrNotOccupied
	}
	return nil
}

func (StayEditRepo) InsertEdit(ctx context.Context, tx app.Tx, e app.StayEdit) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return writeFailure("insert stay edit", sqlcgen.New(t).InsertStayEdit(ctx, sqlcgen.InsertStayEditParams{
		ID: e.ID, TenantID: t.tenant, StayID: e.StayID, Kind: e.Kind, ActorID: optText(e.ActorID),
		OldCheckInAt: optTimePtr(e.OldCheckInAt), NewCheckInAt: optTimePtr(e.NewCheckInAt),
		ReasonCode: optText(e.ReasonCode), Note: optText(e.Note),
		FromRoomCode: optText(e.FromRoomCode), ToRoomCode: optText(e.ToRoomCode),
		CreatedAt: pgtype.Timestamptz{Time: e.CreatedAt, Valid: true},
	}))
}

func optTimePtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}
