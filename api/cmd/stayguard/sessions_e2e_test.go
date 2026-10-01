//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	httpadapter "github.com/pcaokhai/stayguard/api/internal/adapter/http"
	"github.com/pcaokhai/stayguard/api/internal/adapter/ids"
	"github.com/pcaokhai/stayguard/api/internal/adapter/permissions"
	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/platform/config"
)

const (
	sessionTTL = 12 * time.Hour
	trialTTL   = 24 * time.Hour
)

// fakeClock is the server clock the tests move; the real router and adapters read it.
type fakeClock struct {
	mu   sync.Mutex
	t    time.Time
	step time.Duration // when set, every Now() call moves the clock forward by it
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.t
	c.t = c.t.Add(c.step)
	return now
}
func (c *fakeClock) set(t time.Time) { c.mu.Lock(); c.t = t; c.mu.Unlock() }

type env struct {
	t     *testing.T
	srv   *httptest.Server
	clock *fakeClock
	pool  *pgxpool.Pool
	owner *pgx.Conn // table owner: seeds and inspects rows the app role cannot see
	start time.Time
	logs  *syncBuffer // everything the server logged, for the personal-data checks
}

// newEnv wires the real router to a migrated database; the server runs as the application role.
func newEnv(t *testing.T) *env {
	t.Helper()
	return newEnvRooms(t, newRooms)
}

// emptyTrial leaves new trial tenants empty, which most fixtures assume; the seed test uses newSeededEnv.
type emptyTrial struct{}

func (emptyTrial) Seed(context.Context, app.Tx, time.Time) error { return nil }

// newEnvRooms is newEnv with the room use cases built by mk, so a test can swap one port.
func newEnvRooms(t *testing.T, mk func(app.UnitOfWork, app.Clock) *app.Rooms) *env {
	t.Helper()
	return newEnvWith(t, mk, nil)
}

// newEnvWith builds the env; a nil seeder means empty trial tenants.
func newEnvWith(t *testing.T, mk func(app.UnitOfWork, app.Clock) *app.Rooms, seeder app.TrialSeeder) *env {
	t.Helper()
	if seeder == nil {
		seeder = emptyTrial{}
	}
	db := newMigratedDB(t)
	pool, err := postgres.NewPool(context.Background(), postgres.PoolConfig{URL: urlFor(db, appRole), MaxConns: 4})
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	start := time.Now().UTC().Truncate(time.Second) // whole seconds: timestamptz keeps microseconds
	clk := &fakeClock{t: start}
	cfg := config.Config{DemoMode: true, SessionTTL: sessionTTL, TrialTTL: trialTTL, DataEncryptionKey: testDataKey, CheckInEnabled: true, CheckoutEnabled: true}
	uow := postgres.NewUnitOfWork(pool)
	sessions := newSessionsWith(cfg, pool, uow, clk, seeder)
	stays, err := newStays(cfg, uow, postgres.NewIdempotencyStore(0), postgres.NewAuditWriter(), clk)
	if err != nil {
		t.Fatalf("stays: %v", err)
	}
	billing, err := newBilling(cfg, uow, postgres.NewIdempotencyStore(0), postgres.NewAuditWriter(), clk)
	if err != nil {
		t.Fatalf("billing: %v", err)
	}
	payments, err := newPayments(cfg, uow, postgres.NewIdempotencyStore(0), postgres.NewAuditWriter(), clk)
	if err != nil {
		t.Fatalf("payments: %v", err)
	}
	logs := &syncBuffer{}
	h := httpadapter.NewRouter(slog.New(slog.NewJSONHandler(logs, nil)), httpadapter.Options{
		Probe: postgres.NewReadinessProbe(pool), Sessions: sessions, DemoEnabled: true,
		Rooms: mk(uow, clk), RoomMapEnabled: true, Stays: stays, CheckInEnabled: true, Billing: billing, CheckoutEnabled: true, Payments: payments,
		Owner:        app.NewOwner(uow, postgres.OwnerRepo{}, mk(uow, clk), clk),
		Housekeeping: app.NewHousekeeping(uow, postgres.HousekeepingRepo{}, permissions.RoleBased{}, postgres.NewAuditWriter(), ids.New(clk.Now), clk),
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &env{t: t, pool: pool, srv: srv, clock: clk, owner: connect(t, urlFor(db, "owner")), start: start, logs: logs}
}

type reply struct {
	status int
	body   map[string]any
}

func (r reply) str(path ...string) string {
	var cur any = r.body
	for _, k := range path {
		m, _ := cur.(map[string]any)
		cur = m[k]
	}
	s, _ := cur.(string)
	return s
}

func (e *env) call(method, path, token string, body any) reply {
	e.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, e.srv.URL+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(res.Body)
	out := reply{status: res.StatusCode, body: map[string]any{}}
	_ = json.Unmarshal(raw, &out.body)
	return out
}

func (e *env) demo(role, locale, tenantID string) reply {
	body := map[string]any{"role": role, "locale": locale}
	if tenantID != "" {
		body["tenantId"] = tenantID
	}
	return e.call("POST", "/v1/demo/sessions", "", body)
}

func (e *env) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.owner.Exec(context.Background(), sql, args...); err != nil {
		e.t.Fatalf("exec %q: %v", sql, err)
	}
}

func (e *env) seedBuildings(tenant string, ids ...string) {
	e.t.Helper()
	e.exec(`INSERT INTO app.properties (id, tenant_id, name) VALUES ($1, $2, 'P')`, "pr_"+tenant, tenant)
	for _, id := range ids {
		e.exec(`INSERT INTO app.buildings (id, tenant_id, property_id, code, name) VALUES ($1, $2, $3, $1, $1)`, id, tenant, "pr_"+tenant)
	}
}

func (e *env) count(sql string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.owner.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		e.t.Fatalf("count: %v", err)
	}
	return n
}

func TestCreateDemoSession_SG102_AC1(t *testing.T) {
	e := newEnv(t)
	for _, role := range []string{"OWNER", "RECEPTIONIST", "HOUSEKEEPING"} {
		for _, locale := range []string{"vi", "en"} {
			s := e.demo(role, locale, "")
			if s.status != 201 || s.str("accessToken") == "" || s.str("tenantId") == "" || s.str("expiresAt") == "" {
				t.Fatalf("%s/%s: status=%d body=%v", role, locale, s.status, s.body)
			}
			me := e.call("GET", "/v1/me", s.str("accessToken"), nil)
			if me.status != 200 || me.str("user", "role") != role || me.str("user", "locale") != locale ||
				me.str("tenant", "id") != s.str("tenantId") {
				t.Fatalf("%s/%s: me status=%d body=%v", role, locale, me.status, me.body)
			}
		}
	}
	first := e.demo("OWNER", "vi", "")
	tenant := first.str("tenantId")
	joined := e.demo("RECEPTIONIST", "vi", tenant)
	if joined.status != 201 || joined.str("tenantId") != tenant {
		t.Fatalf("join: status=%d body=%v", joined.status, joined.body)
	}
	if n := e.count(`SELECT count(*) FROM app.users WHERE tenant_id = $1`, tenant); n != 2 {
		t.Errorf("users in joined tenant = %d, want 2", n)
	}
}

func TestCreateDemoSessionTenantRules_SG102_AC1(t *testing.T) {
	e := newEnv(t)
	e.exec(`INSERT INTO app.tenants (id, name) VALUES ('tn_real', 'Real business')`) // not a trial
	expiring := e.demo("OWNER", "vi", "").str("tenantId")
	e.clock.set(e.start.Add(trialTTL + time.Minute))

	cases := map[string]map[string]any{
		"unknown":    {"role": "OWNER", "locale": "vi", "tenantId": "tn_unknown"},
		"non-trial":  {"role": "OWNER", "locale": "vi", "tenantId": "tn_real"},
		"expired":    {"role": "OWNER", "locale": "vi", "tenantId": expiring},
		"bad shape":  {"role": "OWNER", "locale": "vi", "tenantId": "x'; DROP TABLE app.users;--"},
		"client ids": {"role": "OWNER", "locale": "vi", "tenantId": "tn_real", "id": "us_evil"},
	}
	for name, body := range cases {
		r := e.call("POST", "/v1/demo/sessions", "", body)
		if r.status != 404 || r.str("code") != "TRIAL_NOT_FOUND" {
			t.Errorf("%s: status=%d body=%v", name, r.status, r.body)
		}
	}
	// A client-supplied id never becomes a primary key, even on a successful request.
	ok := e.call("POST", "/v1/demo/sessions", "", map[string]any{"role": "OWNER", "locale": "vi", "id": "us_evil", "tenant": map[string]any{"id": "tn_evil"}})
	if ok.status != 201 {
		t.Fatalf("status=%d body=%v", ok.status, ok.body)
	}
	if n := e.count(`SELECT count(*) FROM app.users WHERE id = 'us_evil'`) + e.count(`SELECT count(*) FROM app.tenants WHERE id = 'tn_evil'`); n != 0 {
		t.Errorf("client-supplied id reached the database")
	}
	if n := e.count(`SELECT count(*) FROM app.users WHERE tenant_id = 'tn_real'`); n != 0 {
		t.Errorf("non-trial tenant gained a user")
	}
}

func TestTenantContext_SG102_AC4(t *testing.T) {
	e := newEnv(t)
	a := e.demo("OWNER", "vi", "")
	b := e.demo("OWNER", "vi", "")
	ta, tb := a.str("tenantId"), b.str("tenantId")
	e.seedBuildings(ta, "bld_a")
	e.seedBuildings(tb, "bld_b")

	meA := e.call("GET", "/v1/me", a.str("accessToken"), nil)
	buildings, _ := meA.body["buildingAccess"].([]any)
	if meA.str("tenant", "id") != ta || len(buildings) != 1 || buildings[0].(map[string]any)["buildingId"] != "bld_a" {
		t.Fatalf("tenant A me: %v", meA.body)
	}
	if got := e.call("GET", "/v1/me", b.str("accessToken"), nil).str("tenant", "id"); got != tb {
		t.Errorf("tenant B me returned %q", got)
	}
	rec := e.demo("RECEPTIONIST", "vi", ta)
	meRec := e.call("GET", "/v1/me", rec.str("accessToken"), nil)
	if meRec.str("tenant", "id") != ta || meRec.str("user", "role") != "RECEPTIONIST" {
		t.Errorf("joined session: %v", meRec.body)
	}
}

func TestGetMeAndLocale_SG102_AC3(t *testing.T) {
	e := newEnv(t)
	owner := e.demo("OWNER", "vi", "")
	tenant := owner.str("tenantId")
	e.seedBuildings(tenant, "bld_1", "bld_2")
	staff := e.demo("RECEPTIONIST", "vi", tenant)

	for token, want := range map[string]string{owner.str("accessToken"): "EDIT", staff.str("accessToken"): "NONE"} {
		me := e.call("GET", "/v1/me", token, nil)
		levels, _ := me.body["buildingAccess"].([]any)
		if len(levels) != 2 {
			t.Fatalf("buildingAccess = %v", me.body["buildingAccess"])
		}
		for _, l := range levels {
			if l.(map[string]any)["level"] != want {
				t.Errorf("level = %v, want %s", l, want)
			}
		}
		if me.str("user", "id") == "" || me.str("user", "name") == "" || me.str("tenant", "name") == "" ||
			me.str("tenant", "timezone") == "" || me.str("tenant", "currency") != "VND" {
			t.Errorf("incomplete me: %v", me.body)
		}
	}
	tok := staff.str("accessToken")
	if r := e.call("PUT", "/v1/me/locale", tok, map[string]any{"locale": "en"}); r.status != 204 {
		t.Fatalf("set locale status=%d body=%v", r.status, r.body)
	}
	if got := e.call("GET", "/v1/me", tok, nil).str("user", "locale"); got != "en" {
		t.Errorf("locale after PUT = %q", got)
	}
	if r := e.call("PUT", "/v1/me/locale", tok, map[string]any{"locale": "fr"}); r.status != 422 || r.str("code") != "VALIDATION_FAILED" {
		t.Errorf("invalid locale: status=%d body=%v", r.status, r.body)
	}
	if r := e.call("PUT", "/v1/me/locale", "", map[string]any{"locale": "en"}); r.status != 401 {
		t.Errorf("no token: status=%d", r.status)
	}
}

func TestSessionExpiry_SG102_AC2(t *testing.T) {
	e := newEnv(t)
	s := e.demo("OWNER", "vi", "")
	token := s.str("accessToken")

	e.clock.set(e.start.Add(sessionTTL - time.Second))
	if r := e.call("GET", "/v1/me", token, nil); r.status != 200 {
		t.Fatalf("just before expiry: status=%d", r.status)
	}
	e.clock.set(e.start.Add(sessionTTL)) // exactly at expiry is expired
	if r := e.call("GET", "/v1/me", token, nil); r.status != 401 || r.str("code") != "SESSION_EXPIRED" {
		t.Fatalf("at expiry: status=%d body=%v", r.status, r.body)
	}
	if r := e.call("GET", "/v1/me", "not-a-real-token", nil); r.status != 401 || r.str("code") != "UNAUTHENTICATED" {
		t.Errorf("unknown token: status=%d body=%v", r.status, r.body)
	}
	if n := e.count(`SELECT count(*) FROM app.sessions WHERE token_hash = $1`, token); n != 0 {
		t.Errorf("raw token stored")
	}
	if n := e.count(`SELECT count(*) FROM app.sessions WHERE token_hash = $1`, app.HashToken(token)); n != 1 {
		t.Errorf("hashed token rows = %d, want 1", n)
	}
}

// The server under test must run as the least-privileged application role, or RLS would be bypassed
// and these tests would prove nothing about tenant isolation.
func TestServerRunsAsAppRole_SG102_AC4(t *testing.T) {
	e := newEnv(t)
	var user string
	var bypass, super bool
	err := e.pool.QueryRow(context.Background(),
		`SELECT current_user, rolbypassrls, rolsuper FROM pg_roles WHERE rolname = current_user`).Scan(&user, &bypass, &super)
	if err != nil {
		t.Fatalf("query role: %v", err)
	}
	if user != appRole || bypass || super {
		t.Fatalf("server role = %s bypassrls=%v superuser=%v, want %s without privileges", user, bypass, super, appRole)
	}
}
