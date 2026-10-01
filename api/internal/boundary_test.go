package internal_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const modulePrefix = "github.com/pcaokhai/stayguard/api/internal/"

// forbidden returns why file (a path relative to internal/, slash separated) may not import pkg
// (relative to internal/), or "" when allowed. cmd is outside internal and exempt.
func forbidden(file, pkg string) string {
	layer, rest, _ := strings.Cut(file, "/")
	switch layer {
	case "domain":
		if !strings.HasPrefix(pkg, "domain") {
			return "domain imports nothing but domain"
		}
	case "app":
		if !strings.HasPrefix(pkg, "domain") && !strings.HasPrefix(pkg, "app") {
			return "app imports only domain"
		}
	case "platform":
		if strings.HasPrefix(pkg, "adapter") {
			return "platform must not import adapters"
		}
	case "adapter":
		adapter, _, _ := strings.Cut(rest, "/")
		other, _, _ := strings.Cut(strings.TrimPrefix(pkg, "adapter/"), "/")
		if strings.HasPrefix(pkg, "adapter/") && other != adapter {
			return "an adapter must not import another adapter"
		}
	}
	return ""
}

func TestImportBoundaries_SG003(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		rel := filepath.ToSlash(path)
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if !strings.HasPrefix(p, modulePrefix) {
				if why := externalForbidden(rel, p); why != "" {
					t.Errorf("%s imports %s: %s", rel, p, why)
				}
				continue
			}
			if why := forbidden(rel, strings.TrimPrefix(p, modulePrefix)); why != "" {
				t.Errorf("%s imports %s: %s", rel, p, why)
			}
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

// externalForbidden keeps I/O libraries out of domain and app (no framework or I/O there, CLAUDE.md §6 rule 8).
func externalForbidden(file, pkg string) string {
	layer, _, _ := strings.Cut(file, "/")
	if layer != "domain" && layer != "app" {
		return ""
	}
	for _, bad := range []string{"github.com/jackc/", "net/http", "database/sql", "github.com/go-chi/"} {
		if strings.HasPrefix(pkg, bad) {
			return layer + " must not import " + bad + " (I/O belongs in adapters)"
		}
	}
	return ""
}
