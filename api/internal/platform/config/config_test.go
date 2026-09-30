package config

import (
	"log/slog"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestLoadDefaults_SG001_AC2(t *testing.T) {
	c, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != 8080 || c.StaticDir != "web/out" || c.LogLevel != slog.LevelInfo {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestLoadOverrides_SG001_AC2(t *testing.T) {
	c, err := Load(env(map[string]string{"PORT": "9090", "STATIC_DIR": "/srv/web", "LOG_LEVEL": "debug"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != 9090 || c.StaticDir != "/srv/web" || c.LogLevel != slog.LevelDebug {
		t.Fatalf("unexpected config: %+v", c)
	}
}

func TestLoadInvalid_SG001_AC2(t *testing.T) {
	for _, m := range []map[string]string{
		{"PORT": "abc"}, {"PORT": "0"}, {"PORT": "70000"}, {"LOG_LEVEL": "loud"},
	} {
		if _, err := Load(env(m)); err == nil {
			t.Errorf("expected error for %v", m)
		}
	}
}
