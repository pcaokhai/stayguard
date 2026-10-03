//go:build integration

package main

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

// The end-to-end tests live in package main because adapters may not import each other; only cmd
// wires the real router to the real Postgres adapters. This file is a small copy of the container
// helpers in adapter/postgres/testdb_test.go (test files cannot be imported across packages).
const (
	pgImage      = "postgres:17-alpine"
	appRole      = "stayguard_app"
	maintRole    = "stayguard_maint"
	testSecret   = "test-only"
	startTimeout = 3 * time.Minute
)

var (
	adminURL string
	baseURL  string
	dbSeq    int
	dbMu     sync.Mutex
)

func TestMain(m *testing.M) { os.Exit(runContainer(m)) }

func runContainer(m *testing.M) int {
	ctx, cancel := context.WithTimeout(context.Background(), startTimeout)
	defer cancel()
	ctr, err := tcpostgres.Run(ctx, pgImage,
		tcpostgres.WithDatabase("postgres"), tcpostgres.WithUsername("owner"),
		tcpostgres.WithPassword(testSecret), tcpostgres.BasicWaitStrategies())
	if err != nil {
		fmt.Fprintln(os.Stderr, "start postgres container (is Docker running?):", err)
		return 1
	}
	defer func() { _ = testcontainers.TerminateContainer(ctr) }()
	if adminURL, err = ctr.ConnectionString(ctx, "sslmode=disable"); err != nil {
		fmt.Fprintln(os.Stderr, "connection string:", err)
		return 1
	}
	baseURL = adminURL[:strings.LastIndex(adminURL, "/postgres?")]
	code := m.Run()
	if code == 0 {
		if bad := checkAuditContract(); len(bad) > 0 {
			fmt.Fprintln(os.Stderr, "audit-actions.json does not match the audit rows the tests wrote:\n  "+strings.Join(bad, "\n  "))
			return 1
		}
	}
	return code
}

func urlFor(db, user string) string {
	rest := strings.TrimPrefix(baseURL, "postgres://")
	host := rest[strings.LastIndex(rest, "@")+1:]
	return fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", user, testSecret, host, db)
}

// newMigratedDB creates a fresh migrated database with logins enabled and returns its name.
func newMigratedDB(t testing.TB) string {
	t.Helper()
	ctx := context.Background()
	dbMu.Lock()
	dbSeq++
	name := fmt.Sprintf("e2e%d", dbSeq)
	dbMu.Unlock()
	admin := connect(t, adminURL)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	sqlDB, err := sql.Open("pgx", urlFor(name, "owner"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		t.Fatalf("goose: %v", err)
	}
	if _, err = p.Up(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, r := range []string{appRole, maintRole} {
		if _, err := admin.Exec(ctx, fmt.Sprintf("ALTER ROLE %s LOGIN PASSWORD '%s'", r, testSecret)); err != nil {
			t.Fatalf("enable login %s: %v", r, err)
		}
	}
	return name
}

func connect(t testing.TB, url string) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	return conn
}
