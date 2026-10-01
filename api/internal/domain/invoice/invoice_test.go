package invoice

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestBillCode_SG205_AC3(t *testing.T) {
	day := time.Date(2026, 9, 30, 23, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		room    string
		attempt int
		want    string
	}{
		{"contract example", "A101", 1, "PH0930A101"},
		{"dash dropped and lowercased", "a-101", 1, "PH0930A101"},
		{"space dropped", "A 101", 1, "PH0930A101"},
		{"unicode letters dropped", "Phòng Đ1", 1, "PH0930PHNG1"},
		{"cut to 8", "ABCDEFGHIJKL", 1, "PH0930ABCDEFGH"},
		{"second attempt", "A101", 2, "PH0930A1012"},
		{"tenth attempt", "A101", 10, "PH0930A10110"},
		{"longest", "ABCDEFGH", 10, "PH0930ABCDEFGH10"},
	}
	for _, c := range cases {
		got, err := BillCode(day, c.room, c.attempt)
		if err != nil || got != c.want {
			t.Fatalf("%s: got %q %v, want %q", c.name, got, err, c.want)
		}
		if len(got) > MaxBillCodeLen {
			t.Fatalf("%s: too long %q", c.name, got)
		}
	}
	for _, a := range []int{0, -1} {
		if _, err := BillCode(day, "A101", a); err == nil {
			t.Fatalf("attempt %d must fail", a)
		}
	}
	for _, r := range []string{"", "---", "Đ", "  "} {
		if _, err := BillCode(day, r, 1); !errors.Is(err, ErrBillCode) {
			t.Fatalf("room %q: %v", r, err)
		}
	}
	if d := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC); func() string { s, _ := BillCode(d, "B2", 1); return s }() != "PH0105B2" {
		t.Fatal("month and day must be zero padded")
	}
}

func TestParseStatus_SG205_AC3(t *testing.T) {
	for _, s := range []Status{StatusOpen, StatusPaid} {
		if got, err := ParseStatus(string(s)); err != nil || got != s {
			t.Fatalf("%s: %v", s, err)
		}
	}
	_, err := ParseStatus("SECRET_VALUE")
	if err == nil || strings.Contains(err.Error(), "SECRET_VALUE") {
		t.Fatalf("must fail closed without the input: %v", err)
	}
	if _, err := ParseStatus("open"); err == nil {
		t.Fatal("case sensitive")
	}
}
