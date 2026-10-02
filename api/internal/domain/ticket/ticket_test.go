package ticket

import "testing"

func TestCanMove_Forward_Only_SG1201(t *testing.T) {
	ok := [][2]string{{New, New}, {New, InRepair}, {New, Done}, {InRepair, InRepair}, {InRepair, Done}, {Done, Done}}
	for _, m := range ok {
		if !CanMove(m[0], m[1]) {
			t.Errorf("%s -> %s refused", m[0], m[1])
		}
	}
	for _, m := range [][2]string{{InRepair, New}, {Done, New}, {Done, InRepair}, {New, "WEIRD"}, {"WEIRD", Done}} {
		if CanMove(m[0], m[1]) {
			t.Errorf("%s -> %s allowed", m[0], m[1])
		}
	}
}

func TestCheckReport_SG1201(t *testing.T) {
	if errs := CheckReport("TV", "remote missing", "LOCK_ROOM"); len(errs) != 0 {
		t.Fatalf("valid: %v", errs)
	}
	got := map[string]bool{}
	for _, e := range CheckReport("SOFA", "  ", "BROKEN") {
		got[e.Path] = true
	}
	if !got["category"] || !got["description"] || !got["severity"] {
		t.Fatalf("errors: %v", got)
	}
}

func TestTotal_SG1201(t *testing.T) {
	n := func(v int64) *int64 { return &v }
	if Total(nil, nil) != nil {
		t.Error("no costs, no total")
	}
	if got := Total(n(100), nil); got == nil || *got != 100 {
		t.Errorf("parts only: %v", got)
	}
	if got := Total(n(100), n(50)); got == nil || *got != 150 {
		t.Errorf("both: %v", got)
	}
}
