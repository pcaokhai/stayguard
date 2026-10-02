package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/adapter/clock"
	"github.com/pcaokhai/stayguard/api/internal/adapter/crypto"
	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/platform/config"
)

const (
	jobsUsage = "usage: stayguard jobs run [--every 5m]   (needs DATABASE_URL and DATA_ENCRYPTION_KEY)"
	// minJobsEvery keeps a typo from hammering the database.
	minJobsEvery = 10 * time.Second
)

// parseJobsArgs reads `jobs run` with an optional `--every <duration>`; zero means run once.
func parseJobsArgs(args []string) (time.Duration, error) {
	if len(args) < 2 || args[0] != "jobs" || args[1] != "run" {
		return 0, errors.New(jobsUsage)
	}
	switch len(args) {
	case 2:
		return 0, nil
	case 4:
		if args[2] == "--every" {
			d, err := time.ParseDuration(args[3])
			if err == nil && d >= minJobsEvery {
				return d, nil
			}
		}
	}
	return 0, errors.New(jobsUsage)
}

// repeatJobs runs one round now and then every interval until ctx ends. A failed round is reported and the next one still runs.
func repeatJobs(ctx context.Context, every time.Duration, round func(context.Context) error, out io.Writer) error {
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		if err := round(ctx); err != nil && ctx.Err() == nil {
			say(out, "jobs round failed: %v\n", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
	}
}

// tenantJob is one daily task. It runs once per tenant, inside that tenant's own scope, and returns how many things it
// changed. Leave-to-TAKEN needs no job: TAKEN is derived when leave is read.
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
	e := newFinance(uowDeps.uow, postgres.NewIdempotencyStore(0), postgres.NewAuditWriter(), clock.System{}).Expenses
	pay, err := newPayments(cfg, uowDeps.uow, postgres.NewIdempotencyStore(0), postgres.NewAuditWriter(), clock.System{})
	if err != nil {
		return nil, err
	}
	return []tenantJob{
		{name: "partial-transfer-alerts", run: pay.RaisePartialAlerts},
		{name: "guest-id-retention", run: g.Purge},
		{name: "recurring-expenses", run: e.CopyRecurring},
	}, nil
}

// runJobsCLI is `stayguard jobs run [--every 5m]`: every job for every tenant, each in a normal tenant-scoped transaction. One
// tenant or job failing does not stop the others; the exit status says something failed. It prints counts only.
func runJobsCLI(ctx context.Context, args []string) error {
	every, err := parseJobsArgs(args)
	if err != nil {
		return err
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
	if err = postgres.CheckKeyFingerprint(ctx, pool, crypto.KeyFingerprint(cfg.DataEncryptionKey)); err != nil {
		return err
	}
	jobs, err := jobRegistry(cfg, jobDeps{uow: postgres.NewUnitOfWork(pool)})
	if err != nil {
		return err
	}
	round := func(ctx context.Context) error {
		tenants, err := postgres.TenantIDs(ctx, pool)
		if err != nil {
			return err
		}
		return runJobs(ctx, jobs, tenants, time.Now().UTC(), os.Stdout)
	}
	if every == 0 {
		return round(ctx)
	}
	return repeatJobs(ctx, every, round, os.Stdout)
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
