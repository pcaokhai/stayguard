//go:build integration

package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/adapter/crypto"
	"github.com/pcaokhai/stayguard/api/internal/adapter/ids"
	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

func newSeededEnv(t *testing.T) *env {
	t.Helper()
	enc, err := crypto.NewAESGCM(testDataKey)
	if err != nil {
		t.Fatal(err)
	}
	return newEnvWith(t, newRooms, app.NewDemoSeeder(postgres.DemoSeedRepo{}, enc, ids.New(time.Now)))
}

func TestDemoSeed_A1(t *testing.T) {
	e := newSeededEnv(t)
	a, b := e.demo("OWNER", "vi", ""), e.demo("OWNER", "vi", "")
	ta, tb := a.str("tenantId"), b.str("tenantId")

	for _, tenant := range []string{ta, tb} {
		counts := map[string]int{}
		for _, st := range []string{"VACANT", "OCCUPIED", "TO_CLEAN", "MAINTENANCE"} {
			counts[st] = e.count(`SELECT count(*) FROM app.units WHERE tenant_id = $1 AND status = $2`, tenant, st)
		}
		want := map[string]int{"VACANT": 19, "OCCUPIED": 11, "TO_CLEAN": 4, "MAINTENANCE": 1}
		for st, n := range want {
			if counts[st] != n {
				t.Errorf("%s: %d rooms %s, want %d", tenant, counts[st], st, n)
			}
		}
		if n := e.count(`SELECT count(*) FROM app.units WHERE tenant_id = $1`, tenant); n != 35 {
			t.Errorf("%s: %d rooms, want 35", tenant, n)
		}
		if n := e.count(`SELECT count(*) FROM app.stays WHERE tenant_id = $1 AND status = 'ACTIVE'`, tenant); n != 11 {
			t.Errorf("%s: %d active stays, want 11", tenant, n)
		}
		if n := e.count(`SELECT count(*) FROM app.bank_accounts WHERE tenant_id = $1 AND is_default AND sepay_status = 'CONNECTED'`, tenant); n != 1 {
			t.Errorf("%s: bank account not stored", tenant)
		}
	}

	// The API sees 2 buildings and 5 services, and tenant B's buildings are not tenant A's.
	ids := func(r reply) map[string]bool {
		out := map[string]bool{}
		items, _ := r.body["items"].([]any)
		for _, it := range items {
			m, _ := it.(map[string]any)
			out[m["id"].(string)] = true
		}
		return out
	}
	ba := e.call("GET", "/v1/buildings", a.str("accessToken"), nil)
	bb := e.call("GET", "/v1/buildings", b.str("accessToken"), nil)
	ia, ib := ids(ba), ids(bb)
	if len(ia) != 2 || len(ib) != 2 {
		t.Fatalf("buildings: a=%v b=%v", ba.body, bb.body)
	}
	for id := range ia {
		if ib[id] {
			t.Errorf("building %s visible to both tenants", id)
		}
	}
	rec := e.demo("RECEPTIONIST", "vi", ta).str("accessToken")
	if rb := e.call("GET", "/v1/buildings", rec, nil); len(ids(rb)) != 2 {
		t.Errorf("receptionist sees %v, want both buildings", rb.body)
	}
	svc := e.call("GET", "/v1/services", a.str("accessToken"), nil)
	if items, _ := svc.body["items"].([]any); len(items) != 5 {
		t.Errorf("services: %d, want 5 (status %d)", len(items), svc.status)
	}
}

// The embedded copy must equal the contract fixture it is copied from.
func TestDemoSeedCopyMatchesContract_A1(t *testing.T) {
	want, err := os.ReadFile("../../../contracts/fixtures/demo-tenant-seed.json")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("../../internal/app/demo-tenant-seed.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatal("api/internal/app/demo-tenant-seed.json differs from contracts/fixtures/demo-tenant-seed.json")
	}
}

// A101 must check out with a balance, so the demo script reaches the QR step.
func TestDemoSeedA101HasBalance_A1(t *testing.T) {
	e := newSeededEnv(t)
	s := e.demo("OWNER", "vi", "")
	tok := s.str("accessToken")
	var stay string
	if err := e.owner.QueryRow(context.Background(), `SELECT s.id FROM app.stays s JOIN app.units u ON u.id = s.unit_id
		WHERE s.tenant_id = $1 AND u.code = 'A101'`, s.str("tenantId")).Scan(&stay); err != nil {
		t.Fatal(err)
	}
	st, raw := e.checkout(tok, stay, newKey())
	q, _ := parse(raw)["quote"].(map[string]any)
	if st != 201 || q["total"] != float64(140000) || q["balanceDue"] != float64(40000) {
		t.Fatalf("checkout: %d %s", st, raw)
	}
	inv, _ := parse(raw)["id"].(string)
	if st, raw := e.send("POST", "/v1/invoices/"+inv+"/payments", tok, newKey(), map[string]any{"method": "TRANSFER"}); st != 201 {
		t.Fatalf("transfer: %d %s", st, raw)
	}
}
