package web

import (
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/jonathanselby/go-app-shell/shell"
)

// TestVisualAccessibilityContract checks the mechanical contract behind
// AC-43/AC-87: the shell supplies focus, touch targets, responsive layout and
// print; CRAB adds only a small token stylesheet with no remote assets (T-185).
func TestVisualAccessibilityContract(t *testing.T) {
	shellFS, err := shell.AssetsFS()
	if err != nil {
		t.Fatal(err)
	}
	shellCSS, err := fs.ReadFile(shellFS, "shell.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{":focus-visible", "min-height: 2.75rem", "@media (min-width: 48rem)", "@media (min-width: 64rem)", "@media print", "prefers-color-scheme: dark", "prefers-reduced-motion"} {
		if !strings.Contains(string(shellCSS), required) {
			t.Fatalf("shell visual/accessibility contract missing %q", required)
		}
	}
	css, err := os.ReadFile("static/crab.css")
	if err != nil {
		t.Fatal(err)
	}
	if len(css) > 2048 {
		t.Fatalf("crab.css is %d bytes; budget is 2 KiB (T-185)", len(css))
	}
	text := string(css)
	for _, forbidden := range []string{"@import", "url(", "fonts.googleapis.com", "cdnjs.cloudflare.com", "unpkg.com", "grid-template", ".gas-layout", ".gas-nav{", "width:"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("crab.css must hold tokens, native elements and print only; found %q", forbidden)
		}
	}
	for _, required := range []string{"--gas-accent", "prefers-color-scheme:dark", "@media print"} {
		if !strings.Contains(text, required) {
			t.Fatalf("crab.css missing %q", required)
		}
	}
}
