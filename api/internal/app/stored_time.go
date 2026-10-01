package app

import "time"

// storedTime cuts an instant to the precision the database keeps (microseconds), whatever the injected
// clock returns, so what a use case answers equals what a later read returns.
func storedTime(t time.Time) time.Time { return t.UTC().Truncate(time.Microsecond) }
