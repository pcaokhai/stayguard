package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/platform/config"
)

// deps are the constructed adapters. The unit of work, idempotency store and audit writer have no
// consumer until SG-102+; they are built here so the wiring has one home.
type deps struct {
	pool  *pgxpool.Pool
	uow   app.UnitOfWork
	idem  app.IdempotencyStore
	audit app.AuditWriter
	probe app.ReadinessProbe
}

func newDeps(ctx context.Context, cfg config.Config) (deps, error) {
	pool, err := postgres.NewPool(ctx, postgres.PoolConfig{URL: cfg.DatabaseURL})
	if err != nil {
		return deps{}, fmt.Errorf("database: %w", err)
	}
	return deps{
		pool:  pool,
		uow:   postgres.NewUnitOfWork(pool),
		idem:  postgres.NewIdempotencyStore(cfg.IdempotencyTTL),
		audit: postgres.NewAuditWriter(),
		probe: postgres.NewReadinessProbe(pool),
	}, nil
}
