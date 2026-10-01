package httpadapter

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

const problemContentType = "application/problem+json"

// problemBody mirrors the Problem schema of the contract (required fields only).
type problemBody struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Code   string `json:"code"`
}

func writeProblem(w http.ResponseWriter, status int, title, code string) {
	w.Header().Set("Content-Type", problemContentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problemBody{Type: "about:blank", Title: title, Status: status, Code: code})
}

// badRequestResponse maps request decoding and parameter binding errors to 400. The error text
// is not echoed: it can quote client input.
func badRequestResponse(w http.ResponseWriter, _ *http.Request, _ error) {
	writeProblem(w, http.StatusBadRequest, "Bad Request", "BAD_REQUEST")
}

// problemResponder is the single place where handler errors become HTTP problem responses
// (CLAUDE.md §6 rule 10). Unknown errors are logged, never echoed to the client.
func problemResponder(log *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		if errors.Is(err, errNotImplemented) {
			writeProblem(w, http.StatusNotImplemented, "Not Implemented", "NOT_IMPLEMENTED")
			return
		}
		log.ErrorContext(r.Context(), "unhandled handler error", "error", err)
		writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "INTERNAL")
	}
}
