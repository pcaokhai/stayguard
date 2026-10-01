package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver for goose
	"github.com/pressly/goose/v3"

	"github.com/pcaokhai/stayguard/api/migrations"
)

// migrateTimeout bounds a whole `stayguard migrate` run (every migration is one transaction).
const migrateTimeout = 5 * time.Minute

// newProvider builds a forward-only goose provider over the embedded migrations.
func newProvider(db *sql.DB) (*goose.Provider, error) {
	p, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return nil, fmt.Errorf("goose provider: %w", err)
	}
	return p, nil
}

// Migrate applies every pending embedded migration. url must connect as the schema owner; the
// application role has no DDL rights. It is called only by the `migrate` subcommand, never at request time.
func Migrate(ctx context.Context, url string) error {
	ctx, cancel := context.WithTimeout(ctx, migrateTimeout)
	defer cancel()
	if _, err := pgx.ParseConfig(url); err != nil {
		return ErrInvalidDatabaseURL
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		return fmt.Errorf("open migration database: %w", err)
	}
	defer func() { _ = db.Close() }()
	p, err := newProvider(db)
	if err != nil {
		return err
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
