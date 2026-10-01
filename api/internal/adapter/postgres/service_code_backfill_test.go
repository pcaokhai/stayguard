//go:build integration

package postgres

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pressly/goose/v3"

	"github.com/pcaokhai/stayguard/api/migrations"
)

const preServiceCodeVersion, serviceCodeVersion = 6, 7

func TestServiceCodeBackfill_SG205_AC1(t *testing.T) {
	ctx := context.Background()
	db := newEmptyDB(t)
	sqlDB, err := sql.Open("pgx", urlFor(db, "owner", testSecret))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sqlDB.Close() }()
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.UpTo(ctx, preServiceCodeVersion); err != nil {
		t.Fatalf("up to 6: %v", err)
	}
	owner := connAs(t, db, "owner")
	for _, tn := range []string{"tn_a", "tn_b"} {
		mustExec(t, owner, `INSERT INTO app.tenants (id, name) VALUES ($1, 'n')`, tn)
	}
	rows := []struct{ id, tenant, name string }{
		{"s1", "tn_a", `{"en":"Bottled Water"}`},
		{"s2", "tn_a", `{"en":"  Towel!! "}`},
		{"s3", "tn_a", `{}`},
		{"s4", "tn_a", `{"en":"!!!"}`},
		{"s5", "tn_b", `{"en":"Bottled Water"}`},
	}
	for _, r := range rows {
		mustExec(t, owner, `INSERT INTO app.services (id, tenant_id, name, price) VALUES ($1, $2, $3::jsonb, 1)`, r.id, r.tenant, r.name)
	}
	if _, err := p.UpTo(ctx, serviceCodeVersion); err != nil {
		t.Fatalf("up to 7: %v", err)
	}
	got := queryStrings(t, db, `SELECT id || '=' || code FROM app.services ORDER BY id`)
	want := []string{"s1=BOTTLED_WATER", "s2=TOWEL", "s3=s3", "s4=s4", "s5=BOTTLED_WATER"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("backfilled codes (-want +got):\n%s", diff)
	}
	if _, err := owner.Exec(ctx, `INSERT INTO app.services (id, tenant_id, code, name, price) VALUES ('s6', 'tn_a', 'TOWEL', '{}', 1)`); err == nil {
		t.Fatal("a duplicate code in one tenant must be rejected")
	}
	if _, err := owner.Exec(ctx, `INSERT INTO app.services (id, tenant_id, name, price) VALUES ('s7', 'tn_a', '{}', 1)`); err == nil {
		t.Fatal("a missing code must be rejected")
	}
	nullable := queryStrings(t, db, `SELECT is_nullable FROM information_schema.columns WHERE table_schema = 'app' AND table_name = 'services' AND column_name = 'code'`)
	if diff := cmp.Diff([]string{"NO"}, nullable); diff != "" {
		t.Fatalf("code nullability: %s", diff)
	}
}
