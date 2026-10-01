package httpadapter

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/pcaokhai/stayguard/api/internal/app"
)

const (
	v1Prefix     = "/v1/"
	bearerScheme = "bearer"
)

// publicRoutes are the only /v1 routes served without a session token. Everything else under /v1
// is protected by default, registered or not, so a new route cannot ship open by accident.
// The bank webhook authenticates by signature in its own story.
var publicRoutes = map[string]bool{
	"POST /v1/demo/sessions": true,
	"POST /v1/webhooks/bank": true,
}

type authenticator interface {
	Authenticate(ctx context.Context, token string) (app.Caller, error)
}

// authenticate requires a bearer token on /v1 and puts the resolved caller on the context.
// The token is read from the Authorization header only, and neither it nor the header is logged.
func authenticate(log *slog.Logger, a authenticator) func(http.Handler) http.Handler {
	respond := problemResponder(log)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, v1Prefix) || publicRoutes[r.Method+" "+r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			token, ok := bearerToken(r.Header.Get("Authorization"))
			if !ok {
				respond(w, r, app.ErrUnauthenticated)
				return
			}
			caller, err := a.Authenticate(r.Context(), token)
			if err != nil {
				respond(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(app.WithCaller(r.Context(), caller)))
		})
	}
}

// bearerToken accepts exactly "Bearer <token>" (scheme case-insensitive, token without spaces).
func bearerToken(h string) (string, bool) {
	scheme, token, found := strings.Cut(h, " ")
	if !found || !strings.EqualFold(scheme, bearerScheme) || token == "" || strings.ContainsAny(token, " \t") {
		return "", false
	}
	return token, true
}
