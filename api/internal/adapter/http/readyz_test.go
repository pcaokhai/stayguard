package httpadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/app"
)

type fakeProbe struct{ err error }

func (f fakeProbe) Check(context.Context) error { return f.err }

func TestReadyz_SG003_AC6(t *testing.T) {
	const driverText = "dial tcp 10.9.8.7:5432: password authentication failed for user hunter2"
	tests := []struct {
		name     string
		err      error
		wantCode int
		wantProb string
	}{
		{"current", nil, http.StatusOK, ""},
		{"database unreachable", errors.New(driverText), http.StatusServiceUnavailable, "NOT_READY"},
		{"migrations pending", errors.New("2 pending: " + driverText), http.StatusServiceUnavailable, "NOT_READY"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			r := NewRouter(slog.New(slog.NewJSONHandler(&logs, nil)), "", app.ReadinessProbe(fakeProbe{tc.err}))
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest("GET", "/readyz", nil))

			if rec.Code != tc.wantCode {
				t.Fatalf("code=%d want %d", rec.Code, tc.wantCode)
			}
			if strings.Contains(rec.Body.String(), "hunter2") || strings.Contains(rec.Body.String(), "10.9.8.7") {
				t.Fatalf("body leaks the cause: %q", rec.Body.String())
			}
			if tc.err == nil {
				return
			}
			var p problemBody
			if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || p.Code != tc.wantProb || p.Status != 503 {
				t.Fatalf("body=%q err=%v", rec.Body.String(), err)
			}
			if rec.Header().Get("Content-Type") != problemContentType {
				t.Errorf("content type %q", rec.Header().Get("Content-Type"))
			}
			if !strings.Contains(logs.String(), `"level":"ERROR"`) || !strings.Contains(logs.String(), "hunter2") {
				t.Errorf("cause not logged at error level: %s", logs.String())
			}
		})
	}
}
