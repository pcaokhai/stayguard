package httpadapter

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func webDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

func TestStaticServesFilesAndIndex_SG001_AC3(t *testing.T) {
	h := newTestRouterDir(&bytes.Buffer{}, webDir(t, map[string]string{
		"index.html": "INDEX", "about.html": "ABOUT", "_next/app.js": "JS",
	}))
	for path, want := range map[string]string{"/": "INDEX", "/about": "ABOUT", "/_next/app.js": "JS"} {
		if rec := get(h, path); rec.Code != 200 || rec.Body.String() != want {
			t.Errorf("%s: code=%d body=%q want %q", path, rec.Code, rec.Body.String(), want)
		}
	}
}

func TestStaticUnknownPathFallsBackToIndex_SG001_AC3(t *testing.T) {
	h := newTestRouterDir(&bytes.Buffer{}, webDir(t, map[string]string{"index.html": "INDEX"}))
	if rec := get(h, "/stays/42"); rec.Code != 200 || rec.Body.String() != "INDEX" {
		t.Fatalf("code=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestStaticUnknownPathServes404Page_SG001_AC3(t *testing.T) {
	h := newTestRouterDir(&bytes.Buffer{}, webDir(t, map[string]string{"index.html": "INDEX", "404.html": "NOPE"}))
	if rec := get(h, "/missing"); rec.Code != 404 || rec.Body.String() != "NOPE" {
		t.Fatalf("code=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestStaticDoesNotShadowHealthzOrAPI_SG001_AC3(t *testing.T) {
	h := newTestRouterDir(&bytes.Buffer{}, webDir(t, map[string]string{"index.html": "INDEX", "healthz": "SHADOW"}))
	if rec := get(h, "/healthz"); rec.Body.String() != `{"status":"ok"}` {
		t.Errorf("healthz shadowed: %q", rec.Body.String())
	}
	if rec := get(h, "/v1/stays"); rec.Code != 404 || rec.Body.String() == "INDEX" {
		t.Errorf("api path served web: code=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestStaticRejectsTraversal_SG001_AC3(t *testing.T) {
	parent := webDir(t, map[string]string{"secret.txt": "SECRET", "web/index.html": "INDEX"})
	h := newTestRouterDir(&bytes.Buffer{}, filepath.Join(parent, "web"))
	for _, p := range []string{"/../secret.txt", "/%2e%2e/secret.txt"} {
		if rec := get(h, p); rec.Body.String() == "SECRET" {
			t.Errorf("%s escaped static dir", p)
		}
	}
}
