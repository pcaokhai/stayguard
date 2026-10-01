// Package clock is the system time source; use cases get it injected so tests can fake time.
package clock

import "time"

// System reads the server clock.
type System struct{}

// Now returns the current instant in UTC, cut to microseconds: PostgreSQL stores microseconds, so a time
// kept at nanoseconds would read back different from the value the use case answered with.
func (System) Now() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }
