package app

import "testing"

func TestAuditDetailsOf_AddsRoomAndBillWithoutOverwriting_Audit(t *testing.T) {
	got := AuditDetailsOf(AuditRow{After: []byte(`{"stayId":"st_1","room":"B2"}`), Room: "A1", Bill: "PH1A1"})
	if got["room"] != "B2" || got["bill"] != "PH1A1" || got["stayId"] != "st_1" {
		t.Fatalf("details: %v", got)
	}
	got = AuditDetailsOf(AuditRow{After: nil, Room: "A1"})
	if got["room"] != "A1" || len(got) != 1 {
		t.Fatalf("details without a document: %v", got)
	}
	if got = AuditDetailsOf(AuditRow{After: []byte(`{"a":1}`)}); len(got) != 1 {
		t.Fatalf("empty room and bill must add nothing: %v", got)
	}
}
