package config

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

func randKey(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(b)
}

func TestConfigEncryptionKey_SG203_AC4(t *testing.T) {
	good := randKey(t, 32)
	c, err := Load(env(map[string]string{"DATABASE_URL": testDBURL, "DATA_ENCRYPTION_KEY": good}))
	if err != nil || len(c.DataEncryptionKey) != 32 {
		t.Fatalf("valid key must load: %v", err)
	}
	bad := map[string]string{
		"missing":    "",
		"bad base64": "not base64 !!" + good,
		"31 bytes":   randKey(t, 31),
		"33 bytes":   randKey(t, 33),
	}
	for name, v := range bad {
		_, err := Load(env(map[string]string{"DATABASE_URL": testDBURL, "DATA_ENCRYPTION_KEY": v}))
		if err == nil {
			t.Errorf("%s: must fail startup", name)
			continue
		}
		if v != "" && strings.Contains(err.Error(), v) {
			t.Errorf("%s: error prints the key", name)
		}
	}
}

func TestConfigMigrateNeedsNoKey_SG203_AC4(t *testing.T) {
	c, err := LoadMigrate(env(map[string]string{"DATABASE_URL": testDBURL}))
	if err != nil || c.DatabaseURL != testDBURL {
		t.Fatalf("migrate must not need the key: %v", err)
	}
	if _, err = LoadMigrate(env(map[string]string{})); err == nil {
		t.Fatal("migrate still needs the database URL")
	}
}

func TestCheckInConfig_SG203_AC1(t *testing.T) {
	base := map[string]string{"DATABASE_URL": testDBURL, "DATA_ENCRYPTION_KEY": randKey(t, 32)}
	with := func(v string) map[string]string {
		m := map[string]string{"FF_S2_CHECKIN": v}
		for k, x := range base {
			m[k] = x
		}
		return m
	}
	if c, err := Load(env(base)); err != nil || c.CheckInEnabled {
		t.Fatalf("default must be off: %v", err)
	}
	if c, err := Load(env(with("true"))); err != nil || !c.CheckInEnabled {
		t.Fatalf("true not applied: %v", err)
	}
	if _, err := Load(env(with("maybe"))); err == nil {
		t.Fatal("invalid value must fail startup")
	}
}
