// Package app holds use cases and the small ports they consume.
package app

import (
	"context"
	"errors"
)

// ErrTenantRequired is returned before any query when a unit of work has no tenant.
var ErrTenantRequired = errors.New("tenant is required")

// Tx is the tenant-scoped handle a unit of work gives to repositories. It is opaque to use cases:
// only the adapter that created it can read or write through it.
type Tx interface {
	TenantID() string
}

// UnitOfWork runs fn in one transaction scoped to tenantID: commit on nil, rollback on error or panic.
type UnitOfWork interface {
	Do(ctx context.Context, tenantID string, fn func(ctx context.Context, tx Tx) error) error
}
