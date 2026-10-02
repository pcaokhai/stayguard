// Package config reads typed, validated settings from the environment at start.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"
)

const (
	defaultPort      = 8080
	maxPort          = 65535
	defaultStaticDir = "web/out"
	// Docs/04 line 20: a retried write within a day returns the stored response.
	defaultIdempotencyTTL = 24 * time.Hour
)

type Config struct {
	Port      int
	StaticDir string
	LogLevel  slog.Level
	// DatabaseURL connects as the application role (NOBYPASSRLS, no DDL): DATABASE_URL, required.
	DatabaseURL string
	// MigrateDatabaseURL connects as the schema owner for `stayguard migrate`: MIGRATE_DATABASE_URL,
	// defaults to DatabaseURL. Only the migrate subcommand uses it.
	MigrateDatabaseURL string
	IdempotencyTTL     time.Duration // IDEMPOTENCY_TTL (Go duration), default 24h
	// AllowPrivilegedDB (ALLOW_PRIVILEGED_DB, default false) lets the server run as a superuser, BYPASSRLS
	// or owner role. LOCAL DEV ONLY: it switches off the second guard (RLS).
	AllowPrivilegedDB bool
	// DemoMode (DEMO_MODE, default false) registers the demo routes; off in production.
	DemoMode bool
	// TrustProxy (TRUST_PROXY, default false) reads the client address from X-Forwarded-For; set it only behind the reverse proxy.
	TrustProxy bool
	SessionTTL time.Duration // SESSION_TTL_HOURS, default 12
	TrialTTL   time.Duration // TRIAL_TTL_HOURS, default 24
	// RoomMapEnabled (FF_S1_ROOM_MAP, default false) turns on the room map operations (slice S1).
	RoomMapEnabled bool
	// DataEncryptionKey (DATA_ENCRYPTION_KEY, standard base64 of 32 bytes) encrypts sensitive fields.
	// Required wherever the server starts; `stayguard migrate` does not need it.
	DataEncryptionKey []byte
	// CheckInEnabled (FF_S2_CHECKIN, default false) turns on createStay and getStay (slice S2).
	CheckInEnabled bool
	// CheckoutEnabled (FF_S3_CHECKOUT, default false) turns on listServices, addStayExtras and checkoutStay (slice S3).
	CheckoutEnabled bool
}

// Load takes the env lookup as a parameter so tests need no process environment.
func Load(getenv func(string) string) (Config, error) {
	return load(getenv, true)
}

// LoadMigrate is Load for the migrate subcommand, which only needs the database URLs.
func LoadMigrate(getenv func(string) string) (Config, error) {
	return load(getenv, false)
}

func load(getenv func(string) string, needKey bool) (Config, error) {
	c := Config{Port: defaultPort, StaticDir: defaultStaticDir, LogLevel: slog.LevelInfo, IdempotencyTTL: defaultIdempotencyTTL}
	if v := getenv("PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil || p < 1 || p > maxPort {
			return Config{}, fmt.Errorf("PORT must be an integer in 1..%d, got %q", maxPort, v)
		}
		c.Port = p
	}
	if v := getenv("STATIC_DIR"); v != "" {
		c.StaticDir = v
	}
	if v := getenv("LOG_LEVEL"); v != "" {
		if err := c.LogLevel.UnmarshalText([]byte(v)); err != nil {
			return Config{}, fmt.Errorf("LOG_LEVEL must be debug, info, warn or error, got %q", v)
		}
	}
	c, err := loadDatabase(c, getenv)
	if err != nil {
		return Config{}, err
	}
	if c, err = loadSessions(c, getenv); err != nil {
		return Config{}, err
	}
	if c, err = loadRoomMap(c, getenv); err != nil {
		return Config{}, err
	}
	if c, err = loadCheckIn(c, getenv); err != nil {
		return Config{}, err
	}
	if c, err = loadCheckout(c, getenv); err != nil {
		return Config{}, err
	}
	if !needKey {
		return c, nil
	}
	return loadEncryption(c, getenv)
}

func loadDatabase(c Config, getenv func(string) string) (Config, error) {
	if c.DatabaseURL = getenv("DATABASE_URL"); c.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if c.MigrateDatabaseURL = getenv("MIGRATE_DATABASE_URL"); c.MigrateDatabaseURL == "" {
		c.MigrateDatabaseURL = c.DatabaseURL
	}
	if v := getenv("IDEMPOTENCY_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return Config{}, fmt.Errorf("IDEMPOTENCY_TTL must be a positive duration such as 24h, got %q", v)
		}
		c.IdempotencyTTL = d
	}
	if v := getenv("TRUST_PROXY"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("TRUST_PROXY must be a boolean, got %q", v)
		}
		c.TrustProxy = b
	}
	if v := getenv("ALLOW_PRIVILEGED_DB"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("ALLOW_PRIVILEGED_DB must be a boolean, got %q", v)
		}
		c.AllowPrivilegedDB = b
	}
	return c, nil
}
