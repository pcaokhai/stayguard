package httpadapter

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/adapter/http/gen"
)

// Compile-time proof (SG-002 AC2): Server must implement every operation of the contract.
// Adding an operation to openapi.yaml, or deleting a stub without a real handler, breaks the build.
var _ gen.StrictServerInterface = (*Server)(nil)

func TestUnimplementedOperation_Returns501Problem_SG002_AC2(t *testing.T) {
	rec := do(newTestRouter(&bytes.Buffer{}), "GET", "/v1/services", "Bearer "+goodToken)

	if rec.Code != 501 || rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("code=%d ct=%q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var p struct {
		Status int    `json:"status"`
		Code   string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || p.Status != 501 || p.Code != "NOT_IMPLEMENTED" {
		t.Fatalf("body=%q err=%v", rec.Body.String(), err)
	}
}

func TestMalformedBody_Returns400Problem_SG002_AC2(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/rooms/r1/stays", strings.NewReader("{not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+goodToken)
	newTestRouter(&bytes.Buffer{}).ServeHTTP(rec, req)

	if rec.Code != 400 || rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("code=%d ct=%q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), `"code":"BAD_REQUEST"`) {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestUnknownError_LoggedNotLeaked_SG002_AC2(t *testing.T) {
	var logs bytes.Buffer
	rec := httptest.NewRecorder()
	problemResponder(slog.New(slog.NewJSONHandler(&logs, nil)))(rec, httptest.NewRequest("GET", "/", nil), errors.New("secret-detail"))

	if rec.Code != 500 || strings.Contains(rec.Body.String(), "secret-detail") {
		t.Fatalf("code=%d body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(logs.String(), "secret-detail") {
		t.Fatalf("error not logged: %q", logs.String())
	}
}
