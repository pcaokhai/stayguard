package httpadapter

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const (
	traceHeader   = "X-Request-Id"
	traceIDBytes  = 16
	unmatchedPath = "unmatched"
)

// validTraceID keeps client-supplied ids from injecting odd characters into logs.
var validTraceID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func traceID(r *http.Request) string {
	if id := r.Header.Get(traceHeader); validTraceID.MatchString(id) {
		return id
	}
	b := make([]byte, traceIDBytes)
	_, _ = rand.Read(b) // never fails on supported platforms
	return hex.EncodeToString(b)
}

// requestLog emits one line per request. It logs the chi route pattern, never the raw
// URL, so paths and query strings cannot carry personal data into logs.
func requestLog(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			id := traceID(r)
			w.Header().Set(traceHeader, id)
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = unmatchedPath
			}
			log.InfoContext(r.Context(), "http request",
				"trace_id", id,
				"tenant_id", "", // SG-102 supplies tenant and user from the session
				"user_id", "",
				"route", route,
				"status", ww.Status(),
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}
