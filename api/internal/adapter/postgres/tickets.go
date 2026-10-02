package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres/sqlcgen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// TicketRepo implements app.TicketRepo. The tenant always comes from the Tx.
type TicketRepo struct{}

var _ app.TicketRepo = TicketRepo{}

func (TicketRepo) Timezone(ctx context.Context, tx app.Tx) (string, error) {
	return RoomRepo{}.Timezone(ctx, tx)
}

func (TicketRepo) NextSeq(ctx context.Context, tx app.Tx) (int, error) {
	t, err := pgTx(tx)
	if err != nil {
		return 0, err
	}
	n, err := sqlcgen.New(t).NextTicketSeq(ctx, t.tenant)
	return int(n), wrap("next ticket number", err)
}

func (TicketRepo) Insert(ctx context.Context, tx app.Tx, n app.NewTicket) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return writeFailure("insert ticket", sqlcgen.New(t).InsertTicket(ctx, sqlcgen.InsertTicketParams{ID: n.ID, TenantID: t.tenant, Code: n.Code,
		UnitID: n.RoomID, Category: n.Category, Description: n.Description, RoomLocked: n.RoomLocked, ReportedBy: optText(n.ReportedBy),
		ReportedAt: ts(n.ReportedAt)}))
}

func (TicketRepo) Lock(ctx context.Context, tx app.Tx, ticketID string) (bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return false, err
	}
	_, err = sqlcgen.New(t).LockTicket(ctx, sqlcgen.LockTicketParams{TenantID: t.tenant, TicketID: ticketID})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, wrap("lock ticket", err)
}

func toTicketRecord(r sqlcgen.GetTicketRow) app.TicketRecord {
	rec := app.TicketRecord{ID: r.ID, Code: r.Code, RoomID: r.UnitID, RoomCode: r.RoomCode, BuildingID: r.BuildingID, RoomStatus: r.RoomStatus,
		Category: r.Category, Description: r.Description, Status: r.Status, ReportedBy: r.ReportedBy, RoomLocked: r.RoomLocked,
		ReportedAt: r.ReportedAt.Time, PartsCost: intPtr(r.PartsCost), LabourCost: intPtr(r.LabourCost), CompletedAt: timePtr(r.CompletedAt)}
	if r.ExpectedDoneOn.Valid {
		d := r.ExpectedDoneOn.Time
		rec.ExpectedDoneOn = &d
	}
	if r.Repairer.Valid {
		rec.Repairer = &r.Repairer.String
	}
	if r.Note.Valid {
		rec.Note = &r.Note.String
	}
	return rec
}

func (TicketRepo) ByID(ctx context.Context, tx app.Tx, ticketID string) (app.TicketRecord, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.TicketRecord{}, false, err
	}
	r, err := sqlcgen.New(t).GetTicket(ctx, sqlcgen.GetTicketParams{TenantID: t.tenant, TicketID: ticketID})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.TicketRecord{}, false, nil
	}
	if err != nil {
		return app.TicketRecord{}, false, wrap("select ticket", err)
	}
	return toTicketRecord(r), true, nil
}

func (TicketRepo) List(ctx context.Context, tx app.Tx, status string) ([]app.TicketRecord, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).ListTickets(ctx, sqlcgen.ListTicketsParams{TenantID: t.tenant, Status: optText(status)})
	if err != nil {
		return nil, wrap("list tickets", err)
	}
	out := make([]app.TicketRecord, len(rows))
	for i, r := range rows {
		out[i] = toTicketRecord(sqlcgen.GetTicketRow(r))
	}
	return out, nil
}

func (TicketRepo) Update(ctx context.Context, tx app.Tx, c app.TicketChange) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	arg := sqlcgen.UpdateTicketParams{TenantID: t.tenant, TicketID: c.ID, Status: c.Status, RoomLocked: c.RoomLocked,
		PartsCost: optInt(c.PartsCost), LabourCost: optInt(c.LabourCost), CompletedAt: optTimePtr(c.CompletedAt), CompletedBy: optText(c.CompletedBy)}
	if c.ExpectedDoneOn != nil {
		arg.ExpectedDoneOn = pgtype.Date{Time: *c.ExpectedDoneOn, Valid: true}
	}
	if c.Repairer != nil {
		arg.Repairer = pgtype.Text{String: *c.Repairer, Valid: true}
	}
	if c.Note != nil {
		arg.Note = pgtype.Text{String: *c.Note, Valid: true}
	}
	return writeFailure("update ticket", sqlcgen.New(t).UpdateTicket(ctx, arg))
}

func optInt(n *int64) pgtype.Int8 {
	if n == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *n, Valid: true}
}

func (TicketRepo) LockRoom(ctx context.Context, tx app.Tx, roomID string) (app.RoomLock, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.RoomLock{}, false, err
	}
	r, err := sqlcgen.New(t).LockRoomForMaintenance(ctx, sqlcgen.LockRoomForMaintenanceParams{TenantID: t.tenant, UnitID: roomID})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.RoomLock{}, false, nil
	}
	if err != nil {
		return app.RoomLock{}, false, wrap("lock room", err)
	}
	return app.RoomLock{ID: r.ID, Code: r.Code, BuildingID: r.BuildingID, Status: r.Status}, true, nil
}

func (TicketRepo) SetMaintenance(ctx context.Context, tx app.Tx, roomID string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	n, err := sqlcgen.New(t).SetRoomMaintenance(ctx, sqlcgen.SetRoomMaintenanceParams{TenantID: t.tenant, UnitID: roomID})
	if err != nil {
		return wrap("lock room for maintenance", err)
	}
	if n != 1 {
		return app.ErrRoomOccupied
	}
	return nil
}

func (TicketRepo) Reopen(ctx context.Context, tx app.Tx, roomID string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	_, err = sqlcgen.New(t).ReopenRoom(ctx, sqlcgen.ReopenRoomParams{TenantID: t.tenant, UnitID: roomID})
	return wrap("reopen room", err)
}

func (TicketRepo) OtherLocks(ctx context.Context, tx app.Tx, roomID, ticketID string) (int, error) {
	t, err := pgTx(tx)
	if err != nil {
		return 0, err
	}
	n, err := sqlcgen.New(t).CountOtherLockingTickets(ctx, sqlcgen.CountOtherLockingTicketsParams{TenantID: t.tenant, UnitID: roomID, TicketID: ticketID})
	return int(n), wrap("count locking tickets", err)
}
