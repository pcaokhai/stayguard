package httpadapter

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/pcaokhai/stayguard/api/internal/adapter/http/gen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// AuthService is the PIN sign-in surface the handlers need.
type AuthService interface {
	SignIn(ctx context.Context, ip, code, username, pin string) (app.SignInResult, error)
	SignOut(ctx context.Context, c app.Caller) error
	ChangePin(ctx context.Context, c app.Caller, current, next string) error
}

type ipKey struct{}

// clientIP puts the caller's address on the context for rate limiting. Behind a trusted proxy it is the
// last X-Forwarded-For entry (the one the proxy appended); otherwise the TCP peer, which a client cannot forge.
func clientIP(trustProxy bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			if xff := r.Header.Get("X-Forwarded-For"); trustProxy && xff != "" {
				parts := strings.Split(xff, ",")
				ip = strings.TrimSpace(parts[len(parts)-1])
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ipKey{}, ip)))
		})
	}
}

func (s Server) SignIn(ctx context.Context, req gen.SignInRequestObject) (gen.SignInResponseObject, error) {
	if req.Body == nil || req.Body.Pin == nil {
		return nil, &app.ValidationError{Field: "pin", Reason: "required"}
	}
	ip, _ := ctx.Value(ipKey{}).(string)
	r, err := s.auth.SignIn(ctx, ip, req.Body.GuesthouseCode, req.Body.Username, *req.Body.Pin)
	if err != nil {
		return nil, err
	}
	return gen.SignIn200JSONResponse(gen.SignInResponse{
		AccessToken: r.Token, ExpiresAt: r.ExpiresAt, TenantId: r.TenantID, User: toUser(r.User), MustChangePin: r.MustChangePin,
	}), nil
}

func (s Server) SignOut(ctx context.Context, _ gen.SignOutRequestObject) (gen.SignOutResponseObject, error) {
	c, ok := app.CallerFrom(ctx)
	if !ok {
		return nil, app.ErrUnauthenticated
	}
	if err := s.auth.SignOut(ctx, c); err != nil {
		return nil, err
	}
	return gen.SignOut204Response{}, nil
}

func (s Server) ChangeMyPin(ctx context.Context, req gen.ChangeMyPinRequestObject) (gen.ChangeMyPinResponseObject, error) {
	c, ok := app.CallerFrom(ctx)
	if !ok {
		return nil, app.ErrUnauthenticated
	}
	if req.Body == nil || req.Body.CurrentPin == nil || req.Body.NewPin == nil {
		return nil, &app.ValidationError{Field: "pin", Reason: "required"}
	}
	if err := s.auth.ChangePin(ctx, c, *req.Body.CurrentPin, *req.Body.NewPin); err != nil {
		return nil, err
	}
	return gen.ChangeMyPin204Response{}, nil
}
