package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pcaokhai/stayguard/api/internal/adapter/clock"
	"github.com/pcaokhai/stayguard/api/internal/adapter/crypto"
	"github.com/pcaokhai/stayguard/api/internal/adapter/ids"
	"github.com/pcaokhai/stayguard/api/internal/adapter/permissions"
	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres"
	"github.com/pcaokhai/stayguard/api/internal/adapter/pricing"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/platform/config"
	"github.com/pcaokhai/stayguard/api/internal/platform/ratelimit"
)

// deps are the constructed adapters; the wiring has one home here.
type deps struct {
	pool         *pgxpool.Pool
	uow          app.UnitOfWork
	idem         app.IdempotencyStore
	audit        app.AuditWriter
	probe        app.ReadinessProbe
	sessions     *app.Sessions
	auth         *app.Auth
	staff        *app.Staff
	bank         *app.Bank
	setup        *app.Setup
	rooms        *app.Rooms
	stays        *app.Stays
	billing      *app.Billing
	payments     *app.Payments
	housekeeping *app.Housekeeping
	owner        *app.Owner
	stayOps      stayOps
	shifts       *app.Shifts
	monitor      monitorOps
	maintenance  *app.Maintenance
	roster       *app.Rosters
	finance      financeOps
}

// financeOps is payroll, expenses and the report behind one handler dependency.
type financeOps struct {
	*app.Payroll
	*app.Expenses
	*app.Reports
}

// monitorOps is the owner monitoring reads and the link of an unmatched transfer behind one handler dependency.
type monitorOps struct {
	*app.Monitor
	*app.Payments
}

// stayOps is the stay corrections and history behind one handler dependency.
type stayOps struct {
	*app.StayEdits
	*app.StayHistory
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
	shifts := newShifts(uow, idem, audit, clock.System{})
	stays.WithCash(shifts)
	payments.WithCash(shifts)
	stayOps, err := newStayOps(cfg, uow, idem, audit, clock.System{})
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
	auth, err := newAuth(cfg, sessions, pool, audit, clock.System{})
	if err != nil {
		pool.Close()
		return deps{}, err
	}
	bank, err := newBank(cfg, uow, auth, idem, audit, clock.System{})
	if err != nil {
		pool.Close()
		return deps{}, err
	}
	return deps{
		pool:         pool,
		sessions:     sessions,
		auth:         auth,
		staff:        newStaff(uow, auth, idem, audit, clock.System{}),
		bank:         bank,
		setup:        app.NewSetup(uow, postgres.SetupRepo{}, idem, audit, ids.New(clock.System{}.Now), clock.System{}).WithAlerts(postgres.AlertWriter{}).WithExpenses(postgres.FinanceRepo{}),
		rooms:        newRooms(uow, clock.System{}),
		stays:        stays,
		billing:      billing,
		payments:     payments,
		stayOps:      stayOps,
		shifts:       shifts,
		monitor:      monitorOps{newMonitor(uow, clock.System{}), payments},
		maintenance:  newMaintenance(uow, idem, audit, clock.System{}),
		roster:       newRosters(uow, idem, audit, clock.System{}),
		finance:      newFinance(uow, idem, audit, clock.System{}),
		owner:        app.NewOwner(uow, postgres.OwnerRepo{}, rooms, clock.System{}).WithMonitor(postgres.MonitorRepo{}),
		housekeeping: app.NewHousekeeping(uow, postgres.HousekeepingRepo{}, permissions.Stored{}, audit, ids.New(clock.System{}.Now), clock.System{}),
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
	return app.NewRooms(uow, postgres.RoomRepo{}, permissions.Stored{}, pricing.Quoter{}, clk)
}

// newStays builds the check-in use cases. The encryptor takes its key from the validated config;
// permissions are derived until SG-501, like the room map.
func newStays(cfg config.Config, uow app.UnitOfWork, idem app.IdempotencyStore, audit app.AuditWriter, clk app.Clock) (*app.Stays, error) {
	enc, err := crypto.NewAESGCM(cfg.DataEncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("data encryption key: %w", err)
	}
	return app.NewStays(uow, postgres.StayRepo{}, permissions.Stored{}, enc, idem, audit, ids.New(clk.Now), clk), nil
}

// newBilling builds the extras and check-out use cases. It takes the same encryptor as check-in because
// the stay view it returns masks the ID number.
func newBilling(cfg config.Config, uow app.UnitOfWork, idem app.IdempotencyStore, audit app.AuditWriter, clk app.Clock) (*app.Billing, error) {
	enc, err := crypto.NewAESGCM(cfg.DataEncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("data encryption key: %w", err)
	}
	return app.NewBilling(uow, postgres.BillingRepo{}, postgres.ServiceRepo{}, permissions.Stored{}, enc, idem, audit,
		ids.New(clk.Now), clk), nil
}

// newPayments builds the payment use cases and the settlement handler. The encryptor opens the tenant's bank account.
func newPayments(cfg config.Config, uow app.UnitOfWork, idem app.IdempotencyStore, audit app.AuditWriter, clk app.Clock) (*app.Payments, error) {
	enc, err := crypto.NewAESGCM(cfg.DataEncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("data encryption key: %w", err)
	}
	p := app.NewPayments(uow, postgres.PaymentRepo{}, permissions.Stored{}, enc, idem, audit, ids.New(clk.Now), clk)
	return p.WithAlerts(postgres.AlertWriter{}), nil
}

// newStayOps builds the check-in time and move corrections and the stay history reads.
func newStayOps(cfg config.Config, uow app.UnitOfWork, idem app.IdempotencyStore, audit app.AuditWriter, clk app.Clock) (stayOps, error) {
	enc, err := crypto.NewAESGCM(cfg.DataEncryptionKey)
	if err != nil {
		return stayOps{}, fmt.Errorf("data encryption key: %w", err)
	}
	levels := permissions.Stored{}
	return stayOps{
		StayEdits:   app.NewStayEdits(uow, postgres.StayEditRepo{}, levels, enc, idem, audit, postgres.AlertWriter{}, ids.New(clk.Now), clk),
		StayHistory: app.NewStayHistory(uow, postgres.StayHistoryRepo{}, levels, clk),
	}, nil
}

// Sign-in rate limits: a legitimate front desk signs in a few times a day, so these only stop guessing.
const (
	signInPerIP     = 20
	signInPerCode   = 60
	signInRateEvery = time.Minute
)

// newAuth builds PIN sign-in with per-IP and per-guesthouse-code rate limits.
func newAuth(cfg config.Config, sessions *app.Sessions, pool *pgxpool.Pool, audit app.AuditWriter, clk app.Clock) (*app.Auth, error) {
	return app.NewAuth(sessions, postgres.NewTenantResolver(pool), postgres.NewAuthRepo(), crypto.NewPinHasher(cfg.DataEncryptionKey), audit, postgres.AlertWriter{},
		ratelimit.New(signInPerIP, signInRateEvery, clk.Now), ratelimit.New(signInPerCode, signInRateEvery, clk.Now))
}

// newStaff builds the staff use cases; the shift repository tells removeStaff whether the person still has a shift open.
func newStaff(uow app.UnitOfWork, auth *app.Auth, idem app.IdempotencyStore, audit app.AuditWriter, clk app.Clock) *app.Staff {
	return app.NewStaff(uow, postgres.StaffRepo{}, auth, postgres.NewAuthRepo(), crypto.PinGenerator{}, idem, audit, postgres.ShiftRepo{}, ids.New(clk.Now), clk)
}

// newShifts builds the shift use cases; they are also the cash ledger that check-in deposits and cash payments write to.
func newShifts(uow app.UnitOfWork, idem app.IdempotencyStore, audit app.AuditWriter, clk app.Clock) *app.Shifts {
	return app.NewShifts(uow, postgres.ShiftRepo{}, permissions.Stored{}, idem, audit, postgres.AlertWriter{}, ids.New(clk.Now), clk)
}

func newMonitor(uow app.UnitOfWork, clk app.Clock) *app.Monitor {
	return app.NewMonitor(uow, postgres.MonitorRepo{}, permissions.Stored{}, clk)
}

// newBank builds the property, receiving account and SePay status use cases.
func newBank(cfg config.Config, uow app.UnitOfWork, auth *app.Auth, idem app.IdempotencyStore, audit app.AuditWriter, clk app.Clock) (*app.Bank, error) {
	enc, err := crypto.NewAESGCM(cfg.DataEncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("data encryption key: %w", err)
	}
	return app.NewBank(uow, postgres.BankRepo{}, auth, enc, idem, audit, ids.New(clk.Now), clk), nil
}

// newInstaller builds what the installer CLI runs: import, webhook address, SePay secret and status.
func newInstaller(cfg config.Config, pool *pgxpool.Pool, clk app.Clock) (*app.Installer, error) {
	enc, err := crypto.NewAESGCM(cfg.DataEncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("data encryption key: %w", err)
	}
	return app.NewInstaller(postgres.NewUnitOfWork(pool), postgres.NewTenantResolver(pool), postgres.TenantSetupRepo{}, postgres.DemoSeedRepo{},
		postgres.BankRepo{}, postgres.StaffRepo{}, postgres.NewAuthRepo(), enc, crypto.NewPinHasher(cfg.DataEncryptionKey), crypto.PinGenerator{}, crypto.TokenGenerator{},
		postgres.NewAuditWriter(), postgres.AlertWriter{}, ids.New(clk.Now), clk), nil
}

func newMaintenance(uow app.UnitOfWork, idem app.IdempotencyStore, audit app.AuditWriter, clk app.Clock) *app.Maintenance {
	return app.NewMaintenance(uow, postgres.TicketRepo{}, permissions.Stored{}, idem, audit, postgres.AlertWriter{}, ids.New(clk.Now), clk).
		WithExpenses(postgres.FinanceRepo{})
}

func newRosters(uow app.UnitOfWork, idem app.IdempotencyStore, audit app.AuditWriter, clk app.Clock) *app.Rosters {
	return app.NewRosters(uow, postgres.RosterRepo{}, permissions.Stored{}, idem, audit, postgres.AlertWriter{}, ids.New(clk.Now), clk)
}

// newFinance builds payroll, expenses and the report. The same repository is the ledger that other use cases post
// automatic expense lines to.
func newFinance(uow app.UnitOfWork, idem app.IdempotencyStore, audit app.AuditWriter, clk app.Clock) financeOps {
	repo, levels, id := postgres.FinanceRepo{}, permissions.Stored{}, ids.New(clk.Now)
	return financeOps{
		Payroll:  app.NewPayroll(uow, repo, postgres.RosterRepo{}, repo, levels, idem, audit, id, clk),
		Expenses: app.NewExpenses(uow, repo, levels, idem, audit, id, clk),
		Reports:  app.NewReports(uow, repo, repo, levels, clk),
	}
}
