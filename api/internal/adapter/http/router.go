// Package httpadapter is the HTTP driving adapter: routing, middleware, static web export.
package httpadapter

import (
	"log/slog"
	"net/http"
	"strings"

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
	r.Use(noStoreGuestID)
}

// maxRequestBodyBytes caps every request body: the largest legitimate body (check-in) is well under
// 1 KiB, and without a cap an unauthenticated client could make the strict decoder buffer unbounded input.
const maxRequestBodyBytes = 64 << 10

// limitBody answers 413 at once when the declared length is over the cap and bounds the reader
// otherwise (chunked bodies), so oversize input never reaches a use case.
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := int64(maxRequestBodyBytes)
		if isPhotoUpload(r) {
			limit = photoRouteMaxBody // only the guest ID photo upload may carry an image
		}
		if r.ContentLength > limit {
			writeProblem(w, http.StatusRequestEntityTooLarge, "Payload Too Large", codeForLimit(limit))
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
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
	// Auth serves signIn, signOut and changeMyPin; TrustProxy reads the client address from X-Forwarded-For (behind Caddy).
	Auth AuthService
	// Staff serves the staff and building access operations.
	Staff StaffService
	// Bank serves the property, receiving account and SePay status operations.
	Bank BankService
	// Setup serves buildings, rooms, rate plans and items.
	Setup SetupService
	// GuestIDs serves the guest ID number and photos.
	GuestIDs GuestIDService
	// Webhook receives SePay events at /v1/webhooks/bank/{hookId}.
	Webhook    WebhookService
	TrustProxy bool
	ProxyHops  int
	// StayOps serves stay history, timeline, receipt and the check-in time and move corrections (no flag).
	StayOps StayOpsService
	// Shifts serves the shift and cash operations (no flag).
	Shifts ShiftService
	// Monitor serves alerts, transactions, linking and the activity log (no flag).
	Monitor MonitorService
	// Maintenance serves damage reports, tickets and unused-room reports (no flag).
	Maintenance MaintenanceService
	// Roster serves the duty roster and leave requests (no flag).
	Roster RosterService
	// Finance serves payroll, expenses and the income and cost report (no flag).
	Finance FinanceService
}

func NewRouter(log *slog.Logger, o Options) http.Handler {
	r := chi.NewRouter()
	useBaseMiddleware(r, log)
	r.Use(clientIP(o.TrustProxy, o.ProxyHops))
	r.Use(withMeta(o.TrustProxy))
	r.Use(authenticate(log, o.Sessions))
	strict := gen.NewStrictHandlerWithOptions(NewServer(o.Sessions, o.DemoEnabled, o.Rooms, o.RoomMapEnabled, o.Stays, o.CheckInEnabled, o.Billing, o.CheckoutEnabled, o.Payments, o.Housekeeping, o.Owner, o.Auth, o.Staff).WithStayOps(o.StayOps).WithBank(o.Bank).WithSetup(o.Setup).WithGuestIDs(o.GuestIDs).WithShifts(o.Shifts).WithMonitor(o.Monitor).WithMaintenance(o.Maintenance).WithRoster(o.Roster).WithFinance(o.Finance), nil, gen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  badRequestResponse,
		ResponseErrorHandlerFunc: problemResponder(log),
	})
	gen.HandlerWithOptions(strict, gen.ChiServerOptions{BaseRouter: r, ErrorHandlerFunc: badRequestResponse})
	if o.Webhook != nil { // registered after the generated routes so this raw-body handler serves the path
		r.Post(webhookPrefix+"{hookId}", receiveWebhook(log, o.Webhook))
	}
	r.Get("/healthz", healthz) // keeps the SG-001 body; replaced by Server.GetHealth when ops are implemented
	r.Get("/readyz", readyz(log, o.Probe))
	r.NotFound(staticHandler(o.StaticDir).ServeHTTP)
	return r
}

// isPhotoUpload is PUT /v1/stays/{id}/guest-id/photos/{side}.
func isPhotoUpload(r *http.Request) bool {
	if r.Method != http.MethodPut || !strings.HasPrefix(r.URL.Path, "/v1/stays/") {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/stays/"), "/")
	return len(parts) == 4 && parts[1] == "guest-id" && parts[2] == "photos"
}

func codeForLimit(limit int64) string {
	if limit == photoRouteMaxBody {
		return "PHOTO_TOO_LARGE"
	}
	return "PAYLOAD_TOO_LARGE"
}

// noStoreGuestID marks every guest ID response, errors included, as never cacheable (docs/15 rule 24).
func noStoreGuestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/guest-id") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}
