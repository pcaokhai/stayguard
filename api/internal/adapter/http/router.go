// Package httpadapter is the HTTP driving adapter: routing, middleware, static web export.
package httpadapter

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func NewRouter(log *slog.Logger) http.Handler {
	r := chi.NewRouter()
	r.Use(requestLog(log))
	r.Get("/healthz", healthz)
	return r
}
