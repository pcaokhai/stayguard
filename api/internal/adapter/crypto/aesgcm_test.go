package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/app"
)

var _ app.Encryptor = (*AESGCM)(nil)

func newKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return k
}

func newEnc(t *testing.T, key []byte) *AESGCM {
	t.Helper()
	e, err := NewAESGCM(key)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestEncryptorAdapter_SG203_AC4(t *testing.T) {
	key := newKey(t)
	e := newEnc(t, key)
	plain := []byte("ID-MARKER-12345")
	ct, err := e.Encrypt("t1", "id_number", plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ct, plain) || ct[0] != keyVersion {
		t.Fatal("ciphertext leaks plaintext or lacks version byte")
	}

	t.Run("round trip", func(t *testing.T) {
		got, err := e.Decrypt("t1", "id_number", ct)
		if err != nil || !bytes.Equal(got, plain) {
			t.Fatalf("got %q err=%v", got, err)
		}
	})
	t.Run("two encryptions differ", func(t *testing.T) {
		ct2, err := e.Encrypt("t1", "id_number", plain)
		if err != nil || bytes.Equal(ct, ct2) {
			t.Fatalf("ciphertexts must differ: err=%v", err)
		}
	})

	tampered := func(i int) []byte { c := bytes.Clone(ct); c[i] ^= 1; return c }
	otherKey := newEnc(t, newKey(t))
	fails := map[string]func() ([]byte, error){
		"tamper nonce":    func() ([]byte, error) { return e.Decrypt("t1", "id_number", tampered(2)) },
		"tamper body":     func() ([]byte, error) { return e.Decrypt("t1", "id_number", tampered(len(ct)-1)) },
		"unknown version": func() ([]byte, error) { return e.Decrypt("t1", "id_number", tampered(0)) },
		"truncated":       func() ([]byte, error) { return e.Decrypt("t1", "id_number", ct[:len(ct)-1]) },
		"too short":       func() ([]byte, error) { return e.Decrypt("t1", "id_number", ct[:5]) },
		"nil":             func() ([]byte, error) { return e.Decrypt("t1", "id_number", nil) },
		"wrong key":       func() ([]byte, error) { return otherKey.Decrypt("t1", "id_number", ct) },
		"wrong tenant":    func() ([]byte, error) { return e.Decrypt("t2", "id_number", ct) },
		"wrong field":     func() ([]byte, error) { return e.Decrypt("t1", "bank_account", ct) },
	}
	for name, f := range fails {
		t.Run(name, func(t *testing.T) {
			got, err := f()
			if err == nil || got != nil {
				t.Fatalf("must fail closed: got %q err=%v", got, err)
			}
			if bytes.Contains([]byte(err.Error()), plain) {
				t.Fatal("error leaks plaintext")
			}
		})
	}
}

func TestEncryptorKeyLength_SG203_AC4(t *testing.T) {
	for _, n := range []int{0, 16, 31, 33} {
		if _, err := NewAESGCM(make([]byte, n)); err == nil {
			t.Errorf("%d byte key must be rejected", n)
		}
	}
}
