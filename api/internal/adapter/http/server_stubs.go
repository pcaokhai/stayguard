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
type Server struct{}

func (Server) GetHealth(context.Context, gen.GetHealthRequestObject) (gen.GetHealthResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) GetReadiness(context.Context, gen.GetReadinessRequestObject) (gen.GetReadinessResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) ListBuildings(context.Context, gen.ListBuildingsRequestObject) (gen.ListBuildingsResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) ListRooms(context.Context, gen.ListRoomsRequestObject) (gen.ListRoomsResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) SimulatePaymentReceived(context.Context, gen.SimulatePaymentReceivedRequestObject) (gen.SimulatePaymentReceivedResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) CreateDemoSession(context.Context, gen.CreateDemoSessionRequestObject) (gen.CreateDemoSessionResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) ListHousekeepingTasks(context.Context, gen.ListHousekeepingTasksRequestObject) (gen.ListHousekeepingTasksResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) CompleteHousekeepingTask(context.Context, gen.CompleteHousekeepingTaskRequestObject) (gen.CompleteHousekeepingTaskResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) CreatePayment(context.Context, gen.CreatePaymentRequestObject) (gen.CreatePaymentResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) GetMe(context.Context, gen.GetMeRequestObject) (gen.GetMeResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) SetMyLocale(context.Context, gen.SetMyLocaleRequestObject) (gen.SetMyLocaleResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) GetOwnerOverview(context.Context, gen.GetOwnerOverviewRequestObject) (gen.GetOwnerOverviewResponseObject, error) {
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

func (Server) GetPayment(context.Context, gen.GetPaymentRequestObject) (gen.GetPaymentResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) StreamPaymentEvents(context.Context, gen.StreamPaymentEventsRequestObject) (gen.StreamPaymentEventsResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) GetRoom(context.Context, gen.GetRoomRequestObject) (gen.GetRoomResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) CreateStay(context.Context, gen.CreateStayRequestObject) (gen.CreateStayResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) ReportRoomUsage(context.Context, gen.ReportRoomUsageRequestObject) (gen.ReportRoomUsageResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) ListServices(context.Context, gen.ListServicesRequestObject) (gen.ListServicesResponseObject, error) {
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

func (Server) GetStay(context.Context, gen.GetStayRequestObject) (gen.GetStayResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) CheckoutStay(context.Context, gen.CheckoutStayRequestObject) (gen.CheckoutStayResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) AddStayExtras(context.Context, gen.AddStayExtrasRequestObject) (gen.AddStayExtrasResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) ReceiveBankWebhook(context.Context, gen.ReceiveBankWebhookRequestObject) (gen.ReceiveBankWebhookResponseObject, error) {
	return nil, errNotImplemented
}
