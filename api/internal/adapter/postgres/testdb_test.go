//go:build integration

package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver for goose
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/pcaokhai/stayguard/api/migrations"
)

const (
	pgImage      = "postgres:17-alpine"
	appRole      = "stayguard_app"
	maintRole    = "stayguard_maint"
	testSecret   = "test-only" // roles have no password in migrations; the test container sets one
	dialTimeout  = 30 * time.Second
	startTimeout = 3 * time.Minute
)

var (
	adminURL string // superuser, the migration owner in tests
	baseURL  string // admin URL without database name, for per-test databases
	dbSeq    int
	dbMu     sync.Mutex

	sharedOnce sync.Once
	sharedName string
	sharedErr  error
)

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

// run starts one PostgreSQL container for the package and always terminates it.
func run(m *testing.M) int {
	ctr, url, err := startPostgres()
	if err != nil {
		fmt.Fprintln(os.Stderr, "start postgres container (is Docker running?):", err)
		return 1
	}
	defer func() { _ = testcontainers.TerminateContainer(ctr) }()
	adminURL = url
	baseURL = adminURL[:strings.LastIndex(adminURL, "/postgres?")]
	return m.Run()
}

// startPostgres starts a container and returns it with the superuser URL.
func startPostgres() (*tcpostgres.PostgresContainer, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), startTimeout)
	defer cancel()
	ctr, err := tcpostgres.Run(ctx, pgImage,
		tcpostgres.WithDatabase("postgres"),
		tcpostgres.WithUsername("owner"),
		tcpostgres.WithPassword(testSecret),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, "", err
	}
	url, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = testcontainers.TerminateContainer(ctr)
		return nil, "", err
	}
	return ctr, url, nil
}

// urlFor builds a connection URL for a database and role.
func urlFor(db, user, pass string) string {
	rest := strings.TrimPrefix(baseURL, "postgres://")
	host := rest[strings.LastIndex(rest, "@")+1:]
	return fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", user, pass, host, db)
}

// newEmptyDB creates an empty database owned by the superuser and returns its name.
func newEmptyDB(t testing.TB) string {
	t.Helper()
	dbMu.Lock()
	dbSeq++
	name := fmt.Sprintf("t%d", dbSeq)
	dbMu.Unlock()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	defer func() { _ = conn.Close(ctx) }()
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	return name
}

// migrate applies all embedded migrations and returns how many ran.
func migrate(ctx context.Context, db string) (int, error) {
	return migrateURL(ctx, urlFor(db, "owner", testSecret))
}

func migrateURL(ctx context.Context, url string) (int, error) {
	sqlDB, err := sql.Open("pgx", url)
	if err != nil {
		return 0, err
	}
	defer func() { _ = sqlDB.Close() }()
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		return 0, err
	}
	res, err := p.Up(ctx)
	return len(res), err
}

// migratedDB returns a database migrated once per package and gives the roles a login.
func migratedDB(t testing.TB) string {
	t.Helper()
	sharedOnce.Do(func() {
		sharedName = newEmptyDB(t)
		ctx := context.Background()
		if _, sharedErr = migrate(ctx, sharedName); sharedErr != nil {
			return
		}
		sharedErr = enableLogins(ctx)
	})
	if sharedErr != nil {
		t.Fatalf("prepare shared database: %v", sharedErr)
	}
	return sharedName
}

// enableLogins sets a password on the NOLOGIN roles; migrations never carry secrets.
func enableLogins(ctx context.Context) error {
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()
	for _, r := range []string{appRole, maintRole} {
		if _, err := conn.Exec(ctx, fmt.Sprintf("ALTER ROLE %s LOGIN PASSWORD '%s'", r, testSecret)); err != nil {
			return err
		}
	}
	return nil
}

// connAs opens a connection to db as the given role (owner, appRole or maintRole).
func connAs(t testing.TB, db, role string) *pgx.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()
	conn, err := pgx.Connect(ctx, urlFor(db, role, testSecret))
	if err != nil {
		t.Fatalf("connect as %s: %v", role, err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	return conn
}

// connAtURL opens a connection to an arbitrary URL.
func connAtURL(t testing.TB, url string) *pgx.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	return conn
}
