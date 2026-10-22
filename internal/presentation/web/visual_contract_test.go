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
		".crab-dashboard-grid",
		".crab-toolbar",
		"@media (max-width: 900px)",
		"@media (max-width: 520px)",
		"@media print",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("CRAB presentation contract missing %q", required)
		}
	}
	for _, forbidden := range []string{"body:not(.gas-body)", "@import url(", "fonts.googleapis.com", "cdnjs.cloudflare.com", "unpkg.com", ".gas-header {", ".gas-nav {", ".gas-main {"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("CRAB CSS must not recreate shell responsibility %q", forbidden)
		}
	}
}

func TestSpecialistSurfacesAreShellFragments(t *testing.T) {
	for _, name := range []string{
		"drop2_access.go",
		"drop2_communications.go",
		"drop2_control.go",
		"drop2_evidence.go",
		"drop2_obligations.go",
		"continuity_template.go",
	} {
		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := strings.ToLower(string(content))
		for _, forbidden := range []string{"<!doctype", "<html", "<head", "<body", "<header class=\"shell\"", "<nav class=\"crumb\""} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s still contains legacy full-document presentation %q", name, forbidden)
			}
		}
	}
	server, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(server), "drop2Navigation") {
		t.Fatal("server still wraps the shell in legacy HTML-injection navigation")
	}
}
