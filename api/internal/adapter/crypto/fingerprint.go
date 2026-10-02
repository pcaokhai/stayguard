package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// keyCheckLabel keeps the key fingerprint apart from the pepper and every other use of the key.
const keyCheckLabel = "stayguard/key-fingerprint/v1"

// KeyFingerprint is a short keyed digest of the data encryption key. It is stored beside the data so a different key
// is noticed at start; it cannot be turned back into the key.
func KeyFingerprint(dataKey []byte) string {
	m := hmac.New(sha256.New, dataKey)
	m.Write([]byte(keyCheckLabel))
	return hex.EncodeToString(m.Sum(nil)[:16])
}
