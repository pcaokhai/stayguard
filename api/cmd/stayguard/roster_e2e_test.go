//go:build integration

package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

// F-A3 over HTTP: the owner rosters a receptionist, copies the week, the receptionist asks for leave and the owner decides,
// and the shift that opens on the receptionist's first cash action carries the rostered shift.
func TestRosterE2E_FA3(t *testing.T) {
	e := newEnv(t)
	owner := e.demo("OWNER", "vi", "")
	tenant, boss := owner.str("tenantId"), owner.str("accessToken")
	desk := e.demo("RECEPTIONIST", "vi", tenant).str("accessToken")
	e.seedStayTenant(tenant, 1)
	e.exec(`INSERT INTO app.building_permissions (tenant_id, user_id, building_id, level)
		SELECT tenant_id, id, $2, 'EDIT' FROM app.users WHERE tenant_id = $1 AND role = 'RECEPTIONIST'`, tenant, stayBuilding)
	e.clock.set(e.start)
	ctx := context.Background()
	var lan string
	if err := e.owner.QueryRow(ctx, `SELECT id FROM app.users WHERE tenant_id = $1 AND role = 'RECEPTIONIST'`, tenant).Scan(&lan); err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	today := e.start.In(loc)
	ymd := func(d time.Time) string { return d.Format("2006-01-02") }
	monday := today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7)) // this week's Monday
	nextMonday := monday.AddDate(0, 0, 7)

	// Roster the receptionist on the afternoon of today and the morning of the day after; the rest are gaps.
	put := map[string]any{"set": []map[string]any{
		{"userId": lan, "date": ymd(today), "shift": "AFTERNOON"},
		{"userId": lan, "date": ymd(today.AddDate(0, 0, 1)), "shift": "MORNING"},
	}, "remove": []any{}}
	st, raw := e.send("PUT", "/v1/owner/roster", boss, newKey(), put)
	roster := parse(raw)
	if st != 200 || len(roster["assignments"].([]any)) != 2 || len(roster["gaps"].([]any)) == 0 ||
		roster["assignments"].([]any)[0].(map[string]any)["userName"] == nil {
		t.Fatalf("put roster: %d %s", st, raw)
	}
	if st, _ = e.send("PUT", "/v1/owner/roster", desk, newKey(), put); st != 403 {
		t.Fatalf("a receptionist edits the roster: %d", st)
	}
	bad := map[string]any{"set": []map[string]any{{"userId": "nobody", "date": ymd(today), "shift": "NIGHT"}}, "remove": []any{}}
	if st, _ = e.send("PUT", "/v1/owner/roster", boss, newKey(), bad); st != 422 {
		t.Fatalf("unknown person: %d", st)
	}
	st, raw = e.send("GET", "/v1/me/roster?from="+ymd(today)+"&to="+ymd(today.AddDate(0, 0, 6)), desk, "", nil)
	if mine := parse(raw); st != 200 || len(mine["assignments"].([]any)) != 2 || mine["gaps"] != nil {
		t.Fatalf("my roster: %d %s", st, raw)
	}

	// The first cash action opens a shift that carries the rostered shift (the hour says MORNING at 08:00, the roster says AFTERNOON).
	if st, raw = e.checkIn(desk, 1, newKey(), stayBody(map[string]any{"deposit": 50000})); st != 201 {
		t.Fatalf("check-in: %d %s", st, raw)
	}
	var code string
	if err := e.owner.QueryRow(ctx, `SELECT shift_code FROM app.shifts WHERE tenant_id = $1`, tenant).Scan(&code); err != nil || code != "AFTERNOON" {
		t.Fatalf("shift code %q: %v", code, err)
	}

	// Copy this week into next week once; a second copy is refused.
	st, raw = e.send("POST", "/v1/owner/roster/copy-week", boss, newKey(), map[string]any{"weekStart": ymd(nextMonday)})
	if st != 200 {
		t.Fatalf("copy week: %d %s", st, raw)
	}
	if st, raw = e.send("POST", "/v1/owner/roster/copy-week", boss, newKey(), map[string]any{"weekStart": ymd(nextMonday)}); st != 409 || parse(raw)["code"] != "ROSTER_NOT_EMPTY" {
		t.Fatalf("copy again: %d %s", st, raw)
	}

	// Leave: request, overlap, the owner sees it as needing attention, approves, and the roster refuses to place the person that day.
	leaveDay := nextMonday.AddDate(0, 0, 5)
	req := map[string]any{"fromDate": ymd(leaveDay), "toDate": ymd(leaveDay.AddDate(0, 0, 1)), "kind": "PAID", "reason": "family"}
	st, raw = e.send("POST", "/v1/me/leave-requests", desk, newKey(), req)
	leave := parse(raw)
	id, _ := leave["id"].(string)
	if st != 201 || leave["status"] != "PENDING" || leave["userName"] == "" {
		t.Fatalf("request leave: %d %s", st, raw)
	}
	if st, _ = e.send("POST", "/v1/me/leave-requests", desk, newKey(), req); st != 409 {
		t.Fatalf("overlapping leave: %d", st)
	}
	st, raw = e.send("GET", "/v1/owner/overview", boss, "", nil)
	kinds := ""
	for _, a := range parse(raw)["attention"].([]any) {
		kinds += a.(map[string]any)["kind"].(string) + " "
	}
	if st != 200 || !strings.Contains(kinds, "LEAVE_PENDING") {
		t.Fatalf("overview attention: %d %s", st, kinds)
	}
	if st, _ = e.send("POST", "/v1/owner/leave-requests/"+id+"/approve", desk, newKey(), nil); st != 403 {
		t.Fatalf("a receptionist approves: %d", st)
	}
	if st, raw = e.send("POST", "/v1/owner/leave-requests/"+id+"/approve", boss, newKey(), nil); st != 200 || parse(raw)["status"] != "APPROVED" {
		t.Fatalf("approve: %d %s", st, raw)
	}
	onLeave := map[string]any{"set": []map[string]any{{"userId": lan, "date": ymd(leaveDay), "shift": "MORNING"}}, "remove": []any{}}
	if st, raw = e.send("PUT", "/v1/owner/roster", boss, newKey(), onLeave); st != 409 || parse(raw)["code"] != "LEAVE_CONFLICT" {
		t.Fatalf("rostering a person on leave: %d %s", st, raw)
	}
	st, raw = e.send("GET", "/v1/me/leave-requests", desk, "", nil)
	if mine := parse(raw); st != 200 || len(mine["items"].([]any)) != 1 || mine["balance"].(map[string]any)["used"] != float64(2) {
		t.Fatalf("my leave and balance: %d %s", st, raw)
	}
	if st, raw = e.send("POST", "/v1/me/leave-requests/"+id+"/cancel", desk, newKey(), nil); st != 200 || parse(raw)["status"] != "CANCEL_REQUESTED" {
		t.Fatalf("ask to cancel: %d %s", st, raw)
	}
	st, raw = e.send("GET", "/v1/owner/leave-requests?status=CANCEL_REQUESTED", boss, "", nil)
	if items, _ := parse(raw)["items"].([]any); st != 200 || len(items) != 1 {
		t.Fatalf("leave to decide: %d %s", st, raw)
	}
}
