package architecture

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// T-252 (AC-12): a notification route is never a credential-recovery or
// verification route. Sign-in, enrolment and credential code never read it.
func TestNotificationRouteIsNeverARecoveryRoute(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate source tree")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))
	files := []string{filepath.Join(root, "internal", "app", "authentication.go")}
	for _, dir := range []string{filepath.Join(root, "internal", "security", "authn"), filepath.Join(root, "internal", "security", "ceremony")} {
		matches, _ := filepath.Glob(filepath.Join(dir, "*.go"))
		files = append(files, matches...)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, banned := range []string{"notification_routes", "RouteService", "NoticeService", ".Route(", "SetRoute"} {
			if strings.Contains(string(raw), banned) {
				t.Errorf("%s refers to %s: notification routes must never be used for sign-in or recovery", f, banned)
			}
		}
	}
}
