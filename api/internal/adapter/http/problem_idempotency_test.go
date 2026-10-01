package httpadapter

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/app"
)

func TestIdempotencyKeyReused_Maps409Problem_SG003_AC5(t *testing.T) {
	rec := httptest.NewRecorder()
	err := fmt.Errorf("create stay: %w", app.ErrIdempotencyKeyReused)
	problemResponder(slog.New(slog.NewTextHandler(io.Discard, nil)))(rec, httptest.NewRequest("POST", "/", nil), err)

	if rec.Code != 409 || rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("code=%d content-type=%q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), `"code":"IDEMPOTENCY_KEY_REUSED"`) {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestIdempotencyIncomplete_Maps500AndLogs_SG003_AC5(t *testing.T) {
	var logs bytes.Buffer
	rec := httptest.NewRecorder()
	err := fmt.Errorf("create stay: %w", app.ErrIdempotencyIncomplete)
	problemResponder(slog.New(slog.NewTextHandler(&logs, nil)))(rec, httptest.NewRequest("POST", "/", nil), err)

	if rec.Code != 500 || !strings.Contains(rec.Body.String(), `"code":"INTERNAL"`) || strings.Contains(rec.Body.String(), "idempotency") {
		t.Fatalf("code=%d body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(logs.String(), "level=ERROR") || !strings.Contains(logs.String(), "idempotency key committed without a stored response") {
		t.Fatalf("not logged as a programming error: %q", logs.String())
	}
}
