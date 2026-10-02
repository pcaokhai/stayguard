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

const jobsUsage = "usage: stayguard jobs run   (daily, from cron; needs DATABASE_URL and DATA_ENCRYPTION_KEY)"

// tenantJob is one daily task. It runs once per tenant, inside that tenant's own scope, and returns how many things it
// changed. Recurring expenses and leave-to-TAKEN join this registry later.
type tenantJob struct {
	name string
	run  func(ctx context.Context, tenantID string, now time.Time) (int, error)
}

// jobDeps are what the jobs share.
type jobDeps struct{ uow app.UnitOfWork }

// jobRegistry builds the jobs from the shared wiring.
func jobRegistry(cfg config.Config, uowDeps jobDeps) ([]tenantJob, error) {
	g, err := newGuestIDs(cfg, uowDeps.uow, postgres.NewAuditWriter(), clock.System{})
	if err != nil {
		return nil, err
	}
	return []tenantJob{
		{name: "guest-id-retention", run: g.Purge},
	}, nil
}

// runJobsCLI is `stayguard jobs run`: every job for every tenant, each in a normal tenant-scoped transaction. One
// tenant or job failing does not stop the others; the exit status says something failed. It prints counts only.
func runJobsCLI(ctx context.Context, args []string) error {
	if len(args) != 2 || args[0] != "jobs" || args[1] != "run" {
		return errors.New(jobsUsage)
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
	jobs, err := jobRegistry(cfg, jobDeps{uow: postgres.NewUnitOfWork(pool)})
	if err != nil {
		return err
	}
	tenants, err := postgres.TenantIDs(ctx, pool)
	if err != nil {
		return err
	}
	return runJobs(ctx, jobs, tenants, time.Now().UTC(), os.Stdout)
}

func runJobs(ctx context.Context, jobs []tenantJob, tenants []string, now time.Time, out io.Writer) error {
	var failed []error
	totals := map[string]int{}
	for _, t := range tenants {
		for _, j := range jobs {
			n, err := j.run(ctx, t, now)
			if err != nil {
				failed = append(failed, fmt.Errorf("%s for tenant %s: %w", j.name, t, err))
				continue
			}
			totals[j.name] += n
		}
	}
	for _, j := range jobs {
		say(out, "%s: %d changed in %d tenants\n", j.name, totals[j.name], len(tenants))
	}
	return errors.Join(failed...)
}
