package architecture

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// T-194: the time-status vocabulary lives in the ledger package, and only the
// kernel writes ledger entries, so every write passes through the one clock.
func TestTimeAuthorityHasOneWritePath(t *testing.T) {
	_, here, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))
	allowedWriters := map[string]bool{
		filepath.Join("internal", "data", "postgres", "kernel.go"):          true,
		filepath.Join("internal", "data", "postgres", "bootstrap_batch.go"): true,
	}
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.Walk(filepath.Join(root, dir), func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			text := string(raw)
			if strings.Contains(text, "INSERT INTO crab.ledger_entries") && !allowedWriters[rel] {
				t.Errorf("%s writes ledger entries outside the kernel", rel)
			}
			if strings.Contains(text, "ledger.BuildEntry(") && !allowedWriters[rel] {
				t.Errorf("%s builds ledger entries outside the kernel", rel)
			}
			if allowedWriters[rel] && strings.Contains(text, "ledger.BuildEntry(") && !strings.Contains(text, "k.stampTime(") {
				t.Errorf("%s builds ledger entries without the clock", rel)
			}
			if rel != filepath.Join("internal", "ledger", "ledger.go") && strings.Contains(text, `"trusted"`) {
				t.Errorf("%s asserts a time status literal; use the ledger constants and the clock", rel)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
