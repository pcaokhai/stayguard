package httpadapter

import (
	"context"
	"errors"

	"github.com/pcaokhai/stayguard/api/internal/adapter/http/gen"
)

// errNotImplemented marks an operation whose handler has not been written yet. It is mapped
// to a 501 problem response in one place (problemResponse).
var errNotImplemented = errors.New("operation not implemented")

// Server implements every operation of contracts/openapi.yaml. Each story replaces the stubs
// of its own operations with real handlers in its own file and deletes those stubs from here;
// an operation with neither a stub nor a handler does not compile (SG-002 AC2).
type Server struct {
	sessions     sessionService
	demoEnabled  bool
	rooms        RoomService
	roomMap      bool
	stays        StayService
	checkIn      bool
	billing      BillingService
	checkout     bool
	payments     PaymentService
	housekeeping HousekeepingService
	owner        OwnerService
	auth         AuthService
	stayOps      StayOpsService
	shifts       ShiftService
	monitor      MonitorService
	maintenance  MaintenanceService
	roster       RosterService
	finance      FinanceService
	staff        StaffService
	bank         BankService
	setup        SetupService
	guestIDs     GuestIDService
}

// NewServer builds the operation handlers. demoEnabled mirrors DEMO_MODE, roomMap FF_S1_ROOM_MAP and
// checkIn FF_S2_CHECKIN and checkout FF_S3_CHECKOUT.
func NewServer(sessions sessionService, demoEnabled bool, rooms RoomService, roomMap bool, stays StayService, checkIn bool,
	billing BillingService, checkout bool, payments PaymentService, housekeeping HousekeepingService, owner OwnerService, auth AuthService, staff StaffService) Server {
	return Server{sessions: sessions, demoEnabled: demoEnabled, rooms: rooms, roomMap: roomMap, stays: stays, checkIn: checkIn,
		billing: billing, checkout: checkout, payments: payments, housekeeping: housekeeping, owner: owner, auth: auth, staff: staff}
}

// WithStayOps adds the stay history and correction use cases (editCheckInTime, moveStay, listStays,
// getStayTimeline, getReceipt); a Server without them answers those operations with a nil dereference, so the router always sets them.
func (s Server) WithStayOps(ops StayOpsService) Server { s.stayOps = ops; return s }

func (Server) GetHealth(context.Context, gen.GetHealthRequestObject) (gen.GetHealthResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) GetReadiness(context.Context, gen.GetReadinessRequestObject) (gen.GetReadinessResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) StreamPaymentEvents(context.Context, gen.StreamPaymentEventsRequestObject) (gen.StreamPaymentEventsResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) ReceiveBankWebhook(context.Context, gen.ReceiveBankWebhookRequestObject) (gen.ReceiveBankWebhookResponseObject, error) {
	return nil, errNotImplemented
}
