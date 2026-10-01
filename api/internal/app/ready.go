package app

import "context"

// ReadinessProbe reports whether the service can take traffic: nil when the database answers and
// every migration is applied. The error is for logs only and must never reach a client.
type ReadinessProbe interface {
	Check(ctx context.Context) error
}
