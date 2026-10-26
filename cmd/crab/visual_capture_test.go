package main

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// TestVisualCapture renders every screen with seeded data into static HTML
// plus the assets it references, for screenshot review (T-211). It runs only
// when CRAB_VISUAL_CAPTURE_DIR is set; scripts/visual-capture.sh drives it.
func TestVisualCapture(t *testing.T) {
	dir := os.Getenv("CRAB_VISUAL_CAPTURE_DIR")
	if dir == "" {
		t.Skip("CRAB_VISUAL_CAPTURE_DIR not set")
	}
	e := newE2E(t)
	save := func(name, body string) {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	page := func(name, path, cookie string) {
		w := e.do(http.MethodGet, path, cookie, nil)
		save("pages/"+name+".html", w.Body.String())
	}
	page("01-setup", "/", "")
	alice, _ := seedDemo(t, e)
	page("02-signin", "/", "")
	page("03-command-summary", "/", alice)
	page("04-approvals", "/approvals", alice)
	page("05-first-reports", "/intake", alice)
	page("06-board", "/incident/INC-014", alice)
	evidence := e.get("/incident/INC-014/evidence", alice)
	save("pages/07-evidence.html", evidence)
	if m := regexp.MustCompile(`href="(/evidence/[^"/]+)"`).FindStringSubmatch(evidence); m != nil {
		page("08-evidence-detail", m[1], alice)
	}
	page("09-communications", "/incident/INC-014/communications", alice)
	page("10-access", "/incident/INC-014/access", alice)
	page("11-close-out", "/incident/INC-014/close", alice)
	page("12-readiness", "/readiness", alice)
	page("13-obligation-library", "/obligations", alice)
	page("14-access-admin", "/access", alice)
	page("15-offline", "/offline", "")
	page("16-declare-open", "/?open=declare", alice)

	for _, asset := range []string{"/_app-shell/shell.css", "/_app-shell/pwa.js", "/static/crab.css", "/static/logo.png", "/static/logo-dark.png", "/static/icon.svg", "/static/favicon.png"} {
		w := e.do(http.MethodGet, asset, "", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("asset %s code=%d", asset, w.Code)
		}
		save(asset[1:], w.Body.String())
	}
	for _, pdf := range []struct{ name, path string }{
		{"brief-colour.pdf", "/incident/INC-014/brief.pdf?size=A4&mode=colour"},
		{"brief-mono.pdf", "/incident/INC-014/brief.pdf?size=A4&mode=monochrome"},
		{"pack-letter.pdf", "/pack.pdf?size=LETTER&mode=colour"},
	} {
		w := e.do(http.MethodGet, pdf.path, alice, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("%s code=%d", pdf.path, w.Code)
		}
		save("pdf/"+pdf.name, w.Body.String())
	}
}
