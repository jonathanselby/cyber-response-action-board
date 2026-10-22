package web

import (
	"os"
	"strings"
	"testing"
)

func TestVisualAccessibilityContract(t *testing.T) {
	css, err := os.ReadFile("static/app.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(css)
	for _, required := range []string{
		".crab-home-layout",
		".crab-board-grid",
		"@media (max-width: 900px)",
		"@media (max-width: 520px)",
		"@media print",
		"body:not(.gas-body)",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("CRAB presentation contract missing %q", required)
		}
	}
	for _, forbidden := range []string{"@import url(", "fonts.googleapis.com", "cdnjs.cloudflare.com", "unpkg.com", ".gas-header {", ".gas-nav {", ".gas-main {"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("CRAB CSS must not recreate shell responsibility %q", forbidden)
		}
	}
}
