package httpadapter

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

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
		if mapSessionError(w, err) || mapRoomError(w, err) || mapStayError(w, err) {
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
	switch {
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

// mapStayError writes the problem for check-in errors and reports whether it matched. Field errors
// come from the domain as paths and codes only: no input value or error text reaches the client.
func mapStayError(w http.ResponseWriter, err error) bool {
	var ve *stay.ValidationError
	switch {
	case errors.Is(err, room.ErrNotVacant):
		writeProblem(w, http.StatusConflict, "Conflict", "ROOM_NOT_VACANT")
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
