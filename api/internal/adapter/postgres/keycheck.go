package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrKeyMismatch: the database was written with a different DATA_ENCRYPTION_KEY. The text names no key material.
var ErrKeyMismatch = errors.New("key fingerprint mismatch: this database was written with a different DATA_ENCRYPTION_KEY; " +
	"start with the original key (restore it from its separate backup). The key must never be rotated without a re-encryption job")

// CheckKeyFingerprint stores the fingerprint on first use and afterwards requires it to match. It runs at every start
// of the server and of the CLI commands that write encrypted data.
func CheckKeyFingerprint(ctx context.Context, pool *pgxpool.Pool, fingerprint string) error {
	if _, err := pool.Exec(ctx, `INSERT INTO public.key_fingerprint (fingerprint) VALUES ($1) ON CONFLICT DO NOTHING`, fingerprint); err != nil {
		return fmt.Errorf("store key fingerprint: %w", err)
	}
	var stored string
	if err := pool.QueryRow(ctx, `SELECT fingerprint FROM public.key_fingerprint`).Scan(&stored); err != nil {
		return fmt.Errorf("read key fingerprint: %w", err)
	}
	if stored != fingerprint {
		return ErrKeyMismatch
	}
	return nil
}
