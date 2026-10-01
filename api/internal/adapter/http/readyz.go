package httpadapter

import (
	"log/slog"
	"net/http"

	"github.com/pcaokhai/stayguard/api/internal/app"
)

// readyz is readiness, outside the OpenAPI contract like healthz (plan SG-003 Ruling 5). The cause of a
// failure goes to the log only: it can carry hosts, roles or driver text.
func readyz(log *slog.Logger, probe app.ReadinessProbe) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := probe.Check(r.Context()); err != nil {
			log.ErrorContext(r.Context(), "readiness check failed", "error", err)
			writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable", "NOT_READY")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	}
}
