package main

import (
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// T-202: the severity scale is confirmed at setup, changes need a second
// person, and responders can only choose from it.
func TestSeverityScaleIsGovernedAndEnforced(t *testing.T) {
	e := newE2E(t)
	alice, bob := e.bootstrap()
	org := e.get("/organisation", alice)
	for _, want := range []string{"Severity level 1", "Severity level 5", "Least severe", "Most severe", "Confirmed by bob"} {
		if !strings.Contains(org, want) {
			t.Fatalf("organisation page missing %q", want)
		}
	}
	if body := e.refused("/incident", alice, url.Values{"incident_id": {"INC-X"}, "mode": {"exercise"}, "title": {"T"}, "summary": {"S"}, "severity": {"Critical"}}); !strings.Contains(body, "severity from your organisation") {
		t.Fatalf("free-text severity was not refused: %s", firstError(body))
	}
	e.declare(alice, bob, "INC-S", "Scale test", "Severity level 2")

	e.post("/organisation/severity", alice, url.Values{"levels": {"Low\nMedium\nHigh\nCritical"}})
	if n := e.approveAll(alice); n != 0 {
		t.Fatal("maker confirmed their own scale change")
	}
	if !strings.Contains(e.get("/approvals", bob), "New severity scale") {
		t.Fatal("scale change not shown to the second person")
	}
	e.approveAll(bob)
	if !strings.Contains(e.get("/organisation", alice), "Critical") {
		t.Fatal("confirmed scale not current")
	}
	board := e.get("/incident/INC-S", alice)
	if !strings.Contains(board, "Severity level 2") || !strings.Contains(board, `<option value="Critical"`) {
		t.Fatal("existing severity must still display and the new scale must be offered")
	}
	e.refused("/incident/INC-S/severity", alice, url.Values{"severity": {"Severity level 5"}, "reason_evidence": {"old scale"}})
	e.post("/incident/INC-S/severity", alice, url.Values{"severity": {"High"}, "reason_evidence": {"spread"}})
}

// T-203 / T-204: emergency information is governed, fills the pack and brief,
// needs print to be printed, and makes the existing pack stale.
func TestEmergencyConfigurationFillsPaperAndStalesPack(t *testing.T) {
	e := newE2E(t)
	alice, bob := e.bootstrap()
	e.grantOutputs(alice, bob, "alice")
	e.declare(alice, bob, "INC-E", "Emergency test", "Severity level 3")

	w := e.do(http.MethodGet, "/pack.pdf", alice, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("pack: %d", w.Code)
	}
	if strings.Contains(e.get("/", alice), "Emergency Response Pack stale") {
		t.Fatal("fresh pack reported stale")
	}
	form := url.Values{
		"organisation": {"Bafflewick Ltd"},
		"contacts":     {"Alice Smith | Incident commander | +44 7700 900001 | +44 20 7946 0001"},
		"parties":      {"insurer | Example Mutual | +44 20 7946 0000 | claims@example.org"},
		"escalation":   {"Duty manager\nCIO"},
		"role_cards":   {"Incident commander | Runs the response | bob"},
		"procedures":   {"## Ransomware first hour\nIsolate affected hosts.\nCall the insurer."},
	}
	e.post("/organisation/emergency", alice, form)
	if n := e.approveAll(alice); n != 0 {
		t.Fatal("maker confirmed their own emergency information")
	}
	e.approveAll(bob)
	org := e.get("/organisation", alice)
	for _, want := range []string{"Bafflewick Ltd", "Alice Smith", "Example Mutual", "Duty manager", "Ransomware first hour"} {
		if !strings.Contains(org, want) {
			t.Fatalf("organisation page missing %q", want)
		}
	}
	if !strings.Contains(e.get("/", alice), "Emergency Response Pack stale") {
		t.Fatal("pack not stale after emergency information changed (T-204)")
	}

	pack := e.do(http.MethodGet, "/pack.pdf", alice, nil)
	brief := e.do(http.MethodGet, "/incident/INC-E/brief.pdf", alice, nil)
	if pack.Code != http.StatusOK || brief.Code != http.StatusOK {
		t.Fatalf("pack=%d brief=%d", pack.Code, brief.Code)
	}
	if strings.Contains(e.get("/", alice), "Emergency Response Pack stale") {
		t.Fatal("regenerated pack still stale")
	}
	if _, err := exec.LookPath("pdftotext"); err == nil {
		dir := t.TempDir()
		for name, body := range map[string]string{"pack": pack.Body.String(), "brief": brief.Body.String()} {
			pdf := filepath.Join(dir, name+".pdf")
			_ = os.WriteFile(pdf, []byte(body), 0o600)
			out, err := exec.Command("pdftotext", "-layout", pdf, "-").Output()
			if err != nil {
				t.Fatal(err)
			}
			text := string(out)
			for _, want := range []string{"Alice Smith", "Example Mutual"} {
				if !strings.Contains(text, want) {
					t.Fatalf("%s PDF missing %q", name, want)
				}
			}
			if name == "pack" && (!strings.Contains(text, "Bafflewick Ltd") || !strings.Contains(text, "Ransomware first hour") || !strings.Contains(text, "Duty manager")) {
				t.Fatal("pack PDF missing organisation sections")
			}
		}
	}
	// bob has no print permission: refused.
	if w := e.do(http.MethodGet, "/pack.pdf", bob, nil); w.Code != http.StatusForbidden {
		t.Fatalf("print without permission: %d", w.Code)
	}
}
