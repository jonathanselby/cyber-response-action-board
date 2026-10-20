package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestTierImportBoundaries(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate source tree")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))

	checkTree(t, filepath.Join(root, "internal", "presentation"), func(path, imp string) {
		if strings.Contains(imp, "/internal/data") || strings.Contains(imp, "github.com/jackc/pgx") {
			t.Errorf("presentation tier illegally imports data/PostgreSQL: %s -> %s", path, imp)
		}
		if strings.Contains(imp, "/internal/security") {
			t.Errorf("presentation tier must not make security decisions: %s -> %s", path, imp)
		}
	})

	checkTree(t, filepath.Join(root, "internal", "app"), func(path, imp string) {
		if strings.Contains(imp, "github.com/jackc/pgx") || strings.Contains(imp, "/internal/data/postgres") {
			t.Errorf("application tier illegally imports concrete PostgreSQL: %s -> %s", path, imp)
		}
	})

	domain := filepath.Join(root, "internal", "domain")
	if _, err := os.Stat(domain); err == nil {
		checkTree(t, domain, func(path, imp string) {
			if strings.Contains(imp, "github.com/jackc/pgx") || strings.Contains(imp, "/internal/data/postgres") {
				t.Errorf("domain tier illegally imports concrete PostgreSQL: %s -> %s", path, imp)
			}
		})
	}
}

func checkTree(t *testing.T, root string, check func(path, imp string)) {
	t.Helper()
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return
		}
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range f.Imports {
			if spec.Path == nil {
				continue
			}
			imp, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			check(path, imp)
		}
		ast.Inspect(f, func(ast.Node) bool { return true })
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
