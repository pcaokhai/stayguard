package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
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

// aad separates tenant and field with a zero byte so ("a","bc") and ("ab","c") differ.
func aad(tenantID, field string) []byte {
	return []byte(tenantID + "\x00" + field)
}

func (a *AESGCM) Encrypt(tenantID, field string, plaintext []byte) ([]byte, error) {
	nonce := make([]byte, a.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: nonce: %w", err)
	}
	out := append([]byte{keyVersion}, nonce...)
	return a.aead.Seal(out, nonce, plaintext, aad(tenantID, field)), nil
}

func (a *AESGCM) Decrypt(tenantID, field string, ciphertext []byte) ([]byte, error) {
	n := a.aead.NonceSize()
	if len(ciphertext) < 1+n+a.aead.Overhead() || ciphertext[0] != keyVersion {
		return nil, errDecrypt
	}
	plain, err := a.aead.Open(nil, ciphertext[1:1+n], ciphertext[1+n:], aad(tenantID, field))
	if err != nil {
		return nil, errDecrypt
	}
	return plain, nil
}
