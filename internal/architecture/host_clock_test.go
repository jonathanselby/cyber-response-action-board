package architecture

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// hostClockAllowed lists the only places that may read the host clock
// directly, each with its reason. Everything else uses the governed clock
// (the kernel's clock, or internal/adapters/clock), so records and judgements
// such as overdue follow governed time (F-11, R-19).
var hostClockAllowed = map[string]string{
	"internal/adapters/clock/clock.go":       "the clock adapter itself",
	"internal/adapters/notifier/notifier.go": "measures a delivery duration (monotonic), not a time of record",
	"internal/security/authn/session.go":     "default when no clock is injected; the runtime injects the governed clock",
	"cmd/crab/operator_runtime.go":           "record-ID fallback when the random source fails; uniqueness only",
	"internal/presentation/web/server.go":    "health endpoint reports the host clock as such",
	"internal/presentation/web/incident.go":  "download file name only",
	"internal/proof/proof_on.go":             "test-only sampler timestamps, compiled only with -tags proof",
}

// F-11: no new direct host-clock reads.
func TestHostClockReadsAreConfined(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate source tree")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))
	seen := map[string]bool{}
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(b), "time.Now()") {
				rel, _ := filepath.Rel(root, path)
				rel = filepath.ToSlash(rel)
				seen[rel] = true
				if _, ok := hostClockAllowed[rel]; !ok {
					t.Errorf("%s reads the host clock; use the governed clock", rel)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for rel := range hostClockAllowed {
		if !seen[rel] {
			t.Errorf("%s no longer reads the host clock: remove it from hostClockAllowed", rel)
		}
	}
}
