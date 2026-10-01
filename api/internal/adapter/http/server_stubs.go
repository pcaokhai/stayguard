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
}

// NewServer builds the operation handlers. demoEnabled mirrors DEMO_MODE, roomMap FF_S1_ROOM_MAP and
// checkIn FF_S2_CHECKIN and checkout FF_S3_CHECKOUT.
func NewServer(sessions sessionService, demoEnabled bool, rooms RoomService, roomMap bool, stays StayService, checkIn bool,
	billing BillingService, checkout bool, payments PaymentService, housekeeping HousekeepingService, owner OwnerService) Server {
	return Server{sessions: sessions, demoEnabled: demoEnabled, rooms: rooms, roomMap: roomMap, stays: stays, checkIn: checkIn,
		billing: billing, checkout: checkout, payments: payments, housekeeping: housekeeping, owner: owner}
}

func (Server) GetHealth(context.Context, gen.GetHealthRequestObject) (gen.GetHealthResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) GetReadiness(context.Context, gen.GetReadinessRequestObject) (gen.GetReadinessResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) GetShiftReview(context.Context, gen.GetShiftReviewRequestObject) (gen.GetShiftReviewResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) ListStaffPermissions(context.Context, gen.ListStaffPermissionsRequestObject) (gen.ListStaffPermissionsResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) SetBuildingPermission(context.Context, gen.SetBuildingPermissionRequestObject) (gen.SetBuildingPermissionResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) StreamPaymentEvents(context.Context, gen.StreamPaymentEventsRequestObject) (gen.StreamPaymentEventsResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) ReportRoomUsage(context.Context, gen.ReportRoomUsageRequestObject) (gen.ReportRoomUsageResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) GetCurrentShift(context.Context, gen.GetCurrentShiftRequestObject) (gen.GetCurrentShiftResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) CloseShift(context.Context, gen.CloseShiftRequestObject) (gen.CloseShiftResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) RecordCashPayout(context.Context, gen.RecordCashPayoutRequestObject) (gen.RecordCashPayoutResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) ReceiveBankWebhook(context.Context, gen.ReceiveBankWebhookRequestObject) (gen.ReceiveBankWebhookResponseObject, error) {
	return nil, errNotImplemented
}
