package buildinfo

import (
	"os"
	"strings"
	"testing"
)

// Direct developer builds and tests (including the reference PDFs from the
// visual capture) print this default, so it must match VERSION.
func TestDefaultVersionMatchesVERSIONFile(t *testing.T) {
	b, err := os.ReadFile("../../VERSION")
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.TrimSpace(string(b)); Version != want {
		t.Fatalf("buildinfo.Version = %q, VERSION = %q", Version, want)
	}
}
