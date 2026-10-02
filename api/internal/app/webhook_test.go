package app

import (
	"bytes"
	"context"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The vector comes from an independent HMAC-SHA256 of "{timestamp}.{raw body}", as SePay's documentation specifies.
func TestSepaySignature_Vector_SG703(t *testing.T) {
	got := SepaySignature([]byte("s3cret"), "1700000000", []byte(`{"id":1}`))
	if got != "sha256=ee0658aa4e37018df69c24227df01e0f680eb3b87c7f1f9bd936e283cfe01d9b" {
		t.Fatalf("got %s", got)
	}
}

// The tolerance is configurable; a delivery rejected for age is counted and logged without its payload.
func TestWebhookTolerance_CounterAndNoPayload_SG703(t *testing.T) {
	var logs bytes.Buffer
	w := &Webhook{clock: &clockBox{now: t0}, tolerance: defaultWebhookTolerance, log: slog.New(slog.NewJSONHandler(&logs, nil))}
	secret, body := []byte("s3cret"), []byte(`{"id":1,"content":"PAYER-NAME-MARKER"}`)
	at := func(d time.Duration) string { return strconv.FormatInt(t0.Add(d).Unix(), 10) }
	check := func(d time.Duration) bool {
		ts := at(d)
		return w.signatureValid(context.Background(), "tn_a", secret, SepaySignature(secret, ts, body), ts, body)
	}
	if !check(-299*time.Second) || !check(299*time.Second) {
		t.Fatal("inside the default window")
	}
	if check(-6*time.Minute) || check(6*time.Minute) {
		t.Fatal("outside the default window")
	}
	if w.tooOld.Load() != 2 || !strings.Contains(logs.String(), `"rejected_for_age_total":2`) || strings.Contains(logs.String(), "PAYER-NAME-MARKER") || strings.Contains(logs.String(), "s3cret") {
		t.Fatalf("counter %d logs %s", w.tooOld.Load(), logs.String())
	}
	w.WithTolerance(time.Hour, nil)
	if !check(-30 * time.Minute) {
		t.Fatal("a widened window accepts it")
	}
	if check(-2 * time.Hour) {
		t.Fatal("but not beyond it")
	}
}
