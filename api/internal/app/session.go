package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

const (
	trialTenantName = "Demo guesthouse"
	tenantPrefix    = "tn"
	userPrefix      = "us"
)

var locales = map[string]bool{"vi": true, "en": true}

type SessionsConfig struct {
	DemoEnabled          bool
	SessionTTL, TrialTTL time.Duration
}

// Sessions creates and authenticates role-picker sessions (ADR-010, ADR-015).
type Sessions struct {
	cfg      SessionsConfig
	uow      UnitOfWork
	resolver SessionResolver
	repo     IdentityRepo
	clock    Clock
	ids      IDGenerator
	tokens   TokenGenerator
	seeder   TrialSeeder
}

// TrialSeeder fills a newly created trial tenant (rooms, prices, services, sample stays).
type TrialSeeder interface {
	Seed(ctx context.Context, tx Tx, now time.Time) error
}

func NewSessions(cfg SessionsConfig, uow UnitOfWork, resolver SessionResolver, repo IdentityRepo,
	clock Clock, ids IDGenerator, tokens TokenGenerator, seeder TrialSeeder) *Sessions {
	return &Sessions{cfg, uow, resolver, repo, clock, ids, tokens, seeder}
}

// DemoSession carries the raw token, which exists only in this value.
type DemoSession struct {
	Token     string
	ExpiresAt time.Time
	TenantID  string
	User      User
}

const redacted = "[redacted]"

// String, GoString and LogValue keep the raw token out of %v, %+v, %#v and slog output.
func (d DemoSession) String() string {
	return fmt.Sprintf("DemoSession{Token:%s TenantID:%s UserID:%s}", redacted, d.TenantID, d.User.ID)
}

func (d DemoSession) GoString() string { return d.String() }

func (d DemoSession) LogValue() slog.Value {
	return slog.GroupValue(slog.String("token", redacted), slog.String("tenant_id", d.TenantID), slog.String("user_id", d.User.ID))
}

// CreateDemo opens a session for role in tenantID (an existing trial) or in a new empty trial tenant.
// Every created id is generated here; the request never supplies one.
func (s *Sessions) CreateDemo(ctx context.Context, role access.Role, locale, tenantID string) (DemoSession, error) {
	if !s.cfg.DemoEnabled {
		return DemoSession{}, ErrDemoDisabled
	}
	if _, err := access.ParseRole(string(role)); err != nil {
		return DemoSession{}, &ValidationError{"role", "unknown role"}
	}
	if !locales[locale] {
		return DemoSession{}, &ValidationError{"locale", "must be vi or en"}
	}
	create := false
	if tenantID != "" && !validTenantID(tenantID) {
		return DemoSession{}, ErrTrialNotFound
	}
	if tenantID == "" {
		tenantID, create = s.ids.New(tenantPrefix), true
	}
	var out DemoSession
	err := s.uow.Do(ctx, tenantID, func(ctx context.Context, tx Tx) error {
		now := s.clock.Now()
		if err := s.prepareTenant(ctx, tx, tenantID, create, now); err != nil {
			return err
		}
		u, err := s.userFor(ctx, tx, role, locale)
		if err != nil {
			return err
		}
		if u.Locale != locale {
			if err = s.repo.SetLocale(ctx, tx, u.ID, locale); err != nil {
				return err
			}
			u.Locale = locale
		}
		out, err = s.openSession(ctx, tx, tenantID, u, now)
		return err
	})
	return out, err
}

// maxTenantIDLen bounds client input before it reaches the database.
const maxTenantIDLen = 64

// validTenantID is checked before any query; a bad shape answers like an unknown trial.
func validTenantID(id string) bool {
	if len(id) > maxTenantIDLen {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		isWord := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
		if !isWord {
			return false
		}
	}
	return true
}

func (s *Sessions) prepareTenant(ctx context.Context, tx Tx, id string, create bool, now time.Time) error {
	if create {
		if err := s.repo.CreateTrialTenant(ctx, tx, id, trialTenantName, now.Add(s.cfg.TrialTTL)); err != nil {
			return err
		}
		return s.seeder.Seed(ctx, tx, now)
	}
	found, err := s.repo.TrialTenant(ctx, tx, id, now)
	if err != nil {
		return err
	}
	if !found {
		return ErrTrialNotFound
	}
	return nil
}

func (s *Sessions) userFor(ctx context.Context, tx Tx, role access.Role, locale string) (User, error) {
	if u, ok, err := s.repo.UserByRole(ctx, tx, role); err != nil || ok {
		return u, err
	}
	u, err := s.repo.CreateUser(ctx, tx, s.ids.New(userPrefix), strings.ToLower(string(role)), role, locale)
	if !errors.Is(err, ErrConflict) {
		return u, err
	}
	// A concurrent demo request created the role user first; take theirs, once.
	u, ok, err := s.repo.UserByRole(ctx, tx, role)
	if err == nil && !ok {
		err = ErrConflict
	}
	return u, err
}

func (s *Sessions) openSession(ctx context.Context, tx Tx, tenantID string, u User, now time.Time) (DemoSession, error) {
	token, err := s.tokens.New()
	if err != nil {
		return DemoSession{}, fmt.Errorf("generate session token: %w", err)
	}
	exp := now.Add(s.cfg.SessionTTL)
	if err := s.repo.InsertSession(ctx, tx, HashToken(token), u.ID, exp); err != nil {
		return DemoSession{}, err
	}
	return DemoSession{Token: token, ExpiresAt: exp, TenantID: tenantID, User: u}, nil
}

// Authenticate turns a bearer token into a Caller. Expiry uses the injected clock only; a session is
// expired exactly at its expiry instant.
func (s *Sessions) Authenticate(ctx context.Context, token string) (Caller, error) {
	if token == "" {
		return Caller{}, ErrUnauthenticated
	}
	ref, err := s.resolver.Resolve(ctx, HashToken(token))
	if errors.Is(err, ErrSessionNotFound) {
		return Caller{}, ErrUnauthenticated
	}
	if err != nil {
		return Caller{}, err
	}
	if !s.clock.Now().Before(ref.ExpiresAt) {
		return Caller{}, ErrSessionExpired
	}
	var c Caller
	err = s.uow.Do(ctx, ref.TenantID, func(ctx context.Context, tx Tx) error {
		u, ok, err := s.repo.UserByID(ctx, tx, ref.UserID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrUnauthenticated
		}
		c = Caller{TenantID: ref.TenantID, UserID: u.ID, Role: u.Role, Locale: u.Locale}
		return nil
	})
	return c, err
}

// MeView is the signed-in user, the tenant and the access level per building.
type MeView struct {
	User           User
	Tenant         TenantInfo
	BuildingAccess map[string]access.Level
}

func (s *Sessions) Me(ctx context.Context, c Caller) (MeView, error) {
	var v MeView
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		u, ok, err := s.repo.UserByID(ctx, tx, c.UserID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrUnauthenticated
		}
		if v.Tenant, err = s.repo.TenantInfo(ctx, tx); err != nil {
			return err
		}
		ids, err := s.repo.BuildingIDs(ctx, tx)
		if err != nil {
			return err
		}
		v.User, v.BuildingAccess = u, map[string]access.Level{}
		for _, id := range ids {
			// FAST MODE (P5): staff act in every building; SG-501 replaces EDIT with the stored building permission.
			v.BuildingAccess[id] = access.EffectiveLevel(u.Role, access.EDIT)
		}
		return nil
	})
	return v, err
}

func (s *Sessions) SetLocale(ctx context.Context, c Caller, locale string) error {
	if !locales[locale] {
		return &ValidationError{"locale", "must be vi or en"}
	}
	return s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		return s.repo.SetLocale(ctx, tx, c.UserID, locale)
	})
}
