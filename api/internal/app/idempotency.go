package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

// ErrIdempotencyKeyReused: the key was already used with a different request body (HTTP 409).
var ErrIdempotencyKeyReused = errors.New("idempotency key reused with a different request")

// ErrIdempotencyIncomplete: the key exists but no response was stored, so it can be neither replayed nor re-run.
var ErrIdempotencyIncomplete = errors.New("idempotency key has no stored response")

// IdempotencyOutcome is what Begin decides: run the effect (Replay false) or answer with the stored response.
type IdempotencyOutcome struct {
	Replay bool
	Status int
	Body   []byte
}

// IdempotencyStore keys are scoped by (tenant, route, key); the tenant comes from tx. Begin and Complete
// run in the same transaction as the effect, so a rolled-back effect leaves no key behind.
type IdempotencyStore interface {
	Begin(ctx context.Context, tx Tx, route, key, requestHash string) (IdempotencyOutcome, error)
	Complete(ctx context.Context, tx Tx, route, key string, status int, body []byte) error
}

// RequestHash is the SHA-256 (hex) of the canonical body: JSON is re-encoded (sorted keys, no
// whitespace, numbers kept as written) so equal documents hash equal; any other body hashes as raw bytes.
func RequestHash(body []byte) string {
	sum := sha256.Sum256(canonicalJSON(body))
	return hex.EncodeToString(sum[:])
}

func canonicalJSON(body []byte) []byte {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil || dec.More() {
		return body
	}
	out, err := json.Marshal(v)
	if err != nil {
		return body
	}
	return out
}
