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
		":focus-visible",
		"min-height: 44px",
		"@media (max-width: 820px)",
		"@media (max-width: 520px)",
		"@media print",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("visual/accessibility CSS contract missing %q", required)
		}
	}
	for _, forbidden := range []string{"@import url(", "fonts.googleapis.com", "cdnjs.cloudflare.com", "unpkg.com"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("core UI must not depend on remote asset %q", forbidden)
		}
	}
}
