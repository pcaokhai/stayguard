package config

import (
	"fmt"
	"strconv"
	"time"
)

const (
	defaultSessionTTLHours = 12
	defaultTrialTTLHours   = 24
	// maxTTLHours (one year) keeps hours*time.Hour far from Duration overflow.
	maxTTLHours = 24 * 365
)

// loadSessions reads the demo-session settings (SG-102): DEMO_MODE, SESSION_TTL_HOURS, TRIAL_TTL_HOURS.
func loadSessions(c Config, getenv func(string) string) (Config, error) {
	if v := getenv("DEMO_MODE"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("DEMO_MODE must be a boolean, got %q", v)
		}
		c.DemoMode = b
	}
	var err error
	if c.SessionTTL, err = hoursEnv(getenv, "SESSION_TTL_HOURS", defaultSessionTTLHours); err != nil {
		return Config{}, err
	}
	if c.TrialTTL, err = hoursEnv(getenv, "TRIAL_TTL_HOURS", defaultTrialTTLHours); err != nil {
		return Config{}, err
	}
	return c, nil
}

func hoursEnv(getenv func(string) string, key string, def int) (time.Duration, error) {
	v := getenv(key)
	if v == "" {
		return time.Duration(def) * time.Hour, nil
	}
	h, err := strconv.Atoi(v)
	if err != nil || h < 1 || h > maxTTLHours {
		return 0, fmt.Errorf("%s must be a whole number of hours in 1..%d, got %q", key, maxTTLHours, v)
	}
	return time.Duration(h) * time.Hour, nil
}
