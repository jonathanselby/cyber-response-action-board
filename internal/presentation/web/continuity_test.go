package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestIntakeSurfaceSupportsPhoneCapture(t *testing.T) {
	s := withRuntime(t, &fakeRuntime{bootstrapped: true})
	w := get(t, s, "/intake")
	body := w.Body.String()
	if w.Code != http.StatusOK || !strings.Contains(body, "First reports") || !strings.Contains(body, `capture="environment"`) || !strings.Contains(body, "Phone report") {
		t.Fatalf("code=%d body=%q", w.Code, body)
	}
	// Phone-first: the capture form precedes the list of reports (AC-68, T-187).
	if strings.Index(body, "Record first report") > strings.Index(body, "Phone report") {
		t.Fatal("capture form must come before the reports list")
	}
	for _, want := range []string{`accept="audio/*"`, `accept="video/*"`, `type="datetime-local"`, "AWAITING TRIAGE"} {
		if !strings.Contains(body, want) {
			t.Fatalf("intake missing %q", want)
		}
	}
}

// The former Operations screen is now part of the Board: scope, severity
// history, resource requests and governed severity change all live there.
func TestOperationsSurfaceShowsContinuityAndGovernedSeverity(t *testing.T) {
	runtime := &fakeRuntime{bootstrapped: true}
	s := withRuntime(t, runtime)
	old := get(t, s, "/incident/INC-1/operations")
	if old.Code != http.StatusMovedPermanently || old.Header().Get("Location") != "/incident/INC-1" {
		t.Fatalf("old operations route code=%d location=%q", old.Code, old.Header().Get("Location"))
	}
	w := get(t, s, "/incident/INC-1")
	body := w.Body.String()
	for _, want := range []string{"Payroll unavailable", "Propose severity change", "IR surge", "Takes effect when another eligible person confirms"} {
		if !strings.Contains(body, want) {
			t.Fatalf("board missing %q: code=%d", want, w.Code)
		}
	}
	post := postForm(t, s, "/incident/INC-1/severity", url.Values{"severity": {"Critical"}, "reason_evidence": {"Spread confirmed"}})
	if post.Code != http.StatusSeeOther || runtime.severity != "Critical" {
		t.Fatalf("code=%d severity=%q", post.Code, runtime.severity)
	}
}

func TestOperatorNavigationExposesContinuitySurfaces(t *testing.T) {
	s := withRuntime(t, &fakeRuntime{bootstrapped: true})
	home := get(t, s, "/").Body.String()
	for _, want := range []string{`href="/intake"`, `href="/approvals"`, `href="/readiness"`} {
		if !strings.Contains(home, want) {
			t.Fatalf("home navigation missing %s", want)
		}
	}
	incident := get(t, s, "/incident/INC-1").Body.String()
	for _, want := range []string{`href="/incident/INC-1/evidence"`, `href="/incident/INC-1/communications"`, `href="/incident/INC-1/access"`, `href="/incident/INC-1/close"`, `aria-current="page"`} {
		if !strings.Contains(incident, want) {
			t.Fatalf("incident navigation missing %s", want)
		}
	}
}
