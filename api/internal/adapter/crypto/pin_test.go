package crypto

import (
	"strings"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

func TestPinHasher_SG701_AC4(t *testing.T) {
	h := PinHasher{}
	a, err := h.Hash("482915")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := h.Hash("482915")
	if strings.Contains(a, "482915") || !strings.HasPrefix(a, "$2") || a == b {
		t.Fatalf("want a salted bcrypt hash, got %q and %q", a, b)
	}
	if !h.Verify(a, "482915") || h.Verify(a, "482916") || h.Verify("not a hash", "482915") {
		t.Fatal("verify")
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
