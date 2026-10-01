package httpadapter

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/adapter/http/gen"
)

// Compile-time proof (SG-002 AC2): Server must implement every operation of the contract.
// Adding an operation to openapi.yaml, or deleting a stub without a real handler, breaks the build.
var _ gen.StrictServerInterface = (*Server)(nil)

func TestUnimplementedOperation_Returns501Problem_SG002_AC2(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestRouter(&bytes.Buffer{}).ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services", nil))

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
