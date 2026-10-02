package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres/sqlcgen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// GuestIDRepo is the only code that reads guest ID numbers and photos. Stay queries carry yes/no indicators
// only (an EXISTS), so no front-desk read can select the data; the use case that calls this repo is owner and
// manager only for reads.
type GuestIDRepo struct{}

var _ app.GuestIDRepo = GuestIDRepo{}

// putNumber stores a ciphertext and the collection record that came with it (check-in).
func (GuestIDRepo) putNumber(ctx context.Context, t Tx, stayID string, enc []byte, at time.Time, by string) error {
	return wrap("store guest id number", sqlcgen.New(t).UpsertGuestNumber(ctx, sqlcgen.UpsertGuestNumberParams{
		TenantID: t.tenant, StayID: stayID, NumberEnc: enc, At: pgtype.Timestamptz{Time: at, Valid: true}, By: optText(by)}))
}

func (GuestIDRepo) Stay(ctx context.Context, tx app.Tx, stayID string) (app.GuestIDStay, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.GuestIDStay{}, false, err
	}
	r, err := sqlcgen.New(t).GuestIDStay(ctx, sqlcgen.GuestIDStayParams{TenantID: t.tenant, StayID: stayID})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.GuestIDStay{}, false, nil
	}
	if err != nil {
		return app.GuestIDStay{}, false, wrap("select stay for guest id", err)
	}
	return app.GuestIDStay{ID: r.ID, Status: r.Status, BuildingID: r.BuildingID, RoomCode: r.RoomCode, CheckOutAt: optTime(r.CheckOutAt)}, true, nil
}

func (g GuestIDRepo) SetNumber(ctx context.Context, tx app.Tx, stayID string, enc []byte, at time.Time, by string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return g.putNumber(ctx, t, stayID, enc, at, by)
}

func (GuestIDRepo) EnsureRecord(ctx context.Context, tx app.Tx, stayID string, at time.Time, by string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("store guest id record", sqlcgen.New(t).EnsureGuestRecord(ctx, sqlcgen.EnsureGuestRecordParams{
		TenantID: t.tenant, StayID: stayID, At: pgtype.Timestamptz{Time: at, Valid: true}, By: optText(by)}))
}

func (GuestIDRepo) Record(ctx context.Context, tx app.Tx, stayID string) (app.GuestIDRow, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return app.GuestIDRow{}, false, err
	}
	q := sqlcgen.New(t)
	r, err := q.GetGuestIDRow(ctx, sqlcgen.GetGuestIDRowParams{TenantID: t.tenant, StayID: stayID})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.GuestIDRow{}, false, nil
	}
	if err != nil {
		return app.GuestIDRow{}, false, wrap("select guest id", err)
	}
	photos, err := q.ListGuestPhotoMeta(ctx, sqlcgen.ListGuestPhotoMetaParams{TenantID: t.tenant, StayID: stayID})
	if err != nil {
		return app.GuestIDRow{}, false, wrap("list guest id photos", err)
	}
	out := app.GuestIDRow{NumberEnc: r.NumberEnc, CollectedAt: r.CollectedAt.Time}
	for _, p := range photos {
		out.Photos = append(out.Photos, app.GuestPhotoMeta{Side: p.Side, Bytes: int(p.Bytes), UploadedAt: p.UploadedAt.Time, UploadedBy: p.UploadedBy})
	}
	return out, true, nil
}

// Indicators is the batched read for lists: yes/no flags for the given stays in one query.
func (GuestIDRepo) Indicators(ctx context.Context, tx app.Tx, stayIDs []string) (map[string]app.GuestIDIndicators, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(t).GuestIDIndicatorsFor(ctx, sqlcgen.GuestIDIndicatorsForParams{TenantID: t.tenant, StayIds: stayIDs})
	if err != nil {
		return nil, wrap("select guest id indicators", err)
	}
	out := make(map[string]app.GuestIDIndicators, len(rows))
	for _, r := range rows {
		out[r.StayID] = app.GuestIDIndicators{HasIDNumber: r.HasNumber, HasFrontPhoto: r.HasFront, HasBackPhoto: r.HasBack}
	}
	return out, nil
}

func (GuestIDRepo) ClearNumber(ctx context.Context, tx app.Tx, stayID string) (bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return false, err
	}
	n, err := sqlcgen.New(t).ClearGuestNumber(ctx, sqlcgen.ClearGuestNumberParams{TenantID: t.tenant, StayID: stayID})
	return n > 0, wrap("clear guest id number", err)
}

func (GuestIDRepo) PutPhoto(ctx context.Context, tx app.Tx, stayID, side string, enc []byte, size int, at time.Time, by string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	return wrap("store guest id photo", sqlcgen.New(t).UpsertGuestPhoto(ctx, sqlcgen.UpsertGuestPhotoParams{TenantID: t.tenant, StayID: stayID,
		Side: side, ImageEnc: enc, Bytes: int32(size), UploadedAt: pgtype.Timestamptz{Time: at, Valid: true}, UploadedBy: optText(by)})) //nolint:gosec // bounded by the 5 MB upload cap
}

func (GuestIDRepo) Photo(ctx context.Context, tx app.Tx, stayID, side string) ([]byte, bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, false, err
	}
	b, err := sqlcgen.New(t).GetGuestPhotoEnc(ctx, sqlcgen.GetGuestPhotoEncParams{TenantID: t.tenant, StayID: stayID, Side: side})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	return b, err == nil, wrap("select guest id photo", err)
}

func (GuestIDRepo) DeletePhoto(ctx context.Context, tx app.Tx, stayID, side string) (bool, error) {
	t, err := pgTx(tx)
	if err != nil {
		return false, err
	}
	n, err := sqlcgen.New(t).DeleteGuestPhoto(ctx, sqlcgen.DeleteGuestPhotoParams{TenantID: t.tenant, StayID: stayID, Side: side})
	return n > 0, wrap("delete guest id photo", err)
}

func (GuestIDRepo) RetentionDays(ctx context.Context, tx app.Tx) (int, error) {
	t, err := pgTx(tx)
	if err != nil {
		return 0, err
	}
	n, err := sqlcgen.New(t).IdRetentionDays(ctx, t.tenant)
	if errors.Is(err, pgx.ErrNoRows) {
		return 30, nil
	}
	return int(n), wrap("select retention days", err)
}

func (GuestIDRepo) Expired(ctx context.Context, tx app.Tx, cutoff time.Time) ([]string, error) {
	t, err := pgTx(tx)
	if err != nil {
		return nil, err
	}
	ids, err := sqlcgen.New(t).ExpiredGuestIDStays(ctx, sqlcgen.ExpiredGuestIDStaysParams{TenantID: t.tenant, Cutoff: pgtype.Timestamptz{Time: cutoff, Valid: true}})
	return ids, wrap("select expired guest ids", err)
}

func (GuestIDRepo) DeleteAll(ctx context.Context, tx app.Tx, stayID string) error {
	t, err := pgTx(tx)
	if err != nil {
		return err
	}
	q := sqlcgen.New(t)
	if err = q.DeleteGuestPhotosOfStay(ctx, sqlcgen.DeleteGuestPhotosOfStayParams{TenantID: t.tenant, StayID: stayID}); err != nil {
		return wrap("delete guest id photos", err)
	}
	return wrap("delete guest id", q.DeleteGuestIDRow(ctx, sqlcgen.DeleteGuestIDRowParams{TenantID: t.tenant, StayID: stayID}))
}

// TenantIDs lists every tenant id through app.job_tenant_ids(), the one narrow cross-tenant read, so a job can open a
// normal tenant-scoped transaction per tenant. It returns ids only.
func TenantIDs(ctx context.Context, pool interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}) ([]string, error) {
	rows, err := pool.Query(ctx, `SELECT app.job_tenant_ids()`)
	if err != nil {
		return nil, wrap("list tenants for jobs", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, wrap("scan tenant", err)
		}
		out = append(out, id)
	}
	return out, wrap("read tenants", rows.Err())
}
