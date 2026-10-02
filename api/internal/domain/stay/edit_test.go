package stay

import (
	"errors"
	"testing"
	"time"
)

func TestValidateCheckInEdit_SG801_AC2(t *testing.T) {
	original := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	now := original.Add(3 * time.Hour)
	cases := []struct {
		name    string
		newAt   time.Time
		reason  string
		note    string
		wantErr string // "<path> <code>" or ""
	}{
		{"earlier is fine", original.Add(-2 * time.Hour), "WRONG_TIME", "typed wrong", ""},
		{"60 minutes later is the limit", original.Add(60 * time.Minute), "LATE_ARRIVAL", "guest came late", ""},
		{"61 minutes later", original.Add(61 * time.Minute), "WRONG_TIME", "typed wrong", "newCheckInAt MAX"},
		{"in the future", now.Add(time.Second), "WRONG_TIME", "typed wrong", "newCheckInAt MAX"},
		{"unknown reason", original, "BORED", "typed wrong", "reasonCode PATTERN"},
		{"note too short", original, "OTHER", " a ", "note TOO_SHORT"},
	}
	for _, c := range cases {
		err := ValidateCheckInEdit(original, c.newAt, now, c.reason, c.note)
		var ve *ValidationError
		if c.wantErr == "" {
			if err != nil {
				t.Errorf("%s: %v", c.name, err)
			}
			continue
		}
		if !errors.As(err, &ve) || len(ve.Errors) != 1 || ve.Errors[0].Path+" "+ve.Errors[0].Code != c.wantErr {
			t.Errorf("%s: got %v want %s", c.name, err, c.wantErr)
		}
	}
}
