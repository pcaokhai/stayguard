package pricing

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestPricingPurity_SG101_AC4(t *testing.T) {
	// "io" is allowed only for the io.EOF sentinel that parse.go compares against; any other
	// io identifier would mean real I/O and fails below.
	banned := []string{"os", "io/fs", "net", "net/http", "database/sql", "log", "log/slog",
		"math/rand", "math/rand/v2", "crypto/rand", "os/exec"}
	clockCalls := []string{"Now", "Since", "Until", "Sleep"}
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
		for _, imp := range file.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			if slices.Contains(banned, path) {
				t.Errorf("%s imports %s", f, path)
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			id, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			if id.Name == "time" && slices.Contains(clockCalls, sel.Sel.Name) {
				t.Errorf("%s calls time.%s", f, sel.Sel.Name)
			}
			if id.Name == "io" && sel.Sel.Name != "EOF" {
				t.Errorf("%s uses io.%s", f, sel.Sel.Name)
			}
			return true
		})
	}
	if checked == 0 {
		t.Fatal("no non-test files parsed")
	}
}
