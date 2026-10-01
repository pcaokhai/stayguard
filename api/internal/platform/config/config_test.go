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

func TestLoadAllowPrivilegedDB_SG003_AC3(t *testing.T) {
	c, err := Load(env(map[string]string{"DATABASE_URL": testDBURL}))
	if err != nil || c.AllowPrivilegedDB {
		t.Fatalf("default must be false: %+v err=%v", c, err)
	}
	c, err = Load(env(map[string]string{"DATABASE_URL": testDBURL, "ALLOW_PRIVILEGED_DB": "1"}))
	if err != nil || !c.AllowPrivilegedDB {
		t.Fatalf("1 must enable: %+v err=%v", c, err)
	}
	if _, err := Load(env(map[string]string{"DATABASE_URL": testDBURL, "ALLOW_PRIVILEGED_DB": "maybe"})); err == nil {
		t.Fatal("invalid value must be an error")
	}
}

func TestLoadSessionDefaults_SG102_AC1(t *testing.T) {
	c, err := Load(env(map[string]string{"DATABASE_URL": testDBURL}))
	if err != nil {
		t.Fatal(err)
	}
	if c.DemoMode || c.SessionTTL != 12*time.Hour || c.TrialTTL != 24*time.Hour {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	c, err = Load(env(map[string]string{"DATABASE_URL": testDBURL, "DEMO_MODE": "true", "SESSION_TTL_HOURS": "2", "TRIAL_TTL_HOURS": "48"}))
	if err != nil || !c.DemoMode || c.SessionTTL != 2*time.Hour || c.TrialTTL != 48*time.Hour {
		t.Fatalf("overrides not applied: %+v err=%v", c, err)
	}
}

func TestLoadSessionInvalid_SG102_AC2(t *testing.T) {
	for _, kv := range [][2]string{
		{"DEMO_MODE", "maybe"}, {"SESSION_TTL_HOURS", "0"}, {"SESSION_TTL_HOURS", "-1"}, {"SESSION_TTL_HOURS", "x"},
		{"TRIAL_TTL_HOURS", "0"}, {"TRIAL_TTL_HOURS", "1.5"},
	} {
		if _, err := Load(env(map[string]string{"DATABASE_URL": testDBURL, kv[0]: kv[1]})); err == nil {
			t.Errorf("expected error for %v", kv)
		}
	}
}
