//go:build integration

package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

const (
	billTenantA = "tnt_bl_a"
	billTenantB = "tnt_bl_b"
)

var billOutAt = time.Date(2026, 10, 2, 6, 0, 0, 0, time.UTC)

func TestBillingRepo_SG205_AC3(t *testing.T) {
	ctx := context.Background()
	db := migratedDB(t)
	seedRoomTenant(t, db, billTenantA)
	seedRoomTenant(t, db, billTenantB)
	owner := connAs(t, db, "owner")
	seedServices(t, owner, billTenantA)
	// An ACTIVE stay that stays untouched: the foreign tenant must not be able to check it out.
	mustExec(t, owner, `INSERT INTO app.stays (id, tenant_id, unit_id, rental_type, status, guest_name, guest_phone, check_in_at, rate_plan_snapshot, rate_plan_schema)
		VALUES ($1||'_s3', $1, $1||'_u4', 'DAILY', 'ACTIVE', 'Guest Three', '0900000003', '2026-10-01T01:30:00Z', '{}', 1)`, billTenantA)
	uow := NewUnitOfWork(newAppPool(t, db, 4))
	repo := BillingRepo{}
	stayID := billTenantA + "_s1" // ACTIVE on A101

	err := inTx(ctx, t, uow, billTenantA, func(tx app.Tx) error {
		rec, ok, err := repo.LockStay(ctx, tx, stayID)
		if err != nil || !ok || rec.Status != "ACTIVE" || rec.RoomCode != "A101" || len(rec.Extras) != 0 {
			t.Fatalf("lock = %+v ok=%v err=%v", rec, ok, err)
		}
		x := app.NewExtra{ID: "sx1", StayID: stayID, ServiceID: billTenantA + "_w", Quantity: 2, UnitAmount: 10000, Amount: 20000, CreatedAt: billOutAt}
		if err := repo.InsertExtra(ctx, tx, x); err != nil {
			t.Fatalf("insert extra: %v", err)
		}
		rec, _, _ = repo.LockStay(ctx, tx, stayID)
		if len(rec.Extras) != 1 || rec.Extras[0].ServiceCode != "WATER" || rec.Extras[0].Quantity != 2 || rec.Extras[0].UnitAmount != 10000 {
			t.Errorf("extras = %+v", rec.Extras)
		}
		inv := app.NewInvoice{ID: "iv1", StayID: stayID, BillCode: "PH1002A101", Quote: []byte(`{"total":1}`), Total: 1, CreatedAt: billOutAt}
		if err := repo.InsertInvoice(ctx, tx, inv); err != nil {
			t.Fatalf("insert invoice: %v", err)
		}
		return repo.MarkCheckedOut(ctx, tx, stayID, billOutAt)
	})
	if err != nil {
		t.Fatalf("check-out: %v", err)
	}
	var createdAt time.Time
	if err := owner.QueryRow(ctx, `SELECT created_at FROM app.invoices WHERE id = 'iv1'`).Scan(&createdAt); err != nil || !createdAt.Equal(billOutAt) {
		t.Errorf("invoice created_at = %v err=%v, want the argument", createdAt, err)
	}

	_ = inTx(ctx, t, uow, billTenantA, func(tx app.Tx) error {
		if err := repo.MarkCheckedOut(ctx, tx, stayID, billOutAt); !errors.Is(err, stay.ErrNotActive) {
			t.Errorf("second check-out = %v, want ErrNotActive", err)
		}
		rec, _, _ := repo.StayByID(ctx, tx, stayID)
		if rec.Status != "CHECKED_OUT" || rec.CheckOutAt == nil || !rec.CheckOutAt.Equal(billOutAt) {
			t.Errorf("stay = %+v", rec)
		}
		inv, ok, err := repo.InvoiceByStay(ctx, tx, stayID)
		if err != nil || !ok || inv.BillCode != "PH1002A101" || inv.Status != "OPEN" || inv.Total != 1 {
			t.Errorf("invoice = %+v ok=%v err=%v", inv, ok, err)
		}
		if taken, err := repo.BillCodeTaken(ctx, tx, "PH1002A101"); err != nil || !taken {
			t.Errorf("taken = %v err=%v", taken, err)
		}
		if taken, err := repo.BillCodeTaken(ctx, tx, "PH1002A10"); err != nil || taken {
			t.Errorf("prefix must not match: %v err=%v", taken, err)
		}
		return nil
	})
	assertInvoiceUniqueness(ctx, t, uow)

	_ = inTx(ctx, t, uow, billTenantB, func(tx app.Tx) error {
		assertBillingInvisible(ctx, t, repo, tx)
		return nil
	})
	otx, err := owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = otx.Rollback(ctx) }()
	assertBillingInvisible(ctx, t, repo, Tx{tx: pgx.Tx(otx), tenant: billTenantB})
}

// assertInvoiceUniqueness: a second invoice for one stay and a reused bill code both fail, carrying only
// SQLSTATE and constraint name; only the bill code case is the retryable app.ErrBillCodeConflict.
func assertInvoiceUniqueness(ctx context.Context, t *testing.T, uow *UnitOfWork) {
	t.Helper()
	repo := BillingRepo{}
	dupStay := app.NewInvoice{ID: "iv2", StayID: billTenantA + "_s1", BillCode: "PHSECRETCODE", Quote: []byte(`{}`), CreatedAt: billOutAt}
	err := inTx(ctx, t, uow, billTenantA, func(tx app.Tx) error { return repo.InsertInvoice(ctx, tx, dupStay) })
	if err == nil || errors.Is(err, app.ErrBillCodeConflict) || !strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "PHSECRETCODE") {
		t.Errorf("second invoice for a stay = %v", err)
	}
	dupCode := app.NewInvoice{ID: "iv3", StayID: billTenantA + "_s2", BillCode: "PH1002A101", Quote: []byte(`{}`), CreatedAt: billOutAt}
	err = inTx(ctx, t, uow, billTenantA, func(tx app.Tx) error { return repo.InsertInvoice(ctx, tx, dupCode) })
	if !errors.Is(err, app.ErrBillCodeConflict) || !strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "PH1002A101") {
		t.Errorf("reused bill code = %v", err)
	}
}

func assertBillingInvisible(ctx context.Context, t *testing.T, repo BillingRepo, tx app.Tx) {
	t.Helper()
	stayID := billTenantA + "_s1"
	if _, ok, err := repo.LockStay(ctx, tx, stayID); ok || err != nil {
		t.Errorf("foreign stay locked: ok=%v err=%v", ok, err)
	}
	if _, ok, err := repo.InvoiceByStay(ctx, tx, stayID); ok || err != nil {
		t.Errorf("foreign invoice visible: ok=%v err=%v", ok, err)
	}
	if taken, err := repo.BillCodeTaken(ctx, tx, "PH1002A101"); taken || err != nil {
		t.Errorf("foreign bill code visible: %v err=%v", taken, err)
	}
	if err := repo.MarkCheckedOut(ctx, tx, billTenantA+"_s3", billOutAt); !errors.Is(err, stay.ErrNotActive) {
		t.Errorf("foreign mark = %v", err)
	}
}
