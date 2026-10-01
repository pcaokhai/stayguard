//go:build integration

package postgres

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

const (
	svcTenantA = "tnt_sv_a"
	svcTenantB = "tnt_sv_b"
)

func seedServices(t *testing.T, owner *pgx.Conn, tenant string) {
	t.Helper()
	mustExec(t, owner, `INSERT INTO app.services (id, tenant_id, code, name, price, stock) VALUES
		($1||'_w', $1, 'WATER', '{"vi":"Nuoc","en":"Water"}', 10000, 3),
		($1||'_b', $1, 'BEER', '{"vi":"Bia","en":"Beer"}', 25000, 5),
		($1||'_t', $1, 'TOWEL', '{"vi":"Khan","en":"Towel"}', 5000, 0)`, tenant)
}

func TestServicesRepo_SG205_AC1(t *testing.T) {
	ctx := context.Background()
	db := migratedDB(t)
	seedRoomTenant(t, db, svcTenantA)
	seedRoomTenant(t, db, svcTenantB)
	owner := connAs(t, db, "owner")
	seedServices(t, owner, svcTenantA)
	uow := NewUnitOfWork(newAppPool(t, db, 24))
	repo := ServiceRepo{}

	_ = inTx(ctx, t, uow, svcTenantA, func(tx app.Tx) error {
		list, err := repo.List(ctx, tx)
		if err != nil || len(list) != 3 || list[0].Code != "BEER" || list[1].Code != "TOWEL" || list[2].Code != "WATER" {
			t.Fatalf("list = %+v err=%v", list, err)
		}
		if list[0].Name != (app.LocalizedName{VI: "Bia", EN: "Beer"}) || list[0].Price != 25000 || list[0].Stock != 5 {
			t.Errorf("beer = %+v", list[0])
		}
		rows, err := repo.ByCodes(ctx, tx, []string{"WATER", "NOPE", "BEER"})
		if err != nil || len(rows) != 2 || rows[0].Code != "BEER" || rows[0].ID != svcTenantA+"_b" || rows[1].Code != "WATER" {
			t.Errorf("by codes = %+v err=%v", rows, err)
		}
		return nil
	})

	// Guarded decrement: exactly the stock, then ErrInsufficientStock; the price comes back.
	err := inTx(ctx, t, uow, svcTenantA, func(tx app.Tx) error {
		if p, err := repo.DecrementStock(ctx, tx, svcTenantA+"_w", 3); err != nil || p != 10000 {
			t.Errorf("take all = %d, %v", p, err)
		}
		if _, err := repo.DecrementStock(ctx, tx, svcTenantA+"_w", 1); !errors.Is(err, stay.ErrInsufficientStock) {
			t.Errorf("past zero = %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	assertStockRace(ctx, t, uow, owner)

	// Tenant B's tx cannot see, decrement or look up tenant A's services; neither can an RLS-bypassing owner tx.
	_ = inTx(ctx, t, uow, svcTenantB, func(tx app.Tx) error {
		assertServicesInvisible(ctx, t, repo, tx)
		return nil
	})
	otx, err := owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = otx.Rollback(ctx) }()
	assertServicesInvisible(ctx, t, repo, Tx{tx: pgx.Tx(otx), tenant: svcTenantB})
}

// assertStockRace: 20 goroutines take 1 from a stock of 5, exactly 5 win and the stock ends at 0.
func assertStockRace(ctx context.Context, t *testing.T, uow *UnitOfWork, owner *pgx.Conn) {
	t.Helper()
	var won, lost atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := uow.Do(ctx, svcTenantA, func(ctx context.Context, tx app.Tx) error {
				_, err := ServiceRepo{}.DecrementStock(ctx, tx, svcTenantA+"_b", 1)
				return err
			})
			switch {
			case err == nil:
				won.Add(1)
			case errors.Is(err, stay.ErrInsufficientStock):
				lost.Add(1)
			default:
				t.Errorf("racer: %v", err)
			}
		}()
	}
	wg.Wait()
	var stock int64
	if err := owner.QueryRow(ctx, `SELECT stock FROM app.services WHERE id = $1`, svcTenantA+"_b").Scan(&stock); err != nil {
		t.Fatal(err)
	}
	if won.Load() != 5 || lost.Load() != 15 || stock != 0 {
		t.Errorf("won=%d lost=%d stock=%d, want 5, 15, 0", won.Load(), lost.Load(), stock)
	}
}

func assertServicesInvisible(ctx context.Context, t *testing.T, repo ServiceRepo, tx app.Tx) {
	t.Helper()
	if list, err := repo.List(ctx, tx); err != nil || len(list) != 0 {
		t.Errorf("foreign list = %+v err=%v", list, err)
	}
	if rows, err := repo.ByCodes(ctx, tx, []string{"WATER"}); err != nil || len(rows) != 0 {
		t.Errorf("foreign by codes = %+v err=%v", rows, err)
	}
	if _, err := repo.DecrementStock(ctx, tx, svcTenantA+"_t", 1); !errors.Is(err, stay.ErrInsufficientStock) {
		t.Errorf("foreign decrement = %v", err)
	}
}
