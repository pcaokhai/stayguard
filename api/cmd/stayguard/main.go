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

	httpadapter "stayguard/api/internal/adapter/http"
	"stayguard/api/internal/platform/config"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownTimeout   = 10 * time.Second // Cloud Run gives 10 s after SIGTERM
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "stayguard:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))

	srv := &http.Server{
		Addr:              net.JoinHostPort("", strconv.Itoa(cfg.Port)),
		Handler:           httpadapter.NewRouter(log, cfg.StaticDir),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
