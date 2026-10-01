package httpadapter

import "net/http"

// healthz is liveness only; it must never touch the database (readiness is /readyz, SG-003).
func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
