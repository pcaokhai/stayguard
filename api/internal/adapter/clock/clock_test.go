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

func TestSystemNowIsMicrosecondUTC_SG205(t *testing.T) {
	for i := 0; i < 100; i++ {
		n := System{}.Now()
		if n.Nanosecond()%1000 != 0 || n.Location() != time.UTC {
			t.Fatalf("Now() = %v: want whole microseconds in UTC (storage precision)", n)
		}
	}
}
