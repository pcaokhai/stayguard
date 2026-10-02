//go:build integration

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestIdNumberE2E_SG203_AC4(t *testing.T) {
	e := newEnv(t)
	a := e.demo("OWNER", "vi", "")
	tenant, token := a.str("tenantId"), a.str("accessToken")
	e.seedStayTenant(tenant, 2)
	body := stayBody(map[string]any{"guestName": nameMarker, "guestPhone": phoneMarker, "idNumber": idMarker})

	var ids []string
	for room := 1; room <= 2; room++ {
		st, raw := e.checkIn(token, room, newKey(), body)
		if st != 201 || !hasIDNumber(raw) {
			t.Fatalf("create: %d %s", st, raw)
		}
		id, _ := parse(raw)["id"].(string)
		ids = append(ids, id)
		gst, graw := e.send("GET", "/v1/stays/"+id, token, "", nil)
		if gst != 200 || !hasIDNumber(graw) {
			t.Fatalf("get: %d %s", gst, graw)
		}
		for _, r := range [][]byte{raw, graw} {
			if bytes.Contains(r, []byte(idMarker)) {
				t.Errorf("response holds the plain id number: %s", r)
			}
		}
	}

	for _, table := range []string{"stays", "guest_ids", "idempotency_keys", "audit_logs"} {
		if dump := e.dumpTable(table); strings.Contains(dump, idMarker) {
			t.Errorf("%s holds the plain id number", table)
		}
	}
	if n := e.count(`SELECT count(*) FROM app.guest_ids WHERE position($1::bytea in number_enc) > 0`, []byte(idMarker)); n != 0 {
		t.Errorf("%d stays hold the id number as raw bytes", n)
	}
	var c1, c2 []byte
	_ = e.owner.QueryRow(context.Background(), `SELECT number_enc FROM app.guest_ids WHERE stay_id = $1`, ids[0]).Scan(&c1)
	_ = e.owner.QueryRow(context.Background(), `SELECT number_enc FROM app.guest_ids WHERE stay_id = $1`, ids[1]).Scan(&c2)
	if len(c1) == 0 || bytes.Equal(c1, c2) {
		t.Errorf("ciphertext of the same id number must differ per stay (len %d)", len(c1))
	}
	// Audit rows hold ids and amounts only.
	audit := e.dumpTable("audit_logs")
	for _, m := range []string{nameMarker, phoneMarker, idMarker} {
		if strings.Contains(audit, m) {
			t.Errorf("audit rows hold personal data %q", m)
		}
	}
}

func TestNoPersonalDataInLogs_SG203_AC4(t *testing.T) {
	e := newEnv(t)
	a := e.demo("OWNER", "vi", "")
	tenant, token := a.str("tenantId"), a.str("accessToken")
	e.seedStayTenant(tenant, 2)
	// A unit type whose stored plan is corrupt: check-in fails on the server side (500).
	e.exec(`INSERT INTO app.unit_types (id, tenant_id, code, name, rate_plan, rate_plan_version) VALUES ('ut_bad', $1, 'BAD', '{"vi":"x","en":"x"}', '{}', 1)`, tenant)
	e.exec(`UPDATE app.units SET unit_type_id = 'ut_bad' WHERE id = $1`, roomID(2))
	personal := map[string]any{"guestName": nameMarker, "guestPhone": phoneMarker, "idNumber": idMarker}

	if st, raw := e.checkIn(token, 1, newKey(), stayBody(personal)); st != 201 {
		t.Fatalf("success path: %d %s", st, raw)
	}
	bad := stayBody(map[string]any{"guestName": nameMarker, "guestPhone": "12", "idNumber": idMarker})
	if st, _ := e.checkIn(token, 1, newKey(), bad); st != 422 {
		t.Fatalf("validation path: %d", st)
	}
	if st, _ := e.checkIn(token, 1, newKey(), stayBody(personal)); st != 409 {
		t.Fatalf("conflict path: %d", st)
	}
	st, raw := e.checkIn(token, 2, newKey(), stayBody(personal))
	if st != 500 || parse(raw)["code"] != "INTERNAL" {
		t.Fatalf("server error path: %d %s", st, raw)
	}

	logs := e.logs.String()
	if !strings.Contains(logs, "unhandled handler error") {
		t.Fatalf("the forced server error was not logged; capture is broken:\n%s", logs)
	}
	for _, m := range []string{idMarker, nameMarker, phoneMarker, "Nguyen"} {
		if strings.Contains(logs, m) {
			t.Errorf("logs contain personal data %q:\n%s", m, logs)
		}
	}
}

// hasIDNumber reads the contract 1.1.0 indicator; the number itself is only served by the owner endpoints (F-A2).
func hasIDNumber(raw []byte) bool {
	g, _ := parse(raw)["guestId"].(map[string]any)
	return g["hasIdNumber"] == true
}
