package web

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

// T-197: preview uses the same route and authorisation as download, returns
// the PDF inline with no-store and nosniff; download stays an attachment.
func TestPDFPreviewIsInlineAndAuthorised(t *testing.T) {
	runtime := &fakeRuntime{bootstrapped: true}
	s := withRuntime(t, runtime)
	for _, path := range []string{"/incident/INC-1/brief.pdf?view=inline", "/incident/INC-1/pack.pdf?view=inline", "/pack.pdf?view=inline&size=LETTER"} {
		w := get(t, s, path)
		h := w.Header()
		if w.Code != http.StatusOK || h.Get("Content-Type") != "application/pdf" || !strings.HasPrefix(h.Get("Content-Disposition"), "inline;") || h.Get("Cache-Control") != "no-store" || h.Get("X-Content-Type-Options") != "nosniff" || h.Get("X-Frame-Options") != "DENY" || !strings.Contains(h.Get("Content-Security-Policy"), "frame-ancestors 'none'") {
			t.Fatalf("%s code=%d headers=%v", path, w.Code, h)
		}
	}
	w := get(t, s, "/pack.pdf")
	if w.Code != http.StatusOK || !strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment;") {
		t.Fatalf("organisation pack download: code=%d headers=%v", w.Code, w.Header())
	}
	runtime.pdfErr = errors.New("authorisation denied")
	w = get(t, s, "/incident/INC-1/brief.pdf?view=inline")
	if w.Code != http.StatusForbidden || w.Header().Get("Content-Type") == "application/pdf" || !strings.Contains(w.Body.String(), "gas-header") {
		t.Fatalf("refused preview must render in frame without PDF bytes: code=%d", w.Code)
	}
}

// Every paper form offers Preview (new tab) and Download.
func TestPaperFormsOfferPreview(t *testing.T) {
	s := withRuntime(t, &fakeRuntime{bootstrapped: true})
	for _, path := range []string{"/incident/" + testIncident, "/incident/" + testIncident + "/close", "/readiness"} {
		body := get(t, s, path).Body.String()
		if !strings.Contains(body, `name="view" value="inline" formtarget="_blank"`) || !strings.Contains(body, "Download PDF") {
			t.Fatalf("%s lacks preview and download", path)
		}
	}
}
