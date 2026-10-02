package httpadapter

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/pcaokhai/stayguard/api/internal/app"
)

// WebhookService receives provider webhooks.
type WebhookService interface {
	Receive(ctx context.Context, hookID, signature, timestamp string, body []byte) error
}

const webhookPrefix = "/v1/webhooks/bank/"

// isWebhook is POST /v1/webhooks/bank/{hookId}: authenticated by signature, not by session.
func isWebhook(method, path string) bool {
	return method == http.MethodPost && strings.HasPrefix(path, webhookPrefix) && len(path) > len(webhookPrefix) && !strings.Contains(path[len(webhookPrefix):], "/")
}

// receiveWebhook is a plain handler, not a strict operation: the signature covers the raw bytes of the body, which a
// decoded and re-encoded body would not reproduce. SePay wants status 200 or 201 and exactly {"success": true}.
func receiveWebhook(log *slog.Logger, svc WebhookService) http.HandlerFunc {
	respond := problemResponder(log)
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeProblem(w, http.StatusRequestEntityTooLarge, "Payload Too Large", "PAYLOAD_TOO_LARGE")
			return
		}
		err = svc.Receive(r.Context(), chi.URLParam(r, "hookId"), r.Header.Get("X-SePay-Signature"), r.Header.Get("X-SePay-Timestamp"), body)
		switch {
		case err == nil:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success": true}`))
		case errors.Is(err, app.ErrWebhookUnknown):
			writeProblem(w, http.StatusNotFound, "Not Found", "NOT_FOUND")
		case errors.Is(err, app.ErrWebhookRejected):
			writeProblem(w, http.StatusUnauthorized, "Unauthorized", "WEBHOOK_REJECTED") // no detail on why
		default:
			respond(w, r, err)
		}
	}
}
