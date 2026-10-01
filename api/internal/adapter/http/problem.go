package httpadapter

import (
	"encoding/json"
	"errors"
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

// problemResponse is the single place where handler errors become HTTP problem responses
// (CLAUDE.md §6 rule 10). Unknown errors are not echoed to the client.
func problemResponse(w http.ResponseWriter, _ *http.Request, err error) {
	p := problemBody{Type: "about:blank", Title: "Internal Server Error", Status: http.StatusInternalServerError, Code: "INTERNAL"}
	if errors.Is(err, errNotImplemented) {
		p = problemBody{Type: "about:blank", Title: "Not Implemented", Status: http.StatusNotImplemented, Code: "NOT_IMPLEMENTED"}
	}
	w.Header().Set("Content-Type", problemContentType)
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}
