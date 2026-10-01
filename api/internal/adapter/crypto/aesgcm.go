package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// keyVersion prefixes every ciphertext so a later key rotation can tell keys apart.
const keyVersion byte = 1

const keyBytes = 32 // AES-256

// errDecrypt is deliberately generic: it must never reveal which check failed or any bytes.
var errDecrypt = errors.New("crypto: cannot decrypt")

// AESGCM encrypts with AES-256-GCM. Layout: version byte, nonce, sealed bytes.
type AESGCM struct {
	aead cipher.AEAD
}

func NewAESGCM(key []byte) (*AESGCM, error) {
	if len(key) != keyBytes {
		return nil, fmt.Errorf("crypto: key must be %d bytes", keyBytes)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: gcm: %w", err)
	}
	return &AESGCM{aead: aead}, nil
}

// aad is version, then each part behind a 4-byte big-endian length, so no two (tenant, field)
// pairs share an encoding whatever bytes they hold, and the version is authenticated.
func aad(version byte, tenantID, field string) []byte {
	out := []byte{version}
	for _, p := range []string{tenantID, field} {
		out = binary.BigEndian.AppendUint32(out, uint32(len(p))) //nolint:gosec // tenant ids and field names are short constants
		out = append(out, p...)
	}
	return out
}

func (a *AESGCM) Encrypt(tenantID, field string, plaintext []byte) ([]byte, error) {
	nonce := make([]byte, a.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: nonce: %w", err)
	}
	out := append([]byte{keyVersion}, nonce...)
	return a.aead.Seal(out, nonce, plaintext, aad(keyVersion, tenantID, field)), nil
}

func (a *AESGCM) Decrypt(tenantID, field string, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 || ciphertext[0] != keyVersion {
		return nil, errDecrypt
	}
	return a.open(tenantID, field, ciphertext[0], ciphertext)
}

// open authenticates with the given version as additional data (kept separate so tests can relabel).
func (a *AESGCM) open(tenantID, field string, version byte, ciphertext []byte) ([]byte, error) {
	n := a.aead.NonceSize()
	if len(ciphertext) < 1+n+a.aead.Overhead() {
		return nil, errDecrypt
	}
	plain, err := a.aead.Open(nil, ciphertext[1:1+n], ciphertext[1+n:], aad(version, tenantID, field))
	if err != nil {
		return nil, errDecrypt
	}
	return plain, nil
}
