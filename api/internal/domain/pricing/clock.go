package pricing

import (
	"fmt"
	"regexp"
	"strconv"
)

// clockPattern is the schema pattern verbatim; the schema agreement test keeps them in step.
var clockPattern = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// Clock is a wall-clock time of day in the tenant zone.
type Clock struct {
	Hour, Minute int
}

func parseClock(s string) (Clock, bool) {
	if !clockPattern.MatchString(s) {
		return Clock{}, false
	}
	// The pattern guarantees two digits on each side, so Atoi cannot fail.
	h, _ := strconv.Atoi(s[:2])
	m, _ := strconv.Atoi(s[3:])
	return Clock{Hour: h, Minute: m}, true
}

func (c Clock) String() string { return fmt.Sprintf("%02d:%02d", c.Hour, c.Minute) }

func (c Clock) MarshalJSON() ([]byte, error) { return []byte(`"` + c.String() + `"`), nil }
