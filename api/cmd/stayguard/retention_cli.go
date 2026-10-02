package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/adapter/clock"
	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/platform/config"
)

const retentionUsage = "usage: stayguard guest-id retention   (daily; needs DATABASE_URL)"

// runRetentionCLI is the daily guest ID retention job (docs/15 rule 25), run by cron or a systemd timer on the server.
// One database function lists the tenants with data past retention; the deletion and its audit entry go
// through the application role, tenant by tenant, under RLS. It prints counts, never data.
func runRetentionCLI(ctx context.Context, args []string) error {
	if len(args) != 2 || args[0] != "guest-id" || args[1] != "retention" {
		return errors.New(retentionUsage)
	}
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	pool, err := postgres.NewPool(ctx, postgres.PoolConfig{URL: cfg.DatabaseURL, AllowPrivileged: cfg.AllowPrivilegedDB})
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer pool.Close()
	g, err := newGuestIDs(cfg, postgres.NewUnitOfWork(pool), postgres.NewAuditWriter(), clock.System{})
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	due, err := postgres.TenantsDue(ctx, pool, now)
	if err != nil {
		return err
	}
	return purgeTenants(ctx, g, due, now, os.Stdout)
}

// purgeTenants runs the per-tenant purge and reports counts. One tenant failing does not stop the others.
func purgeTenants(ctx context.Context, g *app.GuestIDs, tenants []string, now time.Time, out io.Writer) error {
	var failed []error
	total := 0
	for _, t := range tenants {
		n, err := g.Purge(ctx, t, now)
		if err != nil {
			failed = append(failed, fmt.Errorf("tenant %s: %w", t, err))
			continue
		}
		total += n
		say(out, "tenant %s: %d stays purged\n", t, n)
	}
	say(out, "guest id retention: %d stays purged in %d tenants\n", total, len(tenants))
	return errors.Join(failed...)
}
