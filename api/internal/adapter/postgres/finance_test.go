//go:build integration

package postgres

import (
	"context"
	"sync"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/app"
)

// Follow-up 3: many reads of one month at once create the recurring copy once.
func TestRecurringCopy_ConcurrentReadsCreateOneRow_FU3(t *testing.T) {
	ctx := context.Background()
	db := migratedDB(t)
	const tenant = "tnt_rec"
	seedRoomTenant(t, db, tenant)
	owner := connAs(t, db, "owner")
	mustExec(t, owner, `INSERT INTO app.expenses (id, tenant_id, month, category, amount, recurring, source)
		VALUES ('ex_tpl', $1, '2026-08', 'RENT', 5000000, true, 'RECURRING')`, tenant)
	uow := NewUnitOfWork(newAppPool(t, db, 12))

	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- uow.Do(ctx, tenant, func(ctx context.Context, tx app.Tx) error {
				_, err := app.EnsureRecurring(ctx, tx, FinanceRepo{}, "2026-10")
				return err
			})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent read: %v", err)
		}
	}
	var sep, oct int
	_ = owner.QueryRow(ctx, `SELECT count(*) FROM app.expenses WHERE tenant_id = $1 AND month = '2026-09' AND source = 'RECURRING'`, tenant).Scan(&sep)
	_ = owner.QueryRow(ctx, `SELECT count(*) FROM app.expenses WHERE tenant_id = $1 AND month = '2026-10' AND source = 'RECURRING'`, tenant).Scan(&oct)
	if sep != 1 || oct != 1 {
		t.Fatalf("copies: september %d october %d, want one each", sep, oct)
	}
	// The database refuses a second copy of one template in one month, whatever the code does.
	if _, err := owner.Exec(ctx, `INSERT INTO app.expenses (id, tenant_id, month, category, amount, recurring, source, ref_id, root_id)
		VALUES ('ex_dup', $1, '2026-09', 'RENT', 1, true, 'RECURRING', 'other-ref', 'ex_tpl')`, tenant); err == nil {
		t.Fatal("a second copy of the same template in the same month was accepted")
	}
}
