package httpadapter

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

const (
	indexPage    = "index.html"
	notFoundPage = "404.html"
	// apiPrefix paths belong to the API; an unknown one must 404, never return web HTML.
	apiPrefix = "/v1/"
)

// staticHandler serves the Next.js export. os.DirFS refuses ".." so requests cannot
// leave dir. Lookup order matches the export layout: file, name.html, name/index.html.
// Unknown paths get 404.html (status 404) when the export has one, else index.html
// so client-side routes still load.
func staticHandler(dir string) http.Handler {
	fsys := os.DirFS(dir)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (r.Method != http.MethodGet && r.Method != http.MethodHead) || strings.HasPrefix(r.URL.Path, apiPrefix) {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		for _, c := range candidates(name) {
			if isFile(fsys, c) {
				http.ServeFileFS(w, r, fsys, c)
				return
			}
		}
		if page, err := fs.ReadFile(fsys, notFoundPage); err == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write(page)
			return
		}
		http.ServeFileFS(w, r, fsys, indexPage)
	})
}

func candidates(name string) []string {
	if name == "" {
		return []string{indexPage}
	}
	return []string{name, name + ".html", name + "/" + indexPage}
}

func isFile(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && info.Mode().IsRegular()
}
