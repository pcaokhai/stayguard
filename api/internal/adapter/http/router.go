// Package httpadapter is the HTTP driving adapter: routing, middleware, static web export.
package httpadapter

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/pcaokhai/stayguard/api/internal/adapter/http/gen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// useBaseMiddleware wires requestLog outside Recoverer so a panicking handler is
// turned into a 500 first and still produces its request log line.
func useBaseMiddleware(r chi.Router, log *slog.Logger) {
	r.Use(requestLog(log))
	r.Use(middleware.Recoverer)
}

// SessionService is the application surface the router needs: authentication for the middleware
// and the session use cases for the handlers.
type SessionService interface {
	authenticator
	sessionService
}

// Options are the router's collaborators; DemoEnabled mirrors DEMO_MODE.
type Options struct {
	StaticDir   string
	Probe       app.ReadinessProbe
	Sessions    SessionService
	DemoEnabled bool
}

func NewRouter(log *slog.Logger, o Options) http.Handler {
	r := chi.NewRouter()
	useBaseMiddleware(r, log)
	r.Use(authenticate(log, o.Sessions))
	strict := gen.NewStrictHandlerWithOptions(NewServer(o.Sessions, o.DemoEnabled), nil, gen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  badRequestResponse,
		ResponseErrorHandlerFunc: problemResponder(log),
	})
	gen.HandlerWithOptions(strict, gen.ChiServerOptions{BaseRouter: r, ErrorHandlerFunc: badRequestResponse})
	r.Get("/healthz", healthz) // keeps the SG-001 body; replaced by Server.GetHealth when ops are implemented
	r.Get("/readyz", readyz(log, o.Probe))
	r.NotFound(staticHandler(o.StaticDir).ServeHTTP)
	return r
}
