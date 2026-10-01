// Package crypto holds randomness-backed generators.
package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

const tokenBytes = 32 // 256 bits

// TokenGenerator makes session bearer tokens. The zero value reads crypto/rand.
type TokenGenerator struct {
	Reader io.Reader // tests only; nil means crypto/rand
}

// New returns 32 random bytes, URL-safe base64 without padding. It fails rather than return a weak token.
func (g TokenGenerator) New() (string, error) {
	r := g.Reader
	if r == nil {
		r = rand.Reader
	}
	b := make([]byte, tokenBytes)
	if _, err := io.ReadFull(r, b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
