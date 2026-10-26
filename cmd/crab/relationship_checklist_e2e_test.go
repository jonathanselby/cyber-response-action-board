package main

import (
	"net/url"
	"strings"
	"testing"
)

// addMember makes person a member of incident: the sponsor proposes, the
// other Custodian confirms.
func (e *e2e) setMember(sponsor, custodian, incident, person string, active bool) {
	e.t.Helper()
	flag := "true"
	if !active {
		flag = "false"
	}
	e.post("/incident/"+incident+"/access/member", sponsor, url.Values{"human_id": {person}, "active": {flag}, "reason": {"test"}})
	e.approveAll(custodian)
}

// T-205 / T-206: a link needs authority over both incidents, reveals nothing
// to someone who can see only one, grants no access and is withdrawn by a
// new version.
func TestRelatedIncidentsDoNotLeak(t *testing.T) {
	e := newE2E(t)
	alice, bob := e.bootstrap()
	e.declare(alice, bob, "INC-A", "Supplier compromise", "Severity level 3")
	e.declare(bob, alice, "INC-B", "Ransomware on payroll", "Severity level 4")
	e.setMember(alice, bob, "INC-A", "bob", true)

	// alice cannot see INC-B: her proposal is refused exactly like an unknown incident.
	missing := firstError(e.refused("/incident/INC-A/relationship", alice, url.Values{"related_incident_id": {"INC-NOPE"}, "reason": {"x"}, "active": {"true"}}))
	hidden := firstError(e.refused("/incident/INC-A/relationship", alice, url.Values{"related_incident_id": {"INC-B"}, "reason": {"x"}, "active": {"true"}}))
	if missing != hidden {
		t.Fatalf("refusal reveals existence: %q vs %q", missing, hidden)
	}
	if strings.Contains(e.get("/incident/INC-A", alice), "INC-B") {
		t.Fatal("relate form offered an incident alice cannot see")
	}

	// bob sees both and proposes; alice must briefly see both to confirm.
	e.post("/incident/INC-A/relationship", bob, url.Values{"related_incident_id": {"INC-B"}, "reason": {"Same supplier VPN account"}, "active": {"true"}})
	if strings.Contains(e.get("/approvals", alice), "Same supplier VPN account") || strings.Contains(e.get("/approvals", alice), "INC-B") {
		t.Fatal("approval reveals INC-B to alice")
	}
	e.setMember(bob, alice, "INC-B", "alice", true)
	if !strings.Contains(e.get("/approvals", alice), "Relate INC-A to INC-B") {
		t.Fatal("confirmer with both incidents does not see the link proposal")
	}
	e.approveAll(alice)
	for _, view := range []struct{ who, inc, other string }{{alice, "INC-A", "INC-B"}, {bob, "INC-B", "INC-A"}} {
		board := e.get("/incident/"+view.inc, view.who)
		if !strings.Contains(board, "Same supplier VPN account") || !strings.Contains(board, `href="/incident/`+view.other+`"`) {
			t.Fatalf("%s does not show the link to %s", view.inc, view.other)
		}
	}

	// alice loses INC-B: the link and its reason vanish from INC-A for her.
	e.setMember(bob, alice, "INC-B", "alice", false)
	board := e.get("/incident/INC-A", alice)
	if strings.Contains(board, "INC-B") || strings.Contains(board, "Same supplier VPN account") || strings.Contains(board, "Ransomware on payroll") {
		t.Fatal("link leaks INC-B after access was withdrawn")
	}
	if w := e.do("GET", "/incident/INC-B", alice, nil); w.Code == 200 {
		t.Fatal("a link granted access to INC-B")
	}

	// Withdrawal is a new governed version.
	e.post("/incident/INC-A/relationship", bob, url.Values{"related_incident_id": {"INC-B"}, "reason": {"Not related after all"}, "active": {"false"}})
	e.setMember(bob, alice, "INC-B", "alice", true)
	if !strings.Contains(e.get("/approvals", alice), "Remove link between INC-A and INC-B") {
		t.Fatal("withdrawal not proposed for confirmation")
	}
	e.approveAll(alice)
	if strings.Contains(e.get("/incident/INC-A", bob), "Same supplier VPN account") {
		t.Fatal("withdrawn link still shown")
	}
}

// T-207: templates are governed; activation proposes one action per step
// with template provenance, grouped on Approvals and decided one by one.
func TestChecklistTemplateActivation(t *testing.T) {
	e := newE2E(t)
	alice, bob := e.bootstrap()
	e.declare(alice, bob, "INC-C", "Checklist test", "Severity level 2")
	e.setMember(alice, bob, "INC-C", "bob", true)
	e.post("/incident/INC-C/role", alice, url.Values{"role_name": {"Technical lead"}, "human_id": {"bob"}})
	e.approveAll(bob)

	e.post("/organisation/template", alice, url.Values{"name": {"Ransomware initial response"}, "source_ref": {"Runbook RB-07"}, "active": {"true"},
		"steps": {"Isolate affected hosts | P1 | Technical lead | 15\nCall the insurer hotline | P1\nPreserve logs | P2 | Technical lead"}})
	if n := e.approveAll(alice); n != 0 {
		t.Fatal("maker confirmed own template")
	}
	e.approveAll(bob)
	org := e.get("/organisation", alice)
	if !strings.Contains(org, "Ransomware initial response") || !strings.Contains(org, "ransomware-initial-response") {
		t.Fatal("confirmed template not listed")
	}
	e.post("/incident/INC-C/checklist", alice, url.Values{"template_id": {"ransomware-initial-response"}})
	approvals := e.get("/approvals", bob)
	if !strings.Contains(approvals, "Checklist ransomware-initial-response") || strings.Count(approvals, "New action:") < 3 {
		t.Fatal("activation not grouped or not one proposal per step")
	}
	if !strings.Contains(approvals, "Isolate affected hosts (owner bob)") || !strings.Contains(approvals, "Call the insurer hotline (owner alice)") {
		t.Fatal("owners not resolved from default role, else the activating person")
	}
	if n := e.approveAll(bob); n < 3 {
		t.Fatalf("decided %d checklist actions", n)
	}
	board := e.get("/incident/INC-C", alice)
	if !strings.Contains(board, "checklist ransomware-initial-response") || !strings.Contains(board, "Runbook RB-07") {
		t.Fatal("actions lost template provenance")
	}
	// Withdrawal is governed and stops further activation.
	e.post("/organisation/template", alice, url.Values{"template_id": {"ransomware-initial-response"}, "name": {"Ransomware initial response"}, "active": {"false"}})
	e.approveAll(bob)
	if body := e.refused("/incident/INC-C/checklist", alice, url.Values{"template_id": {"ransomware-initial-response"}}); !strings.Contains(body, "confirmed checklist template") {
		t.Fatal("withdrawn template still activates")
	}
}
