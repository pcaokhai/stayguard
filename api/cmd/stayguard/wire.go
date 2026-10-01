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
	pool         *pgxpool.Pool
	uow          app.UnitOfWork
	idem         app.IdempotencyStore
	audit        app.AuditWriter
	probe        app.ReadinessProbe
	sessions     *app.Sessions
	rooms        *app.Rooms
	stays        *app.Stays
	billing      *app.Billing
	payments     *app.Payments
	housekeeping *app.Housekeeping
	owner        *app.Owner
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
	billing, err := newBilling(cfg, uow, idem, audit, clock.System{})
	if err != nil {
		pool.Close()
		return deps{}, err
	}
	payments, err := newPayments(cfg, uow, idem, audit, clock.System{})
	if err != nil {
		pool.Close()
		return deps{}, err
	}
	rooms := newRooms(uow, clock.System{})
	sessions, err := newSessions(cfg, pool, uow, clock.System{})
	if err != nil {
		pool.Close()
		return deps{}, err
	}
	return deps{
		pool:         pool,
		sessions:     sessions,
		rooms:        newRooms(uow, clock.System{}),
		stays:        stays,
		billing:      billing,
		payments:     payments,
		owner:        app.NewOwner(uow, postgres.OwnerRepo{}, rooms, clock.System{}),
		housekeeping: app.NewHousekeeping(uow, postgres.HousekeepingRepo{}, permissions.RoleBased{}, audit, ids.New(clock.System{}.Now), clock.System{}),
		uow:          uow,
		idem:         idem,
		audit:        audit,
		probe:        postgres.NewReadinessProbe(pool),
	}, nil
}

// newSessions builds the session use cases; the clock is a parameter so tests can move time.
func newSessions(cfg config.Config, pool *pgxpool.Pool, uow app.UnitOfWork, clk app.Clock) (*app.Sessions, error) {
	enc, err := crypto.NewAESGCM(cfg.DataEncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("data encryption key: %w", err)
	}
	return newSessionsWith(cfg, pool, uow, clk, app.NewDemoSeeder(postgres.DemoSeedRepo{}, enc, ids.New(clk.Now))), nil
}

// newSessionsWith lets a test swap the trial seeder (an empty tenant keeps older e2e fixtures simple).
func newSessionsWith(cfg config.Config, pool *pgxpool.Pool, uow app.UnitOfWork, clk app.Clock, seeder app.TrialSeeder) *app.Sessions {
	return app.NewSessions(
		app.SessionsConfig{DemoEnabled: cfg.DemoMode, SessionTTL: cfg.SessionTTL, TrialTTL: cfg.TrialTTL},
		uow, postgres.NewSessionResolver(pool), postgres.NewIdentityRepo(),
		clk, ids.New(clk.Now), crypto.TokenGenerator{}, seeder,
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

// newBilling builds the extras and check-out use cases. It takes the same encryptor as check-in because
// the stay view it returns masks the ID number.
func newBilling(cfg config.Config, uow app.UnitOfWork, idem app.IdempotencyStore, audit app.AuditWriter, clk app.Clock) (*app.Billing, error) {
	enc, err := crypto.NewAESGCM(cfg.DataEncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("data encryption key: %w", err)
	}
	return app.NewBilling(uow, postgres.BillingRepo{}, postgres.ServiceRepo{}, permissions.Derived{}, enc, idem, audit,
		ids.New(clk.Now), clk), nil
}

// newPayments builds the payment use cases and the settlement handler. The encryptor opens the tenant's bank account.
func newPayments(cfg config.Config, uow app.UnitOfWork, idem app.IdempotencyStore, audit app.AuditWriter, clk app.Clock) (*app.Payments, error) {
	enc, err := crypto.NewAESGCM(cfg.DataEncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("data encryption key: %w", err)
	}
	return app.NewPayments(uow, postgres.PaymentRepo{}, permissions.Derived{}, enc, idem, audit, ids.New(clk.Now), clk), nil
}
