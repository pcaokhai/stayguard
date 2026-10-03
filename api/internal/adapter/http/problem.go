package httpadapter

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/pricing"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

const (
	problemContentType = "application/problem+json"
	codeInvalid        = "INVALID"
)

// problemBody mirrors the Problem schema of the contract (required fields only).
type problemBody struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Code   string `json:"code"`
	// Errors lists field failures of a 422; omitted for every other problem.
	Errors []fieldProblem `json:"errors,omitempty"`
	// LockedUntil is set only on ACCOUNT_LOCKED.
	LockedUntil *time.Time `json:"lockedUntil,omitempty"`
}

// fieldProblem is one entry of the contract's errors[]: a field path and a code, never the value.
type fieldProblem struct {
	Code  string `json:"code"`
	Field string `json:"field"`
}

func writeProblem(w http.ResponseWriter, status int, title, code string) {
	writeProblemWith(w, status, title, code, nil)
}

func writeProblemWith(w http.ResponseWriter, status int, title, code string, errs []fieldProblem) {
	if status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	w.Header().Set("Content-Type", problemContentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problemBody{Type: "about:blank", Title: title, Status: status, Code: code, Errors: errs})
}

// badRequestResponse maps request decoding and parameter binding errors to 400. The error text
// is not echoed: it can quote client input.
func badRequestResponse(w http.ResponseWriter, _ *http.Request, err error) {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		writeProblem(w, http.StatusRequestEntityTooLarge, "Payload Too Large", "PAYLOAD_TOO_LARGE")
		return
	}
	writeProblem(w, http.StatusBadRequest, "Bad Request", "BAD_REQUEST")
}

// problemResponder is the single place where handler errors become HTTP problem responses
// (CLAUDE.md §6 rule 10). Unknown errors are logged, never echoed to the client.
// Errors from the authenticator reach this function too: they must never wrap or quote the bearer
// token (the middleware logs unknown errors).
func problemResponder(log *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		if errors.Is(err, errNotImplemented) {
			writeProblem(w, http.StatusNotImplemented, "Not Implemented", "NOT_IMPLEMENTED")
			return
		}
		if errors.Is(err, app.ErrIdempotencyKeyReused) {
			writeProblem(w, http.StatusConflict, "Conflict", "IDEMPOTENCY_KEY_REUSED")
			return
		}
		if errors.Is(err, app.ErrIdempotencyIncomplete) {
			log.ErrorContext(r.Context(), "idempotency key committed without a stored response: a use case skipped Complete")
			writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "INTERNAL")
			return
		}
		if mapSessionError(w, err) || mapRoomError(w, err) || mapStayError(w, err) || mapPaymentError(w, err) || mapHousekeepingError(w, err) {
			return
		}
		log.ErrorContext(r.Context(), "unhandled handler error", "error", err)
		writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "INTERNAL")
	}
}

// mapSessionError writes the problem for identity errors and reports whether it matched. Titles are
// generic and nothing from the request is echoed.
func mapSessionError(w http.ResponseWriter, err error) bool {
	var ve *app.ValidationError
	var locked *app.AccountLockedError
	switch {
	case errors.As(err, &locked):
		w.Header().Set("Content-Type", problemContentType)
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(problemBody{Type: "about:blank", Title: "Unauthorized", Status: http.StatusUnauthorized,
			Code: "ACCOUNT_LOCKED", LockedUntil: &locked.Until})
	case errors.Is(err, app.ErrPinInvalid):
		writeProblem(w, http.StatusUnauthorized, "Unauthorized", "PIN_INVALID")
	case errors.Is(err, app.ErrTooManyRequests):
		w.Header().Set("Retry-After", "60")
		writeProblem(w, http.StatusTooManyRequests, "Too Many Requests", "RATE_LIMITED")
	case errors.Is(err, app.ErrPinChangeRequired):
		writeProblem(w, http.StatusForbidden, "Forbidden", "PIN_CHANGE_REQUIRED")
	case errors.Is(err, app.ErrOwnerPinInvalid):
		writeProblem(w, http.StatusForbidden, "Forbidden", "OWNER_PIN_INVALID")
	case errors.Is(err, app.ErrShiftOpen):
		writeProblem(w, http.StatusConflict, "Conflict", "SHIFT_OPEN")
	case errors.Is(err, app.ErrPinTooSimple):
		writeProblem(w, http.StatusUnprocessableEntity, "Unprocessable Entity", "PIN_TOO_SIMPLE")
	case errors.Is(err, app.ErrDemoDisabled):
		writeProblem(w, http.StatusNotFound, "Not Found", "DEMO_DISABLED")
	case errors.Is(err, app.ErrTrialNotFound):
		writeProblem(w, http.StatusNotFound, "Not Found", "TRIAL_NOT_FOUND")
	case errors.Is(err, app.ErrUnauthenticated):
		writeProblem(w, http.StatusUnauthorized, "Unauthorized", "UNAUTHENTICATED")
	case errors.Is(err, app.ErrSessionExpired):
		writeProblem(w, http.StatusUnauthorized, "Unauthorized", "SESSION_EXPIRED")
	case errors.As(err, &ve):
		writeProblem(w, http.StatusUnprocessableEntity, "Unprocessable Entity", "VALIDATION_FAILED")
	case errors.Is(err, app.ErrConflict):
		writeProblem(w, http.StatusConflict, "Conflict", "CONFLICT")
	default:
		return false
	}
	return true
}

// mapRoomError writes the problem for room map errors and reports whether it matched. A foreign
// tenant's id and an unknown id are both app.ErrNotFound, so their bodies are identical.
func mapRoomError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, errInvalidParam):
		writeProblem(w, http.StatusBadRequest, "Bad Request", "BAD_REQUEST")
	case errors.Is(err, access.ErrRoleForbidden):
		writeProblem(w, http.StatusForbidden, "Forbidden", "ROLE_FORBIDDEN")
	case errors.Is(err, access.ErrBuildingForbidden):
		writeProblem(w, http.StatusForbidden, "Forbidden", "BUILDING_FORBIDDEN")
	case errors.Is(err, app.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "Not Found", "NOT_FOUND")
	case errors.Is(err, app.ErrFeatureDisabled):
		writeProblem(w, http.StatusNotFound, "Not Found", "FEATURE_DISABLED")
	case errors.Is(err, app.ErrPricingUnavailable):
		writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable", "PRICING_UNAVAILABLE")
	default:
		return false
	}
	return true
}

// mapStayError writes the problem for check-in, extras and check-out errors and reports whether it matched. Field errors
// come from the domain as paths and codes only: no input value or error text reaches the client.
func mapStayError(w http.ResponseWriter, err error) bool {
	var ve *stay.ValidationError
	switch {
	case errors.Is(err, room.ErrNotVacant):
		writeProblem(w, http.StatusConflict, "Conflict", "ROOM_NOT_VACANT")
	case errors.Is(err, stay.ErrInsufficientStock):
		writeProblem(w, http.StatusConflict, "Conflict", "INSUFFICIENT_STOCK")
	case errors.Is(err, stay.ErrNotActive):
		writeProblem(w, http.StatusConflict, "Conflict", "STAY_NOT_ACTIVE")
	case errors.Is(err, app.ErrExpenseAutomatic):
		writeProblem(w, http.StatusConflict, "Conflict", "EXPENSE_AUTOMATIC")
	case errors.Is(err, app.ErrPayrollPaid):
		writeProblem(w, http.StatusConflict, "Conflict", "PAYROLL_PAID")
	case errors.Is(err, app.ErrPayrollNothing):
		writeProblem(w, http.StatusConflict, "Conflict", "PAYROLL_NOTHING_TO_PAY")
	case errors.Is(err, app.ErrPayrollFuture):
		writeProblem(w, http.StatusConflict, "Conflict", "PAYROLL_FUTURE")
	case errors.Is(err, app.ErrLeaveConflict):
		writeProblem(w, http.StatusConflict, "Conflict", "LEAVE_CONFLICT")
	case errors.Is(err, app.ErrLeaveState):
		writeProblem(w, http.StatusConflict, "Conflict", "LEAVE_STATE")
	case errors.Is(err, app.ErrRosterNotEmpty):
		writeProblem(w, http.StatusConflict, "Conflict", "ROSTER_NOT_EMPTY")
	case errors.Is(err, app.ErrPhotoTooLarge):
		writeProblem(w, http.StatusRequestEntityTooLarge, "Payload Too Large", "PHOTO_TOO_LARGE")
	case errors.Is(err, app.ErrUnsupportedMedia):
		writeProblem(w, http.StatusUnsupportedMediaType, "Unsupported Media Type", "UNSUPPORTED_MEDIA")
	case errors.Is(err, app.ErrGuestIDClosed):
		writeProblem(w, http.StatusConflict, "Conflict", "GUEST_ID_CLOSED")
	case errors.Is(err, app.ErrRoomOccupied):
		writeProblem(w, http.StatusConflict, "Conflict", "ROOM_OCCUPIED")
	case errors.Is(err, app.ErrTicketDone):
		writeProblem(w, http.StatusConflict, "Conflict", "TICKET_DONE")
	case errors.Is(err, app.ErrTicketStatus):
		writeProblem(w, http.StatusConflict, "Conflict", "TICKET_STATUS")
	case errors.Is(err, app.ErrShiftNotOpen):
		writeProblem(w, http.StatusConflict, "Conflict", "SHIFT_NOT_OPEN")
	case errors.As(err, &ve):
		errs := make([]fieldProblem, len(ve.Errors))
		for i, fe := range ve.Errors {
			errs[i] = fieldProblem{Code: fe.Code, Field: fe.Path}
		}
		writeValidation(w, errs)
	case errors.Is(err, pricing.ErrUnknownRentalType):
		writeValidation(w, []fieldProblem{{Code: codeInvalid, Field: "rentalType"}})
	case errors.Is(err, app.ErrInvalidIdempotencyKey):
		writeValidation(w, []fieldProblem{{Code: codeInvalid, Field: "Idempotency-Key"}})
	default:
		return false
	}
	return true
}

func writeValidation(w http.ResponseWriter, errs []fieldProblem) {
	writeProblemWith(w, http.StatusUnprocessableEntity, "Unprocessable Entity", "VALIDATION_FAILED", errs)
}

// mapPaymentError writes the problem for payment errors and reports whether it matched.
func mapPaymentError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, app.ErrInvoiceNotOpen):
		writeProblem(w, http.StatusConflict, "Conflict", "INVOICE_NOT_OPEN")
	case errors.Is(err, app.ErrEventNotDismissable):
		writeProblem(w, http.StatusConflict, "Conflict", "EVENT_NOT_DISMISSABLE")
	case errors.Is(err, app.ErrEventNotLinkable):
		writeProblem(w, http.StatusConflict, "Conflict", "EVENT_NOT_LINKABLE")
	case errors.Is(err, app.ErrLinkAmount):
		writeProblem(w, http.StatusConflict, "Conflict", "LINK_AMOUNT_MISMATCH")
	case errors.Is(err, app.ErrNoBankAccount):
		writeProblem(w, http.StatusConflict, "Conflict", "BANK_ACCOUNT_MISSING")
	default:
		return false
	}
	return true
}

func mapHousekeepingError(w http.ResponseWriter, err error) bool {
	if !errors.Is(err, app.ErrRoomNotToClean) {
		return false
	}
	writeProblem(w, http.StatusConflict, "Conflict", "ROOM_NOT_TO_CLEAN")
	return true
}
