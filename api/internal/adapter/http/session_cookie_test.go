package httpadapter

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type cookieReq struct {
	method, path, host, body string
	headers                  map[string]string
	cookie, bearer           string
}

func (c cookieReq) do(h http.Handler) *httptest.ResponseRecorder {
	req := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
	req.Header.Set("Content-Type", "application/json")
	if c.host != "" {
		req.Host = c.host
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	if c.cookie != "" {
		req.Header.Set("Cookie", sessionCookie+"="+c.cookie)
	}
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func setCookie(rec *httptest.ResponseRecorder) string { return rec.Header().Get("Set-Cookie") }

func TestSignInCookie_Attributes_Cookie(t *testing.T) {
	h := authRouter(&fakeAuth{}, &bytes.Buffer{}, true)
	signIn := func(host string, hdr map[string]string) string {
		return setCookie(cookieReq{method: "POST", path: "/v1/auth/sign-in", host: host, body: signInBody, headers: hdr}.do(h))
	}
	https := signIn("app.example.com", map[string]string{"X-Forwarded-Proto": "https"})
	for _, want := range []string{"sg_session=tok", "Path=/", "HttpOnly", "SameSite=Strict", "Secure"} {
		if !strings.Contains(https, want) {
			t.Errorf("https cookie %q lacks %q", https, want)
		}
	}
	for _, bad := range []string{"Domain", "Max-Age", "Expires"} {
		if strings.Contains(https, bad) {
			t.Errorf("https cookie %q has %q", https, bad)
		}
	}
	local := signIn("localhost:8080", nil)
	if !strings.Contains(local, "sg_session=tok") || strings.Contains(local, "Secure") {
		t.Errorf("localhost http cookie: %q", local)
	}
	if c := signIn("app.example.com", nil); c != "" { // plain HTTP, not localhost: no cookie, the token still works
		t.Errorf("cookie over plain http: %q", c)
	}
	// X-Forwarded-Proto is ignored unless the proxy is trusted.
	untrusted := authRouter(&fakeAuth{}, &bytes.Buffer{}, false)
	rec := cookieReq{method: "POST", path: "/v1/auth/sign-in", host: "app.example.com", body: signInBody, headers: map[string]string{"X-Forwarded-Proto": "https"}}.do(untrusted)
	if c := setCookie(rec); c != "" {
		t.Errorf("forwarded proto trusted without TRUST_PROXY: %q", c)
	}
}

func TestSignInCookie_BodyKeepsToken_Cookie(t *testing.T) {
	rec := cookieReq{method: "POST", path: "/v1/auth/sign-in", host: "localhost", body: signInBody}.do(authRouter(&fakeAuth{}, &bytes.Buffer{}, false))
	if !strings.Contains(rec.Body.String(), `"accessToken":"tok"`) {
		t.Fatalf("body: %s", rec.Body)
	}
}

func TestCookieAuth_Cookie(t *testing.T) {
	h := authRouter(&fakeAuth{}, &bytes.Buffer{}, false)
	if rec := (cookieReq{method: "GET", path: "/v1/me", cookie: goodToken}).do(h); rec.Code != 200 {
		t.Fatalf("cookie GET /v1/me: %d %s", rec.Code, rec.Body)
	}
	if rec := (cookieReq{method: "GET", path: "/v1/me", cookie: "wrong"}).do(h); rec.Code != 401 {
		t.Fatalf("wrong cookie: %d", rec.Code)
	}
	// The Authorization header wins and is never mixed with the cookie.
	if rec := (cookieReq{method: "GET", path: "/v1/me", cookie: goodToken, bearer: "wrong"}).do(h); rec.Code != 401 {
		t.Fatalf("bad bearer with good cookie: %d", rec.Code)
	}
}

func TestCookieCSRF_Cookie(t *testing.T) {
	good := map[string]string{"X-Requested-With": "stayguard", "Origin": "http://example.com"}
	with := func(k, v string) map[string]string {
		m := map[string]string{}
		for a, b := range good {
			m[a] = b
		}
		if v == "" {
			delete(m, k)
		} else {
			m[k] = v
		}
		return m
	}
	cases := []struct {
		name    string
		method  string
		headers map[string]string
		want    int
	}{
		{"ok with origin", "POST", good, 204},
		{"ok with referer only", "POST", map[string]string{"X-Requested-With": "stayguard", "Referer": "http://example.com/vi/clean?room=1"}, 204},
		{"missing header", "POST", with("X-Requested-With", ""), 403},
		{"wrong header value", "POST", with("X-Requested-With", "x"), 403},
		{"cross-origin", "POST", with("Origin", "https://evil.test"), 403},
		{"cross-origin referer", "POST", map[string]string{"X-Requested-With": "stayguard", "Referer": "https://evil.test/"}, 403},
		{"no origin or referer", "POST", map[string]string{"X-Requested-With": "stayguard"}, 403},
		{"null origin", "POST", with("Origin", "null"), 403},
		{"PUT checked", "PUT", with("Origin", "https://evil.test"), 403},
		{"PATCH checked", "PATCH", with("X-Requested-With", ""), 403},
		{"DELETE checked", "DELETE", with("X-Requested-With", ""), 403},
	}
	h := authRouter(&fakeAuth{}, &bytes.Buffer{}, false)
	for _, c := range cases {
		rec := cookieReq{method: c.method, path: "/v1/auth/sign-out", headers: c.headers, cookie: goodToken}.do(h)
		if c.want == 403 {
			if status, code := problemOf(t, rec); rec.Code != 403 || status != 403 || code != "CSRF_REJECTED" {
				t.Errorf("%s: %d %s", c.name, rec.Code, rec.Body)
			}
		} else if rec.Code == 403 || rec.Code == 401 {
			t.Errorf("%s: %d %s", c.name, rec.Code, rec.Body)
		}
	}
	// GET is unaffected; Bearer clients never need the header.
	if rec := (cookieReq{method: "GET", path: "/v1/me", cookie: goodToken}).do(h); rec.Code != 200 {
		t.Errorf("GET with cookie only: %d", rec.Code)
	}
	if rec := (cookieReq{method: "POST", path: "/v1/auth/sign-out", bearer: goodToken}).do(h); rec.Code != 204 {
		t.Errorf("Bearer POST without csrf header: %d %s", rec.Code, rec.Body)
	}
}

func TestSignOutClearsCookie_Cookie(t *testing.T) {
	a := &fakeAuth{}
	h := authRouter(a, &bytes.Buffer{}, false)
	rec := cookieReq{method: "POST", path: "/v1/auth/sign-out", host: "localhost:8080", cookie: goodToken,
		headers: map[string]string{"X-Requested-With": "stayguard", "Origin": "http://localhost:8080"}}.do(h)
	c := setCookie(rec)
	if rec.Code != 204 || !a.signedOut || !strings.Contains(c, "sg_session=;") || !strings.Contains(c, "Max-Age=0") {
		t.Fatalf("code=%d signedOut=%v cookie=%q", rec.Code, a.signedOut, c)
	}
}

func TestCookieOneTimePinSession_Cookie(t *testing.T) {
	h := authRouter(&fakeAuth{}, &bytes.Buffer{}, false)
	hdr := map[string]string{"X-Requested-With": "stayguard", "Origin": "http://example.com"}
	rec := cookieReq{method: "POST", path: "/v1/rooms/r1/stays", headers: hdr, cookie: oneTimeToken, body: "{}"}.do(h)
	if status, code := problemOf(t, rec); rec.Code != 403 || status != 403 || code != "PIN_CHANGE_REQUIRED" {
		t.Fatalf("one-time session reached another route: %d %s", rec.Code, rec.Body)
	}
	if rec := (cookieReq{method: "PUT", path: "/v1/me/pin", headers: hdr, cookie: oneTimeToken, body: `{"currentPin":"482915","newPin":"739106"}`}).do(h); rec.Code != 204 {
		t.Fatalf("changeMyPin with cookie: %d %s", rec.Code, rec.Body)
	}
}

func TestWebhookIgnoresCookie_Cookie(t *testing.T) {
	h := authRouter(&fakeAuth{}, &bytes.Buffer{}, false)
	rec := cookieReq{method: "POST", path: "/v1/webhooks/bank/hk_x", cookie: goodToken, body: "{}"}.do(h)
	if rec.Code == 403 && strings.Contains(rec.Body.String(), "CSRF_REJECTED") {
		t.Fatalf("webhook went through the cookie check: %s", rec.Body)
	}
	if setCookie(rec) != "" {
		t.Fatalf("webhook set a cookie")
	}
}
