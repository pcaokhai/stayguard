// Package httpadapter is the HTTP driving adapter: routing, middleware, static web export.
package httpadapter

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// useBaseMiddleware wires requestLog outside Recoverer so a panicking handler is
// turned into a 500 first and still produces its request log line.
func useBaseMiddleware(r chi.Router, log *slog.Logger) {
	r.Use(requestLog(log))
	r.Use(middleware.Recoverer)
}

func NewRouter(log *slog.Logger, staticDir string) http.Handler {
	r := chi.NewRouter()
	useBaseMiddleware(r, log)
	r.Get("/healthz", healthz)
	r.NotFound(staticHandler(staticDir).ServeHTTP)
	return r
}
