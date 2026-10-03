package config

import (
	"log/slog"
	"testing"
	"time"
)

// testKey is a throwaway all-zero key (base64 of 32 bytes); env adds it unless the case sets the variable itself.
const testKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

func env(m map[string]string) func(string) string {
	return func(k string) string {
		if v, ok := m[k]; ok || k != "DATA_ENCRYPTION_KEY" {
			return v
		}
		return testKey
	}
}

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
		{"SESSION_TTL_HOURS", "9999999999999"}, {"SESSION_TTL_HOURS", "8761"}, {"TRIAL_TTL_HOURS", "8761"},
	} {
		if _, err := Load(env(map[string]string{"DATABASE_URL": testDBURL, kv[0]: kv[1]})); err == nil {
			t.Errorf("expected error for %v", kv)
		}
	}
}

func TestLoadSessionCap_SG102_AC2(t *testing.T) {
	c, err := Load(env(map[string]string{"DATABASE_URL": testDBURL, "SESSION_TTL_HOURS": "8760"}))
	if err != nil || c.SessionTTL != 8760*time.Hour {
		t.Fatalf("cap itself must be accepted: %+v err=%v", c, err)
	}
}

func TestLoad_TrustedProxyHops_Hardening9(t *testing.T) {
	get := func(v string) (Config, error) {
		m := map[string]string{"DATABASE_URL": testDBURL}
		if v != "" {
			m["TRUSTED_PROXY_HOPS"] = v
		}
		return Load(env(m))
	}
	if c, err := get(""); err != nil || c.TrustedProxyHops != 1 {
		t.Fatalf("default: %+v %v", c, err)
	}
	if c, err := get("2"); err != nil || c.TrustedProxyHops != 2 {
		t.Fatalf("two: %+v %v", c, err)
	}
	for _, bad := range []string{"0", "-1", "x", "99"} {
		if _, err := get(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestLoad_SepayTimestampTolerance(t *testing.T) {
	get := func(v string) (Config, error) {
		m := map[string]string{"DATABASE_URL": testDBURL}
		if v != "" {
			m["SEPAY_TIMESTAMP_TOLERANCE"] = v
		}
		return Load(env(m))
	}
	if c, err := get(""); err != nil || c.SepayTimestampTolerance != 300*time.Second {
		t.Fatalf("default: %v %v", c.SepayTimestampTolerance, err)
	}
	if c, err := get("15m"); err != nil || c.SepayTimestampTolerance != 15*time.Minute {
		t.Fatalf("15m: %v %v", c.SepayTimestampTolerance, err)
	}
	for _, bad := range []string{"0s", "-1s", "x", "48h", "300"} {
		if _, err := get(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// Rehearsal only: SIGNIN_RATE_PER_IP and SIGNIN_RATE_PER_CODE raise the sign-in limits (per minute); unset they are the production values.
func TestLoadSignInRateOverride_FU(t *testing.T) {
	c, err := Load(env(map[string]string{"DATABASE_URL": testDBURL}))
	if err != nil || c.SignInPerIP != 20 || c.SignInPerCode != 60 || c.SignInRateRaised() {
		t.Fatalf("defaults: %+v %v", c, err)
	}
	c, err = Load(env(map[string]string{"DATABASE_URL": testDBURL, "SIGNIN_RATE_PER_IP": "5000", "SIGNIN_RATE_PER_CODE": "9000"}))
	if err != nil || c.SignInPerIP != 5000 || c.SignInPerCode != 9000 || !c.SignInRateRaised() {
		t.Fatalf("override: %+v %v", c, err)
	}
	for _, bad := range []string{"0", "-1", "many", "1000001"} {
		if _, err := Load(env(map[string]string{"DATABASE_URL": testDBURL, "SIGNIN_RATE_PER_IP": bad})); err == nil {
			t.Errorf("SIGNIN_RATE_PER_IP=%q must be refused", bad)
		}
	}
}
