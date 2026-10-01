package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pcaokhai/stayguard/api/internal/adapter/clock"
	"github.com/pcaokhai/stayguard/api/internal/adapter/crypto"
	"github.com/pcaokhai/stayguard/api/internal/adapter/ids"
	"github.com/pcaokhai/stayguard/api/internal/adapter/permissions"
	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres"
	"github.com/pcaokhai/stayguard/api/internal/adapter/pricing"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/platform/config"
)

// deps are the constructed adapters; the wiring has one home here.
type deps struct {
	pool     *pgxpool.Pool
	uow      app.UnitOfWork
	idem     app.IdempotencyStore
	audit    app.AuditWriter
	probe    app.ReadinessProbe
	sessions *app.Sessions
	rooms    *app.Rooms
	stays    *app.Stays
}

func newDeps(ctx context.Context, cfg config.Config) (deps, error) {
	pool, err := postgres.NewPool(ctx, postgres.PoolConfig{URL: cfg.DatabaseURL, AllowPrivileged: cfg.AllowPrivilegedDB})
	if err != nil {
		return deps{}, fmt.Errorf("database: %w", err)
	}
	uow := postgres.NewUnitOfWork(pool)
	idem, audit := postgres.NewIdempotencyStore(cfg.IdempotencyTTL), postgres.NewAuditWriter()
	stays, err := newStays(cfg, uow, idem, audit, clock.System{})
	if err != nil {
		pool.Close()
		return deps{}, err
	}
	return deps{
		pool:     pool,
		sessions: newSessions(cfg, pool, uow, clock.System{}),
		rooms:    newRooms(uow, clock.System{}),
		stays:    stays,
		uow:      uow,
		idem:     idem,
		audit:    audit,
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

// newRooms builds the room map read use cases. Derived permissions (SG-102 rule) are temporary until
// SG-501 stores levels; the quoter prices from each stay's own rate plan snapshot.
func newRooms(uow app.UnitOfWork, clk app.Clock) *app.Rooms {
	return app.NewRooms(uow, postgres.RoomRepo{}, permissions.Derived{}, pricing.Quoter{}, clk)
}

// newStays builds the check-in use cases. The encryptor takes its key from the validated config;
// permissions are derived until SG-501, like the room map.
func newStays(cfg config.Config, uow app.UnitOfWork, idem app.IdempotencyStore, audit app.AuditWriter, clk app.Clock) (*app.Stays, error) {
	enc, err := crypto.NewAESGCM(cfg.DataEncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("data encryption key: %w", err)
	}
	return app.NewStays(uow, postgres.StayRepo{}, permissions.Derived{}, enc, idem, audit, ids.New(clk.Now), clk), nil
}
