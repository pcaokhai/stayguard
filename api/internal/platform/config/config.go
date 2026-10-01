// Package config reads typed, validated settings from the environment at start.
package config

import (
	"fmt"
	"log/slog"
	"strconv"
)

const (
	defaultPort      = 8080
	maxPort          = 65535
	defaultStaticDir = "web/out"
)

type Config struct {
	Port      int
	StaticDir string
	LogLevel  slog.Level
}

// Load takes the env lookup as a parameter so tests need no process environment.
func Load(getenv func(string) string) (Config, error) {
	c := Config{Port: defaultPort, StaticDir: defaultStaticDir, LogLevel: slog.LevelInfo}
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
	return c, nil
}
