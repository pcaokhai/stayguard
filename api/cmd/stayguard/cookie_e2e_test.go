//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"testing"
)

// A browser session: sign in, use the cookie alone (a new tab shares the jar), sign out; the server ends the session.
func TestCookieSession_SignInUseSignOut_Cookie(t *testing.T) {
	e := newEnv(t)
	e.seedOwner()
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}
	do := func(method, path string, hdr map[string]string, body string) (int, http.Header) {
		req, _ := http.NewRequest(method, e.srv.URL+path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		res, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		return res.StatusCode, res.Header
	}
	csrf := map[string]string{"X-Requested-With": "stayguard", "Origin": e.srv.URL}

	in := `{"guesthouseCode":"` + staffCode + `","username":"owner1","pin":"` + ownerPIN + `"}`
	req, _ := http.NewRequest("POST", e.srv.URL+"/v1/auth/sign-in", bytes.NewBufferString(in))
	req.Header.Set("Content-Type", "application/json")
	res, err := c.Do(req)
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("sign-in: %v %v", err, res)
	}
	var body struct {
		AccessToken string `json:"accessToken"`
	}
	_ = json.NewDecoder(res.Body).Decode(&body)
	_ = res.Body.Close()
	if body.AccessToken == "" || len(res.Cookies()) != 1 || res.Cookies()[0].Value != body.AccessToken || !res.Cookies()[0].HttpOnly {
		t.Fatalf("cookie: %+v", res.Cookies())
	}

	if st, _ := do("GET", "/v1/me", nil, ""); st != 200 {
		t.Fatalf("GET /v1/me with the cookie alone: %d", st)
	}
	if st, _ := do("POST", "/v1/auth/sign-out", nil, ""); st != 403 {
		t.Fatalf("sign-out without the csrf proof: %d", st)
	}
	if st, h := do("POST", "/v1/auth/sign-out", csrf, ""); st != 204 || h.Get("Set-Cookie") == "" {
		t.Fatalf("sign-out: %d %v", st, h)
	}
	if st, _ := do("GET", "/v1/me", nil, ""); st != 401 {
		t.Fatalf("cookie still works after sign-out: %d", st)
	}
	if r := e.call("GET", "/v1/me", body.AccessToken, nil); r.status != 401 {
		t.Fatalf("session not revoked on the server: %d", r.status)
	}
}
