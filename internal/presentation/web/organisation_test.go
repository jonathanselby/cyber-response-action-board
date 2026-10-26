package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// The Organisation screen shows the scale and emergency information and
// round-trips the plain-text emergency form.
func TestOrganisationScreenAndEmergencyForm(t *testing.T) {
	runtime := &fakeRuntime{bootstrapped: true}
	s := withRuntime(t, runtime)
	body := get(t, s, "/organisation").Body.String()
	for _, want := range []string{"<h1>Organisation</h1>", "Severity scale", "Severity level 1", "Least severe", "Bafflewick Ltd", "Example Mutual", "Ransomware first hour", "Alice Smith | Incident commander | +44 7700 900001 | +44 20 7946 0001", "## Ransomware first hour"} {
		if !strings.Contains(body, want) {
			t.Fatalf("organisation screen missing %q", want)
		}
	}
	w := postForm(t, s, "/organisation/emergency", url.Values{"organisation": {"Bafflewick Ltd"}, "contacts": {"Alice | Commander | 123"}, "procedures": {"## First hour\nIsolate."}})
	if w.Code != http.StatusSeeOther || runtime.calls[len(runtime.calls)-1] != "emergency:Bafflewick Ltd" {
		t.Fatalf("emergency form: code=%d calls=%v", w.Code, runtime.calls)
	}
	w = postForm(t, s, "/organisation/emergency", url.Values{"organisation": {"Bafflewick Ltd"}, "procedures": {"no heading here"}})
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "## ") {
		t.Fatalf("procedure without heading must be refused in frame: code=%d", w.Code)
	}
	w = postForm(t, s, "/organisation/emergency", url.Values{"organisation": {"X"}, "parties": {"bank | Big Bank | 1"}})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown party type must be refused: code=%d", w.Code)
	}
	w = postForm(t, s, "/organisation/severity", url.Values{"levels": {"Low\nHigh"}})
	if w.Code != http.StatusSeeOther || runtime.calls[len(runtime.calls)-1] != "scale:Low|High" {
		t.Fatalf("scale form: code=%d calls=%v", w.Code, runtime.calls)
	}
}
