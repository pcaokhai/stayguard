package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Timeouts bound every I/O (docs/10): dial, each statement, a transaction left idle, and a whole unit of work.
const (
	connectTimeout       = 5 * time.Second
	statementTimeout     = 10 * time.Second
	idleInTxTimeout      = 15 * time.Second
	unitOfWorkTimeout    = 30 * time.Second
	rollbackTimeout      = 5 * time.Second
	maxConnLifetime      = 30 * time.Minute
	defaultMaxConns      = 10
	statementTimeoutName = "statement_timeout"
	idleInTxTimeoutName  = "idle_in_transaction_session_timeout"
)

// ErrInvalidDatabaseURL replaces any URL parse error: those can echo the connection string.
var ErrInvalidDatabaseURL = errors.New("invalid database URL")

// PoolConfig is the pool input. URL must connect as the application role (NOBYPASSRLS, no DDL).
type PoolConfig struct {
	URL      string
	MaxConns int32 // 0 means defaultMaxConns
}

// NewPool opens a pgx pool with timeouts set on the dial and on every session.
func NewPool(ctx context.Context, cfg PoolConfig) (*pgxpool.Pool, error) {
	pc, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, ErrInvalidDatabaseURL // the parse error can quote the URL, password included
	}
	pc.ConnConfig.ConnectTimeout = connectTimeout
	pc.ConnConfig.RuntimeParams[statementTimeoutName] = fmt.Sprint(statementTimeout.Milliseconds())
	pc.ConnConfig.RuntimeParams[idleInTxTimeoutName] = fmt.Sprint(idleInTxTimeout.Milliseconds())
	pc.MaxConnLifetime = maxConnLifetime
	pc.MaxConns = defaultMaxConns
	if cfg.MaxConns > 0 {
		pc.MaxConns = cfg.MaxConns
	}
	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	return pool, nil
}
