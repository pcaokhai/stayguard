package ids

import (
	"regexp"
	"testing"
	"time"
)

var format = regexp.MustCompile(`^usr_[0-9A-HJKMNP-TV-Z]{26}$`)

func TestNewFormat_SG102(t *testing.T) {
	g := New(func() time.Time { return time.UnixMilli(1_700_000_000_000) })
	if id := g.New("usr"); !format.MatchString(id) {
		t.Fatalf("id %q does not match prefix_ULID", id)
	}
}

func TestNewUnique_SG102(t *testing.T) {
	g := New(time.Now)
	seen := map[string]bool{}
	for range 1000 {
		id := g.New("usr")
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}

func TestNewTimestampPrefixSortsByTime_SG102(t *testing.T) {
	a := New(func() time.Time { return time.UnixMilli(1_000) }).New("x")
	b := New(func() time.Time { return time.UnixMilli(2_000_000) }).New("x")
	if a >= b {
		t.Fatalf("earlier id %q must sort before later id %q", a, b)
	}
}
