package main

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// surfaces is one person signed in four ways at once: two browser sessions,
// a JSON API bearer token and an MCP session token.
type surfaces struct {
	browserA, browserB, api, mcp string
}

func (e *e2e) surfacesFor(id, totpKey, first string) surfaces {
	return surfaces{browserA: first, browserB: e.signIn(id, totpKey), api: e.signIn(id, totpKey), mcp: e.signIn(id, totpKey)}
}

// seesIncident reports, per surface, whether the person can read an incident.
func (e *e2e) seesIncident(s surfaces, incidentID string) (a, b, api, mcp bool) {
	e.t.Helper()
	a = e.reaches(s.browserA, "/incident/"+incidentID)
	b = e.reaches(s.browserB, "/incident/"+incidentID)
	api = e.api(http.MethodGet, "/api/v1/incidents/"+incidentID, s.api, "").Code == http.StatusOK
	_, found, err := e.mcp().Incident(context.Background(), s.mcp, incidentID)
	mcp = err == nil && found
	return
}

func (e *e2e) requireSees(s surfaces, incidentID string, want bool, step string) {
	e.t.Helper()
	a, b, api, mcp := e.seesIncident(s, incidentID)
	for name, got := range map[string]bool{"browser A": a, "browser B": b, "API": api, "MCP": mcp} {
		if got != want {
			e.t.Errorf("%s: %s sees %s = %v, want %v", step, name, incidentID, got, want)
		}
	}
}

// T-223: permission changes take effect on the next request in every live
// session on every surface; an in-flight form is refused and writes nothing.
func TestAccessChangesReachEveryLiveSession(t *testing.T) {
	e := newE2E(t)
	alice, bob := e.bootstrap()
	e.declare(alice, bob, "INC-A", "Revocation fixture", "Severity level 3")
	ria, _ := e.onboard(alice, bob, "ria", "Major", false)
	e.post("/incident/INC-A/access/member", alice, url.Values{"human_id": {"ria"}, "active": {"true"}, "reason": {"responder"}})
	e.approveAll(bob)
	e.post("/incident/INC-A/access/controller", alice, url.Values{"human_id": {"alice"}, "compartment_name": {"FORENSICS"}, "active": {"true"}})
	e.approveAll(bob)
	e.post("/incident/INC-A/access/compartment-member", alice, url.Values{"human_id": {"ria"}, "compartment_name": {"FORENSICS"}, "active": {"true"}, "reason": {"imaging"}})
	e.approveAll(bob)
	s := e.surfacesFor("ria", "", ria)
	e.requireSees(s, "INC-A", true, "baseline")

	// Role reduction: Major → Minor stops proposing decisions in every session.
	e.post("/access/person/role", alice, url.Values{"human_id": {"ria"}, "role": {"Minor"}, "remove": {"false"}})
	e.approveAll(bob)
	before := e.ledgerCount()
	for _, cookie := range []string{s.browserA, s.browserB} {
		e.refused("/incident/INC-A/decision", cookie, url.Values{"decision": {"Isolate payments"}, "rationale": {"containment"}})
	}
	if w := e.api(http.MethodPost, "/api/v1/incidents", s.api, `{"incident_id":"INC-NEW","mode":"exercise","title":"t","summary":"s","severity":"Severity level 1"}`); w.Code == http.StatusCreated {
		t.Error("API declared an incident after role reduction")
	}
	if err := e.mcp().ProposeDecision(context.Background(), s.mcp, "INC-A", "Isolate payments", "containment"); err == nil {
		t.Error("MCP proposed a decision after role reduction")
	}
	if e.ledgerCount() != before {
		t.Fatal("a request after role reduction wrote to the ledger")
	}
	// Role increase takes effect too.
	e.post("/access/person/role", alice, url.Values{"human_id": {"ria"}, "role": {"Major"}, "remove": {"false"}})
	e.approveAll(bob)
	e.post("/incident/INC-A/decision", s.browserB, url.Values{"decision": {"Isolate payments"}, "rationale": {"containment"}})

	// Compartment removal.
	forensics := "/incident/INC-A/evidence?compartment=FORENSICS"
	if !e.reaches(s.browserA, forensics) {
		t.Fatal("baseline FORENSICS not reachable")
	}
	e.post("/incident/INC-A/access/compartment-member", alice, url.Values{"human_id": {"ria"}, "compartment_name": {"FORENSICS"}, "active": {"false"}})
	e.approveAll(bob)
	if e.reaches(s.browserA, forensics) || e.reaches(s.browserB, forensics) {
		t.Error("FORENSICS still reachable after read-out")
	}

	// Incident removal (explicit revocation).
	e.post("/incident/INC-A/access/member", alice, url.Values{"human_id": {"ria"}, "active": {"false"}})
	e.approveAll(bob)
	e.requireSees(s, "INC-A", false, "after incident removal")
	before = e.ledgerCount()
	e.refused("/incident/INC-A/fact", s.browserA, url.Values{"statement": {"in-flight form"}, "source": {"ria"}})
	if w := e.api(http.MethodPost, "/api/v1/incidents/INC-A/facts", s.api, `{"statement":"late","source":"api"}`); w.Code != http.StatusNotFound {
		t.Errorf("API fact after revocation code=%d", w.Code)
	}
	if e.ledgerCount() != before {
		t.Fatal("an in-flight request after revocation wrote to the ledger")
	}

	// Suspension ends every session; the old tokens stay dead after reactivation.
	e.post("/incident/INC-A/access/member", alice, url.Values{"human_id": {"ria"}, "active": {"true"}, "reason": {"back on shift"}})
	e.approveAll(bob)
	e.requireSees(s, "INC-A", true, "after re-grant")
	e.post("/access/person/status", alice, url.Values{"human_id": {"ria"}, "status": {"suspended"}, "reason": {"device lost"}})
	e.approveAll(bob)
	e.requireSees(s, "INC-A", false, "after suspension")
	e.post("/access/person/status", alice, url.Values{"human_id": {"ria"}, "status": {"active"}})
	e.approveAll(bob)
	e.requireSees(s, "INC-A", false, "old sessions after reactivation")
	fresh := e.signIn("ria", "")
	if !e.reaches(fresh, "/incident/INC-A") {
		t.Error("a fresh sign-in after reactivation cannot see INC-A")
	}

	// Disablement: every session ends and sign-in is refused for good.
	e.post("/access/person/status", alice, url.Values{"human_id": {"ria"}, "status": {"disabled"}, "reason": {"left the organisation"}})
	e.approveAll(bob)
	if e.reaches(fresh, "/incident/INC-A") {
		t.Error("session survived disablement")
	}
	if w := e.do(http.MethodPost, "/login", "", url.Values{"human_id": {"ria"}, "password": {"Onboard-ria-26!"}}); w.Code != http.StatusUnauthorized {
		t.Errorf("disabled person signed in: code=%d", w.Code)
	}
}

// T-226: a token kept after logout is refused on the browser, the API and MCP.
func TestStaleTokensAreRefusedEverywhere(t *testing.T) {
	e := newE2E(t)
	alice, bob := e.bootstrap()
	e.declare(alice, bob, "INC-S", "Stale token fixture", "Severity level 2")
	kept := e.login("alice", e2eAlicePassword)
	if !e.reaches(kept, "/incident/INC-S") {
		t.Fatal("baseline session cannot see INC-S")
	}
	if w := e.do(http.MethodPost, "/logout", kept, url.Values{}); w.Code != http.StatusSeeOther {
		t.Fatalf("logout code=%d", w.Code)
	}
	if e.reaches(kept, "/incident/INC-S") {
		t.Error("browser accepted a logged-out token")
	}
	if w := e.api(http.MethodGet, "/api/v1/incidents/INC-S", kept, ""); w.Code != http.StatusNotFound {
		t.Errorf("API accepted a logged-out token: code=%d", w.Code)
	}
	if _, found, err := e.mcp().Incident(context.Background(), kept, "INC-S"); err == nil && found {
		t.Error("MCP accepted a logged-out token")
	}
	if strings.Contains(e.do(http.MethodGet, "/", kept, nil).Body.String(), "Stale token fixture") {
		t.Error("home page rendered for a logged-out token")
	}
}

// T-222: an external person is marked EXTERNAL, in words, wherever their name
// appears beside incident work, and their access cannot outlive their account.
func TestExternalPeopleAreMarkedInWords(t *testing.T) {
	e := newE2E(t)
	alice, bob := e.bootstrap()
	e.declare(alice, bob, "INC-X", "External marking fixture", "Severity level 3")
	exa, totp := e.onboard(alice, bob, "exa", "Major", true)
	_ = totp
	e.post("/incident/INC-X/access/member", alice, url.Values{"human_id": {"exa"}, "active": {"true"}, "reason": {"counsel"}})
	e.approveAll(bob)
	access := e.get("/incident/INC-X/access", alice)
	if !strings.Contains(access, "exa") || !strings.Contains(access, "EXTERNAL") {
		t.Error("incident Access page does not mark the external member")
	}
	// No end was given: the access ends with the account, never later.
	row := regexp.MustCompile(`(?s)<tr><th scope="row">exa.*?</tr>`).FindString(access)
	if row == "" || strings.Contains(row, "No expiry") {
		t.Errorf("external membership shown without an end: %q", row)
	}
	// Bob is not on INC-X; exa, a Major on the incident, confirms the role.
	e.post("/incident/INC-X/role", alice, url.Values{"role_name": {"Legal liaison"}, "human_id": {"exa"}})
	e.approveAll(exa)
	board := e.get("/incident/INC-X", alice)
	if !strings.Contains(board, "Legal liaison") || !strings.Contains(board, "EXTERNAL") {
		t.Error("Board does not mark the external role holder")
	}
	e.post("/incident/INC-X/decision", exa, url.Values{"decision": {"Preserve privilege"}, "rationale": {"counsel advice"}})
	if approvals := e.get("/approvals?incident=INC-X", alice); !strings.Contains(approvals, "Proposed by <strong>Exa</strong> <span class=\"gas-badge gas-badge--warning\">EXTERNAL</span> (Major)") {
		t.Error("Approvals does not mark the external maker")
	}
}
