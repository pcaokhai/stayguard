//go:build integration

package postgres

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/app"
)

const (
	idemRoute    = "POST /v1/stays"
	effectPause  = 300 * time.Millisecond // keeps the first transaction open so the second call must wait
	replayStatus = 201
)

var replayBody = []byte(`{"id":"stay_1"}`)

// idemCall runs one request the way a use case does: Begin, effect once, Complete, all in one transaction.
func idemCall(t testing.TB, uow *UnitOfWork, store *IdempotencyStore, tenant, route, key string, body []byte, effect func()) (app.IdempotencyOutcome, error) {
	t.Helper()
	var out app.IdempotencyOutcome
	err := uow.Do(context.Background(), tenant, func(ctx context.Context, tx app.Tx) error {
		var err error
		if out, err = store.Begin(ctx, tx, route, key, app.RequestHash(body)); err != nil || out.Replay {
			return err
		}
		effect()
		return store.Complete(ctx, tx, route, key, replayStatus, replayBody)
	})
	return out, err
}

func TestIdempotency_SG003_AC5(t *testing.T) {
	db := migratedDB(t)
	pool := newAppPool(t, db, 4)
	uow, store := NewUnitOfWork(pool), NewIdempotencyStore(0)
	for _, id := range []string{"tnt_ac5_a", "tnt_ac5_b"} {
		seedTenant(t, db, id)
	}
	var effects atomic.Int32
	effect := func() { effects.Add(1) }

	t.Run("same key and body replays without a second effect", func(t *testing.T) {
		first, err := idemCall(t, uow, store, "tnt_ac5_a", idemRoute, "k-replay", []byte(`{"a":1,"b":2}`), effect)
		if err != nil || first.Replay {
			t.Fatalf("first call: replay=%v err=%v", first.Replay, err)
		}
		before := effects.Load()
		again, err := idemCall(t, uow, store, "tnt_ac5_a", idemRoute, "k-replay", []byte(`{ "b": 2, "a": 1 }`), effect)
		if err != nil || !again.Replay || again.Status != replayStatus || string(again.Body) != string(replayBody) {
			t.Fatalf("replay: %+v err=%v", again, err)
		}
		if effects.Load() != before {
			t.Fatal("effect ran again on replay")
		}
	})

	t.Run("same key different body is reuse", func(t *testing.T) {
		if _, err := idemCall(t, uow, store, "tnt_ac5_a", idemRoute, "k-reuse", []byte(`{"a":1}`), effect); err != nil {
			t.Fatal(err)
		}
		_, err := idemCall(t, uow, store, "tnt_ac5_a", idemRoute, "k-reuse", []byte(`{"a":2}`), effect)
		if !errors.Is(err, app.ErrIdempotencyKeyReused) {
			t.Fatalf("want ErrIdempotencyKeyReused, got %v", err)
		}
	})

	t.Run("keys are scoped by tenant and route", func(t *testing.T) {
		body := []byte(`{"a":1}`)
		if _, err := idemCall(t, uow, store, "tnt_ac5_a", idemRoute, "k-scope", body, effect); err != nil {
			t.Fatal(err)
		}
		for _, c := range []struct{ tenant, route string }{{"tnt_ac5_b", idemRoute}, {"tnt_ac5_a", "POST /v1/other"}} {
			out, err := idemCall(t, uow, store, c.tenant, c.route, "k-scope", body, effect)
			if err != nil || out.Replay {
				t.Fatalf("%+v: replay=%v err=%v (a different scope must proceed)", c, out.Replay, err)
			}
		}
	})

	t.Run("expired key is not replayed", func(t *testing.T) {
		body := []byte(`{"a":1}`)
		if _, err := idemCall(t, uow, store, "tnt_ac5_a", idemRoute, "k-expired", body, effect); err != nil {
			t.Fatal(err)
		}
		owner := connAs(t, db, "owner")
		if _, err := owner.Exec(context.Background(), `UPDATE app.idempotency_keys SET expires_at = now() - interval '1 second' WHERE key = 'k-expired'`); err != nil {
			t.Fatal(err)
		}
		before := effects.Load()
		out, err := idemCall(t, uow, store, "tnt_ac5_a", idemRoute, "k-expired", []byte(`{"a":2}`), effect)
		if err != nil || out.Replay || effects.Load() != before+1 {
			t.Fatalf("expired key must proceed with a new body: replay=%v err=%v", out.Replay, err)
		}
	})

	t.Run("two concurrent first calls produce one effect", func(t *testing.T) {
		before := effects.Load()
		slow := func() { effects.Add(1); time.Sleep(effectPause) }
		var wg sync.WaitGroup
		outs := make([]app.IdempotencyOutcome, 2)
		errs := make([]error, 2)
		for i := range outs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				outs[i], errs[i] = idemCall(t, uow, store, "tnt_ac5_a", idemRoute, "k-race", []byte(`{"a":1}`), slow)
			}()
		}
		wg.Wait()
		replays := 0
		for i := range outs {
			if errs[i] != nil {
				t.Fatalf("call %d: %v", i, errs[i])
			}
			if outs[i].Replay {
				replays++
			}
		}
		if effects.Load() != before+1 || replays != 1 {
			t.Fatalf("effects=%d replays=%d, want 1 and 1", effects.Load()-before, replays)
		}
	})
}
