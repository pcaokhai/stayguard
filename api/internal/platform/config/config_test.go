package config

import (
	"log/slog"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestLoadDefaults_SG001_AC2(t *testing.T) {
	c, err := Load(env(map[string]string{"DATABASE_URL": testDBURL}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != 8080 || c.StaticDir != "web/out" || c.LogLevel != slog.LevelInfo {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestLoadOverrides_SG001_AC2(t *testing.T) {
	c, err := Load(env(map[string]string{"DATABASE_URL": testDBURL, "PORT": "9090", "STATIC_DIR": "/srv/web", "LOG_LEVEL": "debug"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != 9090 || c.StaticDir != "/srv/web" || c.LogLevel != slog.LevelDebug {
		t.Fatalf("unexpected config: %+v", c)
	}
}

func TestLoadInvalid_SG001_AC2(t *testing.T) {
	for _, m := range []map[string]string{
		{"DATABASE_URL": testDBURL, "PORT": "abc"}, {"DATABASE_URL": testDBURL, "PORT": "0"},
		{"DATABASE_URL": testDBURL, "PORT": "70000"}, {"DATABASE_URL": testDBURL, "LOG_LEVEL": "loud"},
	} {
		if _, err := Load(env(m)); err == nil {
			t.Errorf("expected error for %v", m)
		}
	}
}

const testDBURL = "postgres://app@db/stayguard"

func TestLoadDatabase_SG003_AC6(t *testing.T) {
	c, err := Load(env(map[string]string{"DATABASE_URL": testDBURL}))
	if err != nil {
		t.Fatal(err)
	}
	if c.DatabaseURL != testDBURL || c.MigrateDatabaseURL != testDBURL || c.IdempotencyTTL != 24*time.Hour {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	c, err = Load(env(map[string]string{
		"DATABASE_URL": testDBURL, "MIGRATE_DATABASE_URL": "postgres://owner@db/stayguard", "IDEMPOTENCY_TTL": "1h",
	}))
	if err != nil || c.MigrateDatabaseURL != "postgres://owner@db/stayguard" || c.IdempotencyTTL != time.Hour {
		t.Fatalf("overrides not applied: %+v err=%v", c, err)
	}
}

func TestLoadDatabaseInvalid_SG003_AC6(t *testing.T) {
	for _, m := range []map[string]string{
		{}, {"DATABASE_URL": testDBURL, "IDEMPOTENCY_TTL": "soon"}, {"DATABASE_URL": testDBURL, "IDEMPOTENCY_TTL": "0s"},
	} {
		if _, err := Load(env(m)); err == nil {
			t.Errorf("expected error for %v", m)
		}
	}
}
