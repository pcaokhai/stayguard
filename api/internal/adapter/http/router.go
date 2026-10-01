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
	r.Use(limitBody)
}

// maxRequestBodyBytes caps every request body: the largest legitimate body (check-in) is well under
// 1 KiB, and without a cap an unauthenticated client could make the strict decoder buffer unbounded input.
const maxRequestBodyBytes = 64 << 10

// limitBody answers 413 at once when the declared length is over the cap and bounds the reader
// otherwise (chunked bodies), so oversize input never reaches a use case.
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > maxRequestBodyBytes {
			writeProblem(w, http.StatusRequestEntityTooLarge, "Payload Too Large", "PAYLOAD_TOO_LARGE")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		next.ServeHTTP(w, r)
	})
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
	// Rooms and RoomMapEnabled (FF_S1_ROOM_MAP) serve the room map read operations.
	Rooms          RoomService
	RoomMapEnabled bool
	// Stays and CheckInEnabled (FF_S2_CHECKIN) serve createStay and getStay.
	Stays          StayService
	CheckInEnabled bool
	// Billing and CheckoutEnabled (FF_S3_CHECKOUT) serve listServices, addStayExtras and checkoutStay.
	Billing         BillingService
	CheckoutEnabled bool
	// Payments serves createPayment, getPayment and simulatePaymentReceived (no flag; the simulator needs DemoEnabled).
	Payments PaymentService
	// Housekeeping serves listHousekeepingTasks and completeHousekeepingTask (no flag).
	Housekeeping HousekeepingService
	// Owner serves getOwnerOverview (no flag).
	Owner OwnerService
}

func NewRouter(log *slog.Logger, o Options) http.Handler {
	r := chi.NewRouter()
	useBaseMiddleware(r, log)
	r.Use(authenticate(log, o.Sessions))
	strict := gen.NewStrictHandlerWithOptions(NewServer(o.Sessions, o.DemoEnabled, o.Rooms, o.RoomMapEnabled, o.Stays, o.CheckInEnabled, o.Billing, o.CheckoutEnabled, o.Payments, o.Housekeeping, o.Owner), nil, gen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  badRequestResponse,
		ResponseErrorHandlerFunc: problemResponder(log),
	})
	gen.HandlerWithOptions(strict, gen.ChiServerOptions{BaseRouter: r, ErrorHandlerFunc: badRequestResponse})
	r.Get("/healthz", healthz) // keeps the SG-001 body; replaced by Server.GetHealth when ops are implemented
	r.Get("/readyz", readyz(log, o.Probe))
	r.NotFound(staticHandler(o.StaticDir).ServeHTTP)
	return r
}
