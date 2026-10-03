//go:build integration

package main

import (
	"testing"
	"time"
)

// TD-03: marking an alert read shows in the alert list (read, readAt, readBy) and in the overview's unread count.
func TestAlertRead_ShowsInTheListAndTheOverviewCount_FU(t *testing.T) {
	r := deskRig(t)
	owner := r.ownerToken()
	_, at := r.invoiceTotalAndCreated()
	r.runJobsAt(at.Add(30 * time.Minute)) // one PAYMENT_UNPAID alert

	list := func() map[string]any {
		st, raw := r.e.send("GET", "/v1/owner/alerts?kind=PAYMENT_UNPAID", owner, "", nil)
		items, _ := parse(raw)["items"].([]any)
		if st != 200 || len(items) != 1 {
			t.Fatalf("alerts: %d %s", st, raw)
		}
		return items[0].(map[string]any)
	}
	unread := func() int64 {
		st, raw := r.e.send("GET", "/v1/owner/overview", owner, "", nil)
		if st != 200 {
			t.Fatalf("overview: %d %s", st, raw)
		}
		return num(parse(raw)["unreadAlerts"])
	}
	a := list()
	if a["read"] != false || a["readAt"] != nil || a["readBy"] != nil || unread() != 1 {
		t.Fatalf("a new alert is unread (count %d): %v", unread(), a)
	}
	r.e.clock.set(at.Add(45 * time.Minute))
	if st, raw := r.e.send("POST", "/v1/owner/alerts/"+a["id"].(string)+"/read", owner, "", nil); st != 204 {
		t.Fatalf("markAlertRead: %d %s", st, raw)
	}
	a = list()
	readAt, _ := time.Parse(time.RFC3339, toString(a["readAt"]))
	if a["read"] != true || a["readBy"] == nil || a["readBy"] == "" || !readAt.Equal(at.Add(45*time.Minute).UTC()) {
		t.Fatalf("the list shows who read it and when: %v", a)
	}
	if unread() != 0 {
		t.Fatalf("the overview still counts it as unread: %d", unread())
	}
	// Reading it again keeps the first reader and time.
	r.e.clock.set(at.Add(2 * time.Hour))
	if st, _ := r.e.send("POST", "/v1/owner/alerts/"+a["id"].(string)+"/read", owner, "", nil); st != 204 {
		t.Fatal("a second read must still succeed")
	}
	if again := list(); again["readAt"] != a["readAt"] {
		t.Fatalf("the first read time is kept: %v vs %v", again["readAt"], a["readAt"])
	}
}

func toString(v any) string {
	s, _ := v.(string)
	return s
}
