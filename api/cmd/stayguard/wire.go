package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pcaokhai/stayguard/api/internal/adapter/clock"
	"github.com/pcaokhai/stayguard/api/internal/adapter/crypto"
	"github.com/pcaokhai/stayguard/api/internal/adapter/ids"
	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/platform/config"
)

// deps are the constructed adapters. The idempotency store and audit writer have no consumer yet;
// they are built here so the wiring has one home.
type deps struct {
	pool     *pgxpool.Pool
	uow      app.UnitOfWork
	idem     app.IdempotencyStore
	audit    app.AuditWriter
	probe    app.ReadinessProbe
	sessions *app.Sessions
}

func newDeps(ctx context.Context, cfg config.Config) (deps, error) {
	pool, err := postgres.NewPool(ctx, postgres.PoolConfig{URL: cfg.DatabaseURL, AllowPrivileged: cfg.AllowPrivilegedDB})
	if err != nil {
		return deps{}, fmt.Errorf("database: %w", err)
	}
	uow := postgres.NewUnitOfWork(pool)
	return deps{
		pool:     pool,
		sessions: newSessions(cfg, pool, uow, clock.System{}),
		uow:      uow,
		idem:     postgres.NewIdempotencyStore(cfg.IdempotencyTTL),
		audit:    postgres.NewAuditWriter(),
		probe:    postgres.NewReadinessProbe(pool),
	}, nil
}

// newSessions builds the session use cases; the clock is a parameter so tests can move time.
func newSessions(cfg config.Config, pool *pgxpool.Pool, uow app.UnitOfWork, clk app.Clock) *app.Sessions {
	return app.NewSessions(
		app.SessionsConfig{DemoEnabled: cfg.DemoMode, SessionTTL: cfg.SessionTTL, TrialTTL: cfg.TrialTTL},
		uow, postgres.NewSessionResolver(pool), postgres.NewIdentityRepo(),
		clk, ids.New(clk.Now), crypto.TokenGenerator{},
	)
}
