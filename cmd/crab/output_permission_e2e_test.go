package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// grantOutputs gives each named person print and download through the
// governed Custodian path: one proposes, the other confirms.
func (e *e2e) grantOutputs(maker, confirmer string, people ...string) {
	e.t.Helper()
	for _, person := range people {
		for _, action := range []string{"print", "download"} {
			e.post("/access/output", maker, url.Values{"human_id": {person}, "action": {action}, "active": {"true"}})
		}
	}
	e.approveAll(confirmer)
}

// Print and download are separate, governed permissions (AC-66): reading is
// not enough, the proposer cannot confirm their own grant, and a second
// Custodian's confirmation makes Preview and Download work.
func TestOutputPermissionIsGovernedAndEnablesPrint(t *testing.T) {
	e := newE2E(t)
	alice, bob := e.bootstrap()
	e.declare(alice, bob, "INC-P", "Print test", "Severity level 3")
	if w := e.do(http.MethodGet, "/incident/INC-P/brief.pdf?view=inline", alice, nil); w.Code != http.StatusForbidden {
		t.Fatalf("print before permission: code=%d", w.Code)
	}
	e.post("/access/output", alice, url.Values{"human_id": {"alice"}, "action": {"print"}, "active": {"true"}})
	if n := e.approveAll(alice); n != 0 {
		t.Fatal("proposer could confirm their own output permission")
	}
	if !strings.Contains(e.get("/approvals", bob), "Print or download permission") {
		t.Fatal("second Custodian does not see the output permission proposal")
	}
	e.approveAll(bob)
	w := e.do(http.MethodGet, "/incident/INC-P/brief.pdf?view=inline", alice, nil)
	if w.Code != http.StatusOK || !strings.HasPrefix(w.Body.String(), "%PDF") || !strings.HasPrefix(w.Header().Get("Content-Disposition"), "inline;") {
		t.Fatalf("preview after permission: code=%d headers=%v", w.Code, w.Header())
	}
	if !strings.Contains(e.get("/access", alice), "Print paper and PDFs") {
		t.Fatal("access admin does not list the confirmed permission")
	}
	if w := e.do(http.MethodGet, "/pack.pdf", bob, nil); w.Code != http.StatusForbidden {
		t.Fatalf("bob has no print permission yet: code=%d", w.Code)
	}
}
