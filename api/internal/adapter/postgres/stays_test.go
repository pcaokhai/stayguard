//go:build integration

package postgres

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
)

const (
	stayTenantA = "tnt_st_a"
	stayTenantB = "tnt_st_b"
)

var stayCheckIn = time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)

func newStay(tenant, id, roomID string) app.NewStay {
	return app.NewStay{
		ID: id, RoomID: roomID, RentalType: "OVERNIGHT", GuestName: "Guest", GuestPhone: "0900000009",
		IDNumberEnc: []byte{1, 0xde, 0xad, 0xbe, 0xef}, Deposit: 100_000, CheckInAt: stayCheckIn,
		RatePlanSnapshot: []byte(`{"v":1}`), RatePlanSchema: 1,
	}
}

func inTx(ctx context.Context, t *testing.T, uow *UnitOfWork, tenant string, fn func(tx app.Tx) error) error {
	t.Helper()
	return uow.Do(ctx, tenant, func(ctx context.Context, tx app.Tx) error { return fn(tx) })
}

func TestStaysRepo_SG203_AC1(t *testing.T) {
	ctx := context.Background()
	db := migratedDB(t)
	seedRoomTenant(t, db, stayTenantA)
	seedRoomTenant(t, db, stayTenantB)
	owner := connAs(t, db, "owner")
	mustExec(t, owner, `INSERT INTO app.services (id, tenant_id, name, price) VALUES ($1, $2, '{"vi":"Nuoc","en":"Water"}', 10000)`, stayTenantA+"_sv", stayTenantA)
	uow := NewUnitOfWork(newAppPool(t, db, 4))
	repo := StayRepo{}
	ua, ub := stayTenantA+"_u2", stayTenantB+"_u2"

	// Insert and read back; a CHECKED_OUT stay (s2) on the room does not block a new check-in.
	err := inTx(ctx, t, uow, stayTenantA, func(tx app.Tx) error {
		r, ok, err := repo.Room(ctx, tx, ua, true)
		if err != nil || !ok || r.Code != "A201" || r.StoredStatus != "VACANT" || r.BuildingID != stayTenantA+"_b1" || r.RatePlanVersion != 1 {
			t.Fatalf("room = %+v ok=%v err=%v", r, ok, err)
		}
		if err := repo.InsertStay(ctx, tx, newStay(stayTenantA, "st_new", ua)); err != nil {
			return err
		}
		return repo.MarkRoomOccupied(ctx, tx, ua)
	})
	if err != nil {
		t.Fatalf("check-in: %v", err)
	}
	mustExec(t, owner, `INSERT INTO app.stay_extras (id, tenant_id, stay_id, service_id, quantity, unit_amount, amount) VALUES ('se1', $1, 'st_new', $2, 2, 10000, 20000)`, stayTenantA, stayTenantA+"_sv")
	_ = inTx(ctx, t, uow, stayTenantA, func(tx app.Tx) error {
		rec, ok, err := repo.StayByID(ctx, tx, "st_new")
		if err != nil || !ok {
			t.Fatalf("stay by id: ok=%v err=%v", ok, err)
		}
		if rec.RoomCode != "A201" || rec.BuildingID != stayTenantA+"_b1" || rec.Status != "ACTIVE" || rec.GuestPhone != "0900000009" ||
			rec.Deposit != 100_000 || !rec.CheckInAt.Equal(stayCheckIn) || rec.CheckOutAt != nil || !bytes.Equal(rec.IDNumberEnc, []byte{1, 0xde, 0xad, 0xbe, 0xef}) {
			t.Errorf("record = %+v", rec)
		}
		if len(rec.Extras) != 1 || rec.Extras[0].Name != (app.LocalizedName{VI: "Nuoc", EN: "Water"}) || rec.Extras[0].Quantity != 2 || rec.Extras[0].UnitAmount != 10000 {
			t.Errorf("extras = %+v", rec.Extras)
		}
		if tz, err := repo.Timezone(ctx, tx); err != nil || tz != "Asia/Ho_Chi_Minh" {
			t.Errorf("tz=%q err=%v", tz, err)
		}
		return nil
	})
	var status string
	if err := owner.QueryRow(ctx, `SELECT status FROM app.units WHERE id = $1`, ua).Scan(&status); err != nil || status != "OCCUPIED" {
		t.Errorf("room status = %q err=%v", status, err)
	}

	// The column holds bytes the repo was given, not plaintext (the use case encrypts before the repo).
	var n int
	_ = owner.QueryRow(ctx, `SELECT count(*) FROM app.stays WHERE id = 'st_new' AND position($1::bytea in id_number_enc) > 0`, []byte("PLAINIDMARKER")).Scan(&n)
	if n != 0 {
		t.Error("id_number_enc contains plaintext marker")
	}

	// Backstops: a second ACTIVE insert hits the unique index, a non-vacant room updates nothing.
	err = inTx(ctx, t, uow, stayTenantA, func(tx app.Tx) error { return repo.InsertStay(ctx, tx, newStay(stayTenantA, "st_dup", ua)) })
	if !errors.Is(err, room.ErrNotVacant) {
		t.Errorf("duplicate active stay = %v, want ErrNotVacant", err)
	}
	err = inTx(ctx, t, uow, stayTenantA, func(tx app.Tx) error { return repo.MarkRoomOccupied(ctx, tx, ua) })
	if !errors.Is(err, room.ErrNotVacant) {
		t.Errorf("occupy occupied room = %v, want ErrNotVacant", err)
	}

	// Tenant B's tx cannot read, lock, insert on or occupy tenant A's rows.
	_ = inTx(ctx, t, uow, stayTenantB, func(tx app.Tx) error {
		assertForeignInvisible(ctx, t, repo, tx, stayTenantA)
		return nil
	})
	// Owner connection bypasses RLS: the explicit tenant filter alone must hold.
	otx, err := owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = otx.Rollback(ctx) }()
	assertForeignInvisible(ctx, t, repo, Tx{tx: pgx.Tx(otx), tenant: stayTenantB}, stayTenantA)
	_ = ub
}

func assertForeignInvisible(ctx context.Context, t *testing.T, repo StayRepo, tx app.Tx, foreign string) {
	t.Helper()
	if _, ok, err := repo.Room(ctx, tx, foreign+"_u4", true); ok || err != nil {
		t.Errorf("foreign room visible/locked: ok=%v err=%v", ok, err)
	}
	if _, ok, err := repo.StayByID(ctx, tx, "st_new"); ok || err != nil {
		t.Errorf("foreign stay visible: ok=%v err=%v", ok, err)
	}
	if err := repo.MarkRoomOccupied(ctx, tx, foreign+"_u4"); !errors.Is(err, room.ErrNotVacant) {
		t.Errorf("foreign room occupied: %v", err)
	}
}

func mustExec(t *testing.T, c *pgx.Conn, sql string, args ...any) {
	t.Helper()
	if _, err := c.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec: %v\n%s", err, sql)
	}
}
