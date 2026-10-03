package app

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// auditActionConsts reads every string constant named audit* in this package's non-test files, except the id prefixes.
func auditActionConsts(t *testing.T) map[string]string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	out := map[string]string{}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, sp := range gd.Specs {
				vs := sp.(*ast.ValueSpec)
				for i, n := range vs.Names {
					if !strings.HasPrefix(n.Name, "audit") || i >= len(vs.Values) || n.Name == "auditPrefix" || n.Name == "auditIDPrefix" {
						continue
					}
					if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						v, _ := strconv.Unquote(lit.Value)
						out[v] = n.Name
					}
				}
			}
		}
	}
	return out
}

// intentionallyDefault lists actions that belong in the default category on purpose.
var intentionallyDefault = map[string]bool{}

func TestAuditCategories_NoActionFallsToDefault_Audit(t *testing.T) {
	codes := auditActionConsts(t)
	if len(codes) < 50 {
		t.Fatalf("found only %d audit action constants; the parser lost some", len(codes))
	}
	var bad []string
	for code := range codes {
		if !hasCategoryPrefix(code) && !intentionallyDefault[code] {
			bad = append(bad, code)
		}
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Fatalf("actions with no category prefix (they would show as %s): %v", defaultAuditCategory, bad)
	}
}

// The filter for a category and the category of an action must agree for every action.
func TestAuditCategories_FilterMatchesCategory_Audit(t *testing.T) {
	cats := map[string]bool{}
	for _, c := range auditCategories {
		cats[c.category] = true
	}
	for code := range auditActionConsts(t) {
		want := auditCategoryOf(code)
		for cat := range cats {
			patterns, _ := auditPrefixesOf(cat)
			matched := false
			for _, p := range patterns {
				matched = matched || strings.HasPrefix(code, strings.TrimSuffix(p, "%"))
			}
			if matched != (cat == want) {
				t.Errorf("%s: category %s, but the %s filter matched=%v", code, want, cat, matched)
			}
		}
	}
}

// A prefix that no action matches is dead weight and hides a typo.
func TestAuditCategories_NoUnusedPrefix_Audit(t *testing.T) {
	codes := auditActionConsts(t)
	for _, c := range auditCategories {
		used := false
		for code := range codes {
			used = used || strings.HasPrefix(code, c.prefix)
		}
		if !used {
			t.Errorf("prefix %q (%s) matches no audit action", c.prefix, c.category)
		}
	}
}

func hasCategoryPrefix(action string) bool {
	for _, c := range auditCategories {
		if strings.HasPrefix(action, c.prefix) {
			return true
		}
	}
	return false
}

// contracts/audit-actions.json lists every audit action once, with the category auditCategoryOf gives it.
func TestAuditActionsFile_ListsEveryConstant_Audit(t *testing.T) {
	raw, err := os.ReadFile("../../../contracts/audit-actions.json")
	if err != nil {
		t.Fatal(err)
	}
	var file map[string]struct {
		Category string   `json:"category"`
		Details  []string `json:"details"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	codes := auditActionConsts(t)
	for code := range codes {
		e, ok := file[code]
		if !ok {
			t.Errorf("%s is not listed in contracts/audit-actions.json", code)
			continue
		}
		if want := auditCategoryOf(code); e.Category != want {
			t.Errorf("%s: file says %s, auditCategoryOf says %s", code, e.Category, want)
		}
	}
	for code := range file {
		if _, ok := codes[code]; !ok {
			t.Errorf("%s is in contracts/audit-actions.json but no audit constant has it", code)
		}
	}
}
