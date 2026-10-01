package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
)

const dataKeyBytes = 32 // AES-256

// loadEncryption reads DATA_ENCRYPTION_KEY. Errors say which rule failed and never print the value.
func loadEncryption(c Config, getenv func(string) string) (Config, error) {
	v := getenv("DATA_ENCRYPTION_KEY")
	if v == "" {
		return Config{}, errors.New("DATA_ENCRYPTION_KEY is required")
	}
	key, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return Config{}, errors.New("DATA_ENCRYPTION_KEY must be standard base64")
	}
	if len(key) != dataKeyBytes {
		return Config{}, fmt.Errorf("DATA_ENCRYPTION_KEY must decode to exactly %d bytes", dataKeyBytes)
	}
	c.DataEncryptionKey = key
	return c, nil
}

// loadCheckIn reads the check-in slice flag (SG-203): FF_S2_CHECKIN, default off.
func loadCheckIn(c Config, getenv func(string) string) (Config, error) {
	if v := getenv("FF_S2_CHECKIN"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("FF_S2_CHECKIN must be a boolean, got %q", v)
		}
		c.CheckInEnabled = b
	}
	return c, nil
}
