// Command stayguard wires config, logger, router and server; it holds no business logic.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	httpadapter "github.com/pcaokhai/stayguard/api/internal/adapter/http"
	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres"
	"github.com/pcaokhai/stayguard/api/internal/platform/config"
)

const (
	migrateCommand    = "migrate"
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
	// Cloud Run sends SIGKILL 10 s after SIGTERM; stop waiting at 8 s so we exit cleanly first.
	shutdownTimeout = 8 * time.Second
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "stayguard:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) > 1 && (os.Args[1] == "tenant" || os.Args[1] == "sepay") {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return runInstallerCLI(ctx, os.Args[1:])
	}
	load := config.Load
	isMigrate := len(os.Args) > 1 && os.Args[1] == migrateCommand
	if isMigrate {
		load = config.LoadMigrate
	}
	cfg, err := load(os.Getenv)
	if err != nil {
		return err
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Schema changes run only in this subcommand, as the owner role, never at request time.
	if isMigrate {
		if err := postgres.Migrate(ctx, cfg.MigrateDatabaseURL, cfg.AllowPrivilegedDB); err != nil {
			return err
		}
		log.Info("migrations applied")
		return nil
	}
	return serve(ctx, cfg, log)
}

func serve(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	d, err := newDeps(ctx, cfg)
	if err != nil {
		return err
	}
	defer d.pool.Close()

	srv := &http.Server{
		Addr: net.JoinHostPort("", strconv.Itoa(cfg.Port)),
		Handler: httpadapter.NewRouter(log, httpadapter.Options{
			StaticDir: cfg.StaticDir, Probe: d.probe, Sessions: d.sessions, DemoEnabled: cfg.DemoMode,
			Rooms: d.rooms, RoomMapEnabled: cfg.RoomMapEnabled, Stays: d.stays, CheckInEnabled: cfg.CheckInEnabled,
			Billing: d.billing, CheckoutEnabled: cfg.CheckoutEnabled, Payments: d.payments, Housekeeping: d.housekeeping, Owner: d.owner, StayOps: d.stayOps, Shifts: d.shifts, Monitor: d.monitor,
			Auth: d.auth, Staff: d.staff, Bank: d.bank, TrustProxy: cfg.TrustProxy,
		}),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
	serveErr := make(chan error, 1) // buffered: the goroutine exits even if nobody reads
	go func() { serveErr <- srv.ListenAndServe() }()
	log.Info("server started", "port", cfg.Port)

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	return nil
}
