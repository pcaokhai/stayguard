// Package clock is the system time source; use cases get it injected so tests can fake time.
package clock

import "time"

// System reads the server clock.
type System struct{}

// Now returns the current instant.
func (System) Now() time.Time { return time.Now() }
