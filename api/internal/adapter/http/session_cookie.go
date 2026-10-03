package httpadapter

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/pcaokhai/stayguard/api/internal/adapter/http/gen"
)

const (
	// sessionCookie carries the same opaque token as the Bearer header, for browsers: it is HttpOnly, so page scripts
	// cannot read it, and SameSite=Strict, so other sites cannot send it.
	sessionCookie = "sg_session"
	// csrfHeader must be present on a cookie-authenticated request that changes state. A cross-site form or
	// simple request cannot add it, and a cross-origin fetch that does must pass a CORS preflight, which this API never grants.
	csrfHeader = "X-Requested-With"
	csrfValue  = "stayguard"
)

type metaKey struct{}

// reqMeta is what the cookie needs to know about the connection.
type reqMeta struct{ secure, localhost bool }

// withMeta records whether the request came over HTTPS (directly, or through the trusted proxy) and whether it is for localhost.
func withMeta(trustProxy bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m := reqMeta{secure: r.TLS != nil, localhost: isLocalhost(r.Host)}
			if v := r.Header.Get("X-Forwarded-Proto"); trustProxy && v != "" {
				parts := strings.Split(v, ",")
				m.secure = strings.EqualFold(strings.TrimSpace(parts[len(parts)-1]), "https") // the proxy's own entry
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), metaKey{}, m)))
		})
	}
}

func metaOf(ctx context.Context) reqMeta {
	m, _ := ctx.Value(metaKey{}).(reqMeta)
	return m
}

func isLocalhost(host string) bool {
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		h = host
	}
	h = strings.Trim(h, "[]")
	return h == "localhost" || h == "127.0.0.1" || h == "::1"
}

// sessionCookieHeader builds the Set-Cookie value for a new session, or "" when the connection is plain HTTP to a host
// that is not localhost: a cookie must never travel unencrypted, so those clients keep using the token. No Max-Age
// and no Expires: the cookie ends with the browser session, and the server decides expiry.
func sessionCookieHeader(ctx context.Context, token string) string {
	m := metaOf(ctx)
	if !m.secure && !m.localhost {
		return ""
	}
	c := http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: m.secure} //nolint:gosec // Secure follows the connection: HTTPS always, plain HTTP only on localhost (checked above)
	return c.String()
}

// clearCookieHeader expires the session cookie in the browser.
func clearCookieHeader(ctx context.Context) string {
	m := metaOf(ctx)
	c := http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: m.secure, MaxAge: -1} //nolint:gosec // clearing must match the attributes it was set with
	return c.String()
}

// isUnsafe is a method that changes state.
func isUnsafe(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return true
}

// csrfOK checks a cookie-authenticated state-changing request: the custom header, and an Origin (or, without one, a
// Referer) whose host is this server's host.
func csrfOK(r *http.Request) bool {
	if r.Header.Get(csrfHeader) != csrfValue {
		return false
	}
	src := r.Header.Get("Origin")
	if src == "" {
		src = r.Header.Get("Referer")
	}
	u, err := url.Parse(src)
	if src == "" || err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

// Responses that carry the session cookie next to the generated body. The cookie is added before the generated
// Visit writes the status line.
type signInWithCookie struct {
	gen.SignIn200JSONResponse
	cookie string
}

func (r signInWithCookie) VisitSignInResponse(w http.ResponseWriter) error {
	addCookie(w, r.cookie)
	return r.SignIn200JSONResponse.VisitSignInResponse(w)
}

type demoWithCookie struct {
	gen.CreateDemoSession201JSONResponse
	cookie string
}

func (r demoWithCookie) VisitCreateDemoSessionResponse(w http.ResponseWriter) error {
	addCookie(w, r.cookie)
	return r.CreateDemoSession201JSONResponse.VisitCreateDemoSessionResponse(w)
}

type signOutClearingCookie struct {
	gen.SignOut204Response
	cookie string
}

func (r signOutClearingCookie) VisitSignOutResponse(w http.ResponseWriter) error {
	addCookie(w, r.cookie)
	return r.SignOut204Response.VisitSignOutResponse(w)
}

func addCookie(w http.ResponseWriter, v string) {
	if v != "" {
		w.Header().Add("Set-Cookie", v)
	}
}
