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
	maxProxyHops     = 5
	defaultPort      = 8080
	maxPort          = 65535
	defaultStaticDir = "web/out"
	// Docs/04 line 20: a retried write within a day returns the stored response.
	defaultIdempotencyTTL = 24 * time.Hour
	defaultSepayTolerance = 300 * time.Second
	maxSepayTolerance     = 24 * time.Hour
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
	// SignInPerIP and SignInPerCode are the sign-in attempts allowed per minute per client address and per guesthouse code
	// (SIGNIN_RATE_PER_IP, SIGNIN_RATE_PER_CODE). The defaults are the production limits; only the rehearsal stack raises them.
	SignInPerIP, SignInPerCode int
	// TrustProxy (TRUST_PROXY, default false) reads the client address from X-Forwarded-For; set it only behind the reverse proxy.
	TrustProxy bool
	// SepayTimestampTolerance (SEPAY_TIMESTAMP_TOLERANCE, Go duration, default 300s) is how far X-SePay-Timestamp may be
	// from the server clock. Widen it if SePay deliveries or the server clock drift.
	SepayTimestampTolerance time.Duration
	// TrustedProxyHops (TRUSTED_PROXY_HOPS, default 1) is how many reverse proxies stand in front of the server:
	// the client address is that many entries from the right of X-Forwarded-For. One Caddy means 1.
	TrustedProxyHops int
	SessionTTL       time.Duration // SESSION_TTL_HOURS, default 12
	TrialTTL         time.Duration // TRIAL_TTL_HOURS, default 24
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
	c.SepayTimestampTolerance = defaultSepayTolerance
	if v := getenv("SEPAY_TIMESTAMP_TOLERANCE"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < time.Second || d > maxSepayTolerance {
			return Config{}, fmt.Errorf("SEPAY_TIMESTAMP_TOLERANCE must be a duration between 1s and 24h such as 300s, got %q", v)
		}
		c.SepayTimestampTolerance = d
	}
	c.SignInPerIP, c.SignInPerCode = defaultSignInPerIP, defaultSignInPerCode
	for _, o := range []struct {
		name string
		dst  *int
	}{{"SIGNIN_RATE_PER_IP", &c.SignInPerIP}, {"SIGNIN_RATE_PER_CODE", &c.SignInPerCode}} {
		if v := getenv(o.name); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || n > maxSignInRate {
				return Config{}, fmt.Errorf("%s must be a whole number between 1 and %d, got %q", o.name, maxSignInRate, v)
			}
			*o.dst = n
		}
	}
	if v := getenv("TRUST_PROXY"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("TRUST_PROXY must be a boolean, got %q", v)
		}
		c.TrustProxy = b
	}
	c.TrustedProxyHops = 1
	if v := getenv("TRUSTED_PROXY_HOPS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxProxyHops {
			return Config{}, fmt.Errorf("TRUSTED_PROXY_HOPS must be an integer in 1..%d, got %q", maxProxyHops, v)
		}
		c.TrustedProxyHops = n
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

const (
	defaultSignInPerIP   = 20
	defaultSignInPerCode = 60
	maxSignInRate        = 1_000_000
)

// SignInRateRaised reports whether a sign-in limit is above the production value (the rehearsal override).
func (c Config) SignInRateRaised() bool {
	return c.SignInPerIP > defaultSignInPerIP || c.SignInPerCode > defaultSignInPerCode
}
