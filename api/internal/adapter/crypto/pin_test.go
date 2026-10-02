package crypto

import (
	"strings"
	"testing"
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
