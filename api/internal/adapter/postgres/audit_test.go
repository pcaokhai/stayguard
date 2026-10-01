//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/pcaokhai/stayguard/api/internal/app"
)

const (
	pgRestrictViolation     = "23001" // raised by the append-only trigger
	pgInsufficientPrivilege = "42501"
)

func pgCode(err error) string {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

func TestAuditAppendOnly_SG003_AC6(t *testing.T) {
	db := migratedDB(t)
	seedTenant(t, db, "tnt_ac6")
	uow, audit := NewUnitOfWork(newAppPool(t, db, 2)), NewAuditWriter()
	ctx := context.Background()

	err := uow.Do(ctx, "tnt_ac6", func(ctx context.Context, tx app.Tx) error {
		return audit.Append(ctx, tx, app.AuditEntry{
			ID: "aud_1", Action: "stay.checkin", EntityType: "stay", EntityID: "stay_1",
			After: []byte(`{"status":"IN_HOUSE"}`),
		})
	})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	var n int
	owner := connAs(t, db, "owner")
	if err := owner.QueryRow(ctx, `SELECT count(*) FROM app.audit_logs WHERE tenant_id = 'tnt_ac6' AND id = 'aud_1'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("row not stored under the tx tenant: n=%d err=%v", n, err)
	}

	stmts := []string{
		`UPDATE app.audit_logs SET action = 'x'`,
		`DELETE FROM app.audit_logs`,
		`TRUNCATE app.audit_logs`,
	}
	// The owner is a superuser: only the trigger stops it (grants and RLS do not apply).
	for _, s := range stmts {
		if _, err := owner.Exec(ctx, s); pgCode(err) != pgRestrictViolation {
			t.Errorf("owner %q: want trigger error %s, got %v", s, pgRestrictViolation, err)
		}
	}
	appConn := connAs(t, db, appRole)
	for _, s := range stmts {
		if _, err := appConn.Exec(ctx, s); pgCode(err) != pgInsufficientPrivilege {
			t.Errorf("app role %q: want privilege error %s, got %v", s, pgInsufficientPrivilege, err)
		}
	}
}
