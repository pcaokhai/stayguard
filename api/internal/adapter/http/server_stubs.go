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
	staff        StaffService
	bank         BankService
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

func (Server) ReportRoomUsage(context.Context, gen.ReportRoomUsageRequestObject) (gen.ReportRoomUsageResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) ReceiveBankWebhook(context.Context, gen.ReceiveBankWebhookRequestObject) (gen.ReceiveBankWebhookResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) ApproveLeave(context.Context, gen.ApproveLeaveRequestObject) (gen.ApproveLeaveResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) CancelMyLeave(context.Context, gen.CancelMyLeaveRequestObject) (gen.CancelMyLeaveResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) CopyRosterWeek(context.Context, gen.CopyRosterWeekRequestObject) (gen.CopyRosterWeekResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) CreateBuilding(context.Context, gen.CreateBuildingRequestObject) (gen.CreateBuildingResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) CreateExpense(context.Context, gen.CreateExpenseRequestObject) (gen.CreateExpenseResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) CreateFloor(context.Context, gen.CreateFloorRequestObject) (gen.CreateFloorResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) CreateLeaveRequest(context.Context, gen.CreateLeaveRequestRequestObject) (gen.CreateLeaveRequestResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) CreateRooms(context.Context, gen.CreateRoomsRequestObject) (gen.CreateRoomsResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) CreateService(context.Context, gen.CreateServiceRequestObject) (gen.CreateServiceResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) CreateStocktake(context.Context, gen.CreateStocktakeRequestObject) (gen.CreateStocktakeResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) DeclineLeave(context.Context, gen.DeclineLeaveRequestObject) (gen.DeclineLeaveResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) DeleteExpense(context.Context, gen.DeleteExpenseRequestObject) (gen.DeleteExpenseResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) DeleteGuestIdNumber(context.Context, gen.DeleteGuestIdNumberRequestObject) (gen.DeleteGuestIdNumberResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) DeleteGuestIdPhoto(context.Context, gen.DeleteGuestIdPhotoRequestObject) (gen.DeleteGuestIdPhotoResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) GetExpenseMonth(context.Context, gen.GetExpenseMonthRequestObject) (gen.GetExpenseMonthResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) GetGuestIdPhoto(context.Context, gen.GetGuestIdPhotoRequestObject) (gen.GetGuestIdPhotoResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) GetGuestIdRecord(context.Context, gen.GetGuestIdRecordRequestObject) (gen.GetGuestIdRecordResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) GetIncomeCostReport(context.Context, gen.GetIncomeCostReportRequestObject) (gen.GetIncomeCostReportResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) GetMyRoster(context.Context, gen.GetMyRosterRequestObject) (gen.GetMyRosterResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) GetPayroll(context.Context, gen.GetPayrollRequestObject) (gen.GetPayrollResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) GetRoster(context.Context, gen.GetRosterRequestObject) (gen.GetRosterResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) GetTicket(context.Context, gen.GetTicketRequestObject) (gen.GetTicketResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) ListLeaveRequests(context.Context, gen.ListLeaveRequestsRequestObject) (gen.ListLeaveRequestsResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) ListMyLeaveRequests(context.Context, gen.ListMyLeaveRequestsRequestObject) (gen.ListMyLeaveRequestsResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) ListRatePlans(context.Context, gen.ListRatePlansRequestObject) (gen.ListRatePlansResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) ListStockMovements(context.Context, gen.ListStockMovementsRequestObject) (gen.ListStockMovementsResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) ListTickets(context.Context, gen.ListTicketsRequestObject) (gen.ListTicketsResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) MarkPayrollPaid(context.Context, gen.MarkPayrollPaidRequestObject) (gen.MarkPayrollPaidResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) PreviewPrice(context.Context, gen.PreviewPriceRequestObject) (gen.PreviewPriceResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) PutRoster(context.Context, gen.PutRosterRequestObject) (gen.PutRosterResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) RemoveService(context.Context, gen.RemoveServiceRequestObject) (gen.RemoveServiceResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) ReportDamage(context.Context, gen.ReportDamageRequestObject) (gen.ReportDamageResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) RestockService(context.Context, gen.RestockServiceRequestObject) (gen.RestockServiceResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) RevealGuestIdNumber(context.Context, gen.RevealGuestIdNumberRequestObject) (gen.RevealGuestIdNumberResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) SetGuestIdNumber(context.Context, gen.SetGuestIdNumberRequestObject) (gen.SetGuestIdNumberResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) UpdateBuilding(context.Context, gen.UpdateBuildingRequestObject) (gen.UpdateBuildingResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) UpdateExpense(context.Context, gen.UpdateExpenseRequestObject) (gen.UpdateExpenseResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) UpdatePayrollLine(context.Context, gen.UpdatePayrollLineRequestObject) (gen.UpdatePayrollLineResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) UpdateRatePlan(context.Context, gen.UpdateRatePlanRequestObject) (gen.UpdateRatePlanResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) UpdateRoom(context.Context, gen.UpdateRoomRequestObject) (gen.UpdateRoomResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) UpdateService(context.Context, gen.UpdateServiceRequestObject) (gen.UpdateServiceResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) UpdateTicket(context.Context, gen.UpdateTicketRequestObject) (gen.UpdateTicketResponseObject, error) {
	return nil, errNotImplemented
}

func (Server) UploadGuestIdPhoto(context.Context, gen.UploadGuestIdPhotoRequestObject) (gen.UploadGuestIdPhotoResponseObject, error) {
	return nil, errNotImplemented
}
