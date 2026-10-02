//go:build integration

package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/adapter/crypto"
	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// The follow-up bug: sign-in answered PIN_INVALID for a kept database after the data key changed. A restart with the same
// key keeps working; a different key makes every earlier PIN hash fail (the cause), and the server now refuses to start.
func TestKeyChange_Restart_FailsClosed_FollowUp(t *testing.T) {
	e := newEnv(t)
	e.seedOwner() // the owner's PIN hash is made with testDataKey
	ctx := context.Background()
	keyA := testDataKey
	keyB := bytes.Repeat([]byte{0x77}, 32)

	// Restart with the same database and the same key: the check passes any number of times and sign-in still works.
	for i := 0; i < 2; i++ {
		if err := postgres.CheckKeyFingerprint(ctx, e.pool, crypto.KeyFingerprint(keyA)); err != nil {
			t.Fatalf("restart %d with the same key: %v", i, err)
		}
	}
	if r := e.signIn("owner1", ownerPIN); r.status != 200 {
		t.Fatalf("sign-in after a restart: %d %v", r.status, r.body)
	}

	// The root cause, shown: with another key the PIN hash made earlier does not verify, so the right PIN looks wrong.
	hash, err := crypto.NewPinHasher(keyA).Hash(ownerPIN)
	if err != nil {
		t.Fatal(err)
	}
	if crypto.NewPinHasher(keyB).Verify(hash, ownerPIN) {
		t.Fatal("a hash must not verify under another key")
	}

	// A different key is refused at start with a clear, non-secret error.
	err = postgres.CheckKeyFingerprint(ctx, e.pool, crypto.KeyFingerprint(keyB))
	if !errors.Is(err, postgres.ErrKeyMismatch) || !strings.Contains(err.Error(), "key fingerprint mismatch") {
		t.Fatalf("different key: %v", err)
	}
	for _, secret := range []string{string(keyA), string(keyB), crypto.KeyFingerprint(keyA), crypto.KeyFingerprint(keyB)} {
		if strings.Contains(err.Error(), secret) {
			t.Fatal("the error leaks key material")
		}
	}
	// The stored row cannot be changed or removed by the application role, so a wrong key cannot overwrite the right one.
	for _, q := range []string{`UPDATE public.key_fingerprint SET fingerprint = 'x'`, `DELETE FROM public.key_fingerprint`} {
		if _, err := e.pool.Exec(ctx, q); err == nil {
			t.Errorf("the app role ran %q", q)
		}
	}
	if err := postgres.CheckKeyFingerprint(ctx, e.pool, crypto.KeyFingerprint(keyA)); err != nil {
		t.Fatalf("the original key still passes: %v", err)
	}
	_ = app.ErrPinInvalid
}
