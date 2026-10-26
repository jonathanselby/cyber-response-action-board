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

	// T-166: the presentation shell dependency stays inside the web tier.
	for _, tier := range []string{"app", "domain", "security", "data", "ledger", "adapters"} {
		checkTree(t, filepath.Join(root, "internal", tier), func(path, imp string) {
			if strings.Contains(imp, "github.com/jonathanselby/go-app-shell") {
				t.Errorf("go-app-shell may only be imported by internal/presentation/web: %s -> %s", path, imp)
			}
		})
	}
	for _, other := range []string{"pdf", "mcp"} {
		checkTree(t, filepath.Join(root, "internal", "presentation", other), func(path, imp string) {
			if strings.Contains(imp, "github.com/jonathanselby/go-app-shell") {
				t.Errorf("go-app-shell may only be imported by internal/presentation/web: %s -> %s", path, imp)
			}
		})
	}

	checkTree(t, filepath.Join(root, "internal", "app"), func(path, imp string) {
		if strings.Contains(imp, "github.com/jackc/pgx") || strings.Contains(imp, "/internal/data/postgres") {
			t.Errorf("application tier illegally imports concrete PostgreSQL: %s -> %s", path, imp)
		}
	})

	checkTree(t, filepath.Join(root, "internal", "security"), func(path, imp string) {
		if strings.Contains(imp, "github.com/jackc/pgx") || strings.Contains(imp, "/internal/data/postgres") {
			t.Errorf("security tier illegally imports concrete PostgreSQL: %s -> %s", path, imp)
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
