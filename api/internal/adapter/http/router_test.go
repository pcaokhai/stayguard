package httpadapter

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func newTestRouter(buf *bytes.Buffer) http.Handler {
	return newTestRouterDir(buf, "")
}

func newTestRouterDir(buf *bytes.Buffer, dir string) http.Handler {
	return NewRouter(slog.New(slog.NewJSONHandler(buf, nil)), dir)
}

func TestHealthz_SG001_AC2(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestRouter(&bytes.Buffer{}).ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("code=%d ct=%q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["status"] != "ok" {
		t.Fatalf("body=%q err=%v", rec.Body.String(), err)
	}
}

func TestRequestLogFields_SG001_AC2(t *testing.T) {
	var buf bytes.Buffer
	req := httptest.NewRequest("GET", "/healthz?email=a@b.c", nil)
	req.Header.Set("X-Request-Id", "req-123")
	newTestRouter(&buf).ServeHTTP(httptest.NewRecorder(), req)

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("log not JSON: %q", buf.String())
	}
	for _, k := range []string{"trace_id", "tenant_id", "user_id", "route", "status", "duration_ms"} {
		if _, ok := line[k]; !ok {
			t.Errorf("missing field %s in %v", k, line)
		}
	}
	if line["trace_id"] != "req-123" || line["route"] != "/healthz" || line["status"] != float64(200) {
		t.Errorf("unexpected values: %v", line)
	}
	if bytes.Contains(buf.Bytes(), []byte("email")) {
		t.Errorf("query string leaked into log: %s", buf.String())
	}
}

func TestRequestLogInvalidTraceID_SG001_AC2(t *testing.T) {
	var buf bytes.Buffer
	req := httptest.NewRequest("GET", "/healthz", nil)
	req.Header.Set("X-Request-Id", "bad id\"{")
	newTestRouter(&buf).ServeHTTP(httptest.NewRecorder(), req)
	var line map[string]any
	_ = json.Unmarshal(buf.Bytes(), &line)
	if id, _ := line["trace_id"].(string); id == "" || id == "bad id\"{" {
		t.Errorf("trace_id not regenerated: %v", line["trace_id"])
	}
}

func TestPanicRecovered_SG001_AC2(t *testing.T) {
	var buf bytes.Buffer
	r := chi.NewRouter()
	useBaseMiddleware(r, slog.New(slog.NewJSONHandler(&buf, nil)))
	r.Get("/boom", func(http.ResponseWriter, *http.Request) { panic("boom") })
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/boom", nil))

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("no request log line after panic: %q", buf.String())
	}
	if rec.Code != 500 || line["status"] != float64(500) {
		t.Errorf("code=%d logged status=%v", rec.Code, line["status"])
	}
}
