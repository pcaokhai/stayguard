package app

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashToken is what is stored and looked up; the raw bearer token never leaves the response that creates it.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
