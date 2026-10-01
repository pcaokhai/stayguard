package clock

import (
	"testing"
	"time"
)

func TestSystemNowIsCurrent_SG102(t *testing.T) {
	if d := time.Since(System{}.Now()); d < 0 || d > time.Minute {
		t.Fatalf("System.Now off by %v", d)
	}
}
