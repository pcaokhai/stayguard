package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestMalformedURLHidesPassword_SG003_AC6(t *testing.T) {
	const bad = "postgres://owner:hunter2pw@db:notaport/stayguard" // invalid port: parse fails
	_, err := NewPool(context.Background(), PoolConfig{URL: bad})
	if !errors.Is(err, ErrInvalidDatabaseURL) || strings.Contains(err.Error(), "hunter2pw") {
		t.Fatalf("NewPool error %v", err)
	}
	err = Migrate(context.Background(), bad)
	if !errors.Is(err, ErrInvalidDatabaseURL) || strings.Contains(err.Error(), "hunter2pw") {
		t.Fatalf("Migrate error %v", err)
	}
}
