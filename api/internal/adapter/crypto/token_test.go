package crypto

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestTokenIs32RandomBytes_SG102(t *testing.T) {
	g := TokenGenerator{}
	a, err := g.New()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := g.New()
	raw, err := base64.RawURLEncoding.DecodeString(a)
	if err != nil || len(raw) != 32 {
		t.Fatalf("token must decode to 32 bytes, got %d err=%v", len(raw), err)
	}
	if a == b {
		t.Fatal("two tokens must differ")
	}
}

func TestTokenFailsWhenRandFails_SG102(t *testing.T) {
	want := errors.New("boom")
	_, err := TokenGenerator{Reader: failReader{want}}.New()
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want wrapped %v", err, want)
	}
	_, err = TokenGenerator{Reader: strings.NewReader("short")}.New()
	if err == nil {
		t.Fatal("short read must fail")
	}
}

type failReader struct{ err error }

func (f failReader) Read([]byte) (int, error) { return 0, f.err }
