package crypto

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

var testKey = []byte("0123456789abcdef0123456789abcdef")

func TestPinHasher_SG701_AC4(t *testing.T) {
	h := NewPinHasher(testKey)
	a, err := h.Hash("482915")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := h.Hash("482915")
	if strings.Contains(a, "482915") || !strings.HasPrefix(a, "p1$$2") || a == b {
		t.Fatalf("want a salted peppered bcrypt hash, got %q and %q", a, b)
	}
	if !h.Verify(a, "482915") || h.Verify(a, "482916") || h.Verify("not a hash", "482915") || h.NeedsRehash(a) {
		t.Fatal("verify")
	}
}

// A hash is useless without the key: another key does not verify it, and plain bcrypt of the PIN does not either.
func TestPinHasher_Pepper_Hardening1(t *testing.T) {
	h := NewPinHasher(testKey)
	hash, _ := h.Hash("482915")
	if NewPinHasher([]byte("another-key-another-key-another!")).Verify(hash, "482915") {
		t.Fatal("a different key must not verify")
	}
	rest := strings.TrimPrefix(hash, "p1$")
	if bcrypt.CompareHashAndPassword([]byte(rest), []byte("482915")) == nil {
		t.Fatal("the PIN alone must not open the stored hash")
	}
	if string(DerivePinPepper(testKey)) == string(testKey) {
		t.Fatal("pepper must be derived")
	}
}

// Hashes made before the pepper existed still verify and ask to be upgraded.
func TestPinHasher_LegacyHashStillVerifies_Hardening1(t *testing.T) {
	legacy, err := bcrypt.GenerateFromPassword([]byte("482915"), pinCost)
	if err != nil {
		t.Fatal(err)
	}
	h := NewPinHasher(testKey)
	if !h.Verify(string(legacy), "482915") || h.Verify(string(legacy), "482916") || !h.NeedsRehash(string(legacy)) {
		t.Fatal("legacy verify or rehash flag")
	}
}

func TestPinGenerator_SG701(t *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		pin, err := PinGenerator{}.New()
		if err != nil || access.ValidateNewPin(pin) != nil {
			t.Fatalf("%q %v", pin, err)
		}
		seen[pin] = true
	}
	if len(seen) < 150 {
		t.Fatalf("PINs are not random enough: %d distinct of 200", len(seen))
	}
}
