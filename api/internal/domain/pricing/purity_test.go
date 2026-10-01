package pricing

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// "io" is allowed only for the io.EOF sentinel that parse.go compares against; any other
// io identifier would mean real I/O and is reported.
var (
	bannedImports = []string{"os", "io/fs", "net", "net/http", "database/sql", "log", "log/slog",
		"math/rand", "math/rand/v2", "crypto/rand", "os/exec"}
	bannedTimeFuncs = []string{"Now", "Since", "Until", "Sleep", "After", "Tick", "NewTimer", "NewTicker", "AfterFunc"}
)

// purityViolations resolves each import's local name (alias or dot import included) before
// looking for banned selectors, so `t "time"` and `. "time"` cannot hide a clock read.
func purityViolations(name string, file *ast.File) []string {
	var out []string
	local := map[string]string{} // local identifier -> import path
	dotTime := false
	for _, imp := range file.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		if slices.Contains(bannedImports, p) {
			out = append(out, fmt.Sprintf("%s imports %s", name, p))
		}
		switch {
		case imp.Name != nil && imp.Name.Name == ".":
			dotTime = dotTime || p == "time"
		case imp.Name != nil:
			local[imp.Name.Name] = p
		default:
			local[path.Base(p)] = p
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			id, ok := x.X.(*ast.Ident)
			if !ok {
				return true
			}
			switch local[id.Name] {
			case "time":
				if slices.Contains(bannedTimeFuncs, x.Sel.Name) {
					out = append(out, fmt.Sprintf("%s uses time.%s", name, x.Sel.Name))
				}
			case "io":
				if x.Sel.Name != "EOF" {
					out = append(out, fmt.Sprintf("%s uses io.%s", name, x.Sel.Name))
				}
			}
		case *ast.CallExpr:
			if id, ok := x.Fun.(*ast.Ident); ok && dotTime && slices.Contains(bannedTimeFuncs, id.Name) {
				out = append(out, fmt.Sprintf("%s uses dot-imported time.%s", name, id.Name))
			}
		}
		return true
	})
	return out
}

func TestPricingPurity_SG101_AC4(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	checked := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		checked++
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range purityViolations(f, file) {
			t.Error(v)
		}
	}
	if checked == 0 {
		t.Fatal("no non-test files parsed")
	}
}

func TestPurityCheckerFindsHiddenClockReads_SG101_AC4(t *testing.T) {
	parse := func(src string) *ast.File {
		f, err := parser.ParseFile(token.NewFileSet(), "x.go", src, 0)
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	dirty := parse(`package x
import (
	t "time"
	. "time"
	"os"
	r "io"
)
func f() { _ = t.Now(); Sleep(1); _ = t.NewTimer; _ = r.ReadAll; _ = t.Date }`)
	got := strings.Join(purityViolations("x.go", dirty), "\n")
	for _, want := range []string{"imports os", "time.Now", "dot-imported time.Sleep", "time.NewTimer", "io.ReadAll"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "time.Date") {
		t.Errorf("time.Date is pure but was flagged:\n%s", got)
	}
	clean := parse("package x\nimport \"time\"\nfunc f(a time.Time) time.Time { return a.Add(time.Hour) }")
	if v := purityViolations("x.go", clean); len(v) != 0 {
		t.Errorf("clean source flagged: %v", v)
	}
}
