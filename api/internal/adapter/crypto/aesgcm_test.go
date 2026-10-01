package crypto

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
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

func TestEncryptorAADBinding_SG203_AC4(t *testing.T) {
	e := newEnc(t, newKey(t))
	plain := []byte("x")
	pairs := [][2][2]string{
		{{"a", "bc"}, {"ab", "c"}},
		{{"a\x00b", "c"}, {"a", "b\x00c"}},
		{{"", "a"}, {"a", ""}},
	}
	for _, p := range pairs {
		ct, err := e.Encrypt(p[0][0], p[0][1], plain)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := e.Decrypt(p[1][0], p[1][1], ct); err == nil || got != nil {
			t.Errorf("%q and %q must not collide", p[0], p[1])
		}
	}
}

func TestEncryptorVersionAuthenticated_SG203_AC4(t *testing.T) {
	e := newEnc(t, newKey(t))
	ct, _ := e.Encrypt("t", "f", []byte("x"))
	// The version byte is also additional data, so a relabelled ciphertext fails the tag even if a
	// future reader accepted that version.
	e2 := *e
	if _, err := e2.open("t", "f", 2, ct); err == nil {
		t.Fatal("ciphertext relabelled with another version must not open")
	}
	if _, err := e2.open("t", "f", keyVersion, ct); err != nil {
		t.Fatalf("control: %v", err)
	}
}

func TestEncryptorKeyLength_SG203_AC4(t *testing.T) {
	for _, n := range []int{0, 16, 31, 33} {
		if _, err := NewAESGCM(make([]byte, n)); err == nil {
			t.Errorf("%d byte key must be rejected", n)
		}
	}
}

func TestFingerprint_SG203_AC4(t *testing.T) {
	key := newKey(t)
	e := newEnc(t, key)
	v := []byte("ID-MARKER-12345")
	base := e.Fingerprint("t1", "stays.id_number", v)
	if len(base) != 32 || !bytes.Equal(base, e.Fingerprint("t1", "stays.id_number", v)) {
		t.Fatal("fingerprint must be a deterministic 32-byte digest")
	}
	for name, other := range map[string][]byte{
		"tenant": e.Fingerprint("t2", "stays.id_number", v),
		"field":  e.Fingerprint("t1", "stays.other", v),
		"value":  e.Fingerprint("t1", "stays.id_number", []byte("ID-MARKER-12346")),
		"key":    newEnc(t, newKey(t)).Fingerprint("t1", "stays.id_number", v),
	} {
		if bytes.Equal(base, other) {
			t.Fatalf("different %s gave the same fingerprint", name)
		}
	}
	if plain := sha256.Sum256(v); bytes.Equal(base, plain[:]) {
		t.Fatal("fingerprint must be keyed, not a plain hash")
	}
	if bytes.Equal(e.Fingerprint("ab", "c", v), e.Fingerprint("a", "bc", v)) {
		t.Fatal("tenant/field boundary must be unambiguous")
	}
}

func TestFingerprintKeyDerivation_SG203_AC4(t *testing.T) {
	key := newKey(t)
	e := newEnc(t, key)
	v := []byte("ID-MARKER-12345")
	got := e.Fingerprint("t1", "f", v)
	mac := hmac.New(sha256.New, key) // keyed with the raw encryption key: must not match
	mac.Write(aad(keyVersion, "t1", "f"))
	mac.Write(v)
	if bytes.Equal(got, mac.Sum(nil)) {
		t.Fatal("fingerprint must not use the encryption key directly")
	}
	if bytes.Equal(got, e.fpKey) || bytes.Equal(e.fpKey, key) {
		t.Fatal("derived key must differ from the encryption key and the digest")
	}
}
