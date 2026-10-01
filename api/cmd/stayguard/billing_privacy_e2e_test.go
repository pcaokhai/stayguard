//go:build integration

package main

import (
	"strings"
	"testing"
)

// The marker guest data must not reach logs, audit rows, invoices or idempotency rows of any route, except
// the stored 24 h replay body of the extras answer, which is the stay view and holds name and phone by design
// (docs plan SG-203 ruling). The ID number is never stored or logged in plain text anywhere.
func TestNoPersonalDataInLogs_SG205_AC2(t *testing.T) {
	e := newEnv(t)
	a := e.demo("OWNER", "vi", "")
	tenant, token := a.str("tenantId"), a.str("accessToken")
	e.seedStayTenant(tenant, 1)
	e.seedServices(tenant, 1)
	id := e.openStay(token, 1, map[string]any{"guestName": nameMarker, "guestPhone": phoneMarker, "idNumber": idMarker})

	steps := []struct {
		name string
		want int
		do   func() (int, []byte)
	}{
		{"success", 200, func() (int, []byte) { return e.addExtras(token, id, newKey(), extrasBody("WATER", 1)) }},
		{"validation failure", 422, func() (int, []byte) { return e.addExtras(token, id, newKey(), extrasBody("WATER", 0)) }},
		{"stock conflict", 409, func() (int, []byte) { return e.addExtras(token, id, newKey(), extrasBody("WATER", 1)) }},
		{"forced server error", 500, func() (int, []byte) {
			e.exec(`UPDATE app.services SET name = '[]' WHERE tenant_id = $1 AND code = 'BEER'`, tenant) // a name that cannot be decoded
			return e.addExtras(token, id, newKey(), extrasBody("BEER", 1))
		}},
		{"checkout", 201, func() (int, []byte) { return e.checkout(token, id, newKey()) }},
	}
	for _, s := range steps {
		if st, raw := s.do(); st != s.want {
			t.Fatalf("%s: %s", s.name, describe(st, raw))
		}
	}

	markers := []string{nameMarker, phoneMarker, idMarker}
	for _, table := range []string{"audit_logs", "invoices", "stay_extras"} {
		assertNoMarkers(t, table, e.dumpTable(table), markers)
	}
	assertNoMarkers(t, "logs", e.logs.String(), markers)
	// Idempotency rows: the stay view of check-in and extras keeps name and phone by design; the check-out route never holds guest data.
	for _, m := range markers {
		if n := e.count(`SELECT count(*) FROM app.idempotency_keys WHERE route LIKE '%/checkout' AND position($1::bytea in response_body) > 0`, []byte(m)); n != 0 {
			t.Errorf("a check-out idempotency row holds %q", m)
		}
	}
	if n := e.count(`SELECT count(*) FROM app.idempotency_keys WHERE position($1::bytea in response_body) > 0`, []byte(idMarker)); n != 0 {
		t.Error("an idempotency row holds the plain ID number")
	}
	if !strings.Contains(e.logs.String(), `"level":"ERROR"`) {
		t.Error("the forced server error was not logged")
	}
}

func assertNoMarkers(t *testing.T, where, text string, markers []string) {
	t.Helper()
	for _, m := range markers {
		if strings.Contains(text, m) {
			t.Errorf("%s holds personal data %q", where, m)
		}
	}
}
