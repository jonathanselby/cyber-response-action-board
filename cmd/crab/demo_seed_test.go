package main

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// postMultipart submits a multipart form (first reports, evidence).
func (e *e2e) postMultipart(path, cookie string, fields map[string]string, files map[string][2]string) {
	e.t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	for field, file := range files {
		part, err := w.CreateFormFile(field, file[0])
		if err != nil {
			e.t.Fatal(err)
		}
		_, _ = part.Write([]byte(file[1]))
	}
	_ = w.Close()
	r := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080"+path, &buf)
	r.RemoteAddr = "127.0.0.1:50000"
	r.Header.Set("Content-Type", w.FormDataContentType())
	r.AddCookie(&http.Cookie{Name: "crab_session", Value: cookie})
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, r)
	if rec.Code != http.StatusSeeOther {
		e.t.Fatalf("POST %s code=%d error=%s", path, rec.Code, firstError(rec.Body.String()))
	}
}

// seedDemo builds a representative Bafflewick picture through the browser
// routes: two incidents, facts, unknowns, roles, actions, a clock, evidence,
// a first report and pending approvals. It is used by the visual capture and
// by UAT preparation; it bypasses no control.
func seedDemo(t *testing.T, e *e2e) (alice, bob string) {
	t.Helper()
	alice, bob = e.bootstrap()
	e.grantOutputs(alice, bob, "alice")
	e.grantOutputs(bob, alice, "bob")
	e.declare(alice, bob, "INC-014", "Payment outage", "Severity level 3")
	e.declare(alice, bob, "INC-011", "Credential compromise", "Severity level 2")

	e.post("/organisation/emergency", alice, url.Values{
		"organisation": {"Bafflewick Ltd"},
		"contacts":     {"Alice Smith | Incident commander | +44 7700 900001 | +44 20 7946 0001\nBob Jones | Technical lead | +44 7700 900002 | +44 20 7946 0002\nCarol White | Communications lead | +44 7700 900003"},
		"parties":      {"insurer | Example Mutual cyber line | +44 20 7946 0000 | claims@example.org | Policy CY-1234\nregulator | Information Commissioner's Office | 0303 123 1113 | | 72-hour reporting\ncounsel | Smith & Partners LLP | +44 20 7946 0100"},
		"escalation":   {"Duty manager\nHead of IT operations\nChief Information Officer\nChief Executive"},
		"role_cards":   {"Incident commander | Owns the response, chairs calls, approves containment | Bob Jones\nCommunications lead | Drafts and issues all internal and external messages | Alice Smith"},
		"procedures":   {"## Ransomware: first hour\nIsolate affected hosts from the network.\nDo not power off encrypted machines.\nCall the insurer hotline before engaging any third party.\n\n## Suspected data breach\nRecord the awareness time.\nStart the regulator clock in CRAB or on paper."},
	})
	e.approveAll(bob)

	inc := "/incident/INC-014"
	// bob joins INC-014 so the second person can confirm its proposals.
	e.post(inc+"/access/member", alice, url.Values{"human_id": {"bob"}, "active": {"true"}, "reason": {"Technical lead for the response"}})
	e.approveAll(bob)
	e.post(inc+"/fact", alice, url.Values{"statement": {"Payroll file share FS-02 encrypted at 07:42 UTC"}, "source": {"EDR console"}})
	e.post(inc+"/fact", alice, url.Values{"statement": {"Card payments failing on the web portal"}, "source": {"Service desk"}})
	e.post(inc+"/uncertainty", alice, url.Values{"kind": {"unknown"}, "statement": {"Initial access vector"}})
	e.post(inc+"/uncertainty", alice, url.Values{"kind": {"hypothesis"}, "statement": {"Compromised supplier VPN account"}})
	e.post(inc+"/role", alice, url.Values{"role_name": {"Incident commander"}, "human_id": {"alice"}})
	e.post(inc+"/role", alice, url.Values{"role_name": {"Technical lead"}, "human_id": {"bob"}})
	e.post(inc+"/action", alice, url.Values{"title": {"Verify last good backup of FS-02"}, "owner": {"bob"}, "priority": {"P1"}, "source_ref": {"Runbook RB-07 step 3"}})
	e.post(inc+"/action", alice, url.Values{"title": {"Notify cyber insurer hotline"}, "owner": {"alice"}, "priority": {"P2"}, "source_ref": {"Insurance policy section 4"}})
	e.post(inc+"/decision", alice, url.Values{"decision": {"Isolate payroll VLAN from the corporate network"}, "rationale": {"Contain spread while backups are verified"}})
	e.approveAll(bob)

	e.post("/obligations/definition", alice, url.Values{"name": {"Regulator notification"}, "trigger_field": {"awareness"}, "duration": {"72"}, "unit": {"elapsed_hours"}, "timezone": {"Europe/London"}, "applicability": {"Personal data affected"}, "recipient": {"Regulator"}, "owner": {"legal"}, "authority": {"Statute s.33"}, "adjustment": {"none"}})
	e.approveAll(bob)

	e.post(inc+"/scope", alice, url.Values{"affected_services": {"Payments\nPayroll"}, "impact": {"Customers cannot pay by card"}, "confidence": {"medium"}, "reason_evidence": {"Service desk tickets"}})
	e.post(inc+"/severity", alice, url.Values{"severity": {"Severity level 4"}, "reason_evidence": {"Customer-facing outage"}})
	e.post(inc+"/decision", alice, url.Values{"decision": {"Engage external forensics firm"}, "rationale": {"Retainer in place"}})

	// Organisation checklist, a related incident and an activated checklist.
	e.post("/organisation/template", alice, url.Values{"name": {"Ransomware initial response"}, "source_ref": {"Runbook RB-07"}, "active": {"true"},
		"steps": {"Isolate affected hosts | P1 | Technical lead | 15\nCall the insurer hotline | P1 | Incident commander | 30\nPreserve logs and memory images | P2 | Technical lead | 60\nBrief the executive team | P2 | Incident commander | 120"}})
	e.post("/organisation/template", alice, url.Values{"name": {"Suspected data breach"}, "source_ref": {"Privacy policy section 9"}, "active": {"true"},
		"steps": {"Record the awareness time | P1 | Incident commander\nAssess personal data involved | P1\nDecide on regulator notification | P1 | Incident commander | 1440"}})
	e.approveAll(bob)
	e.setMember(alice, bob, "INC-011", "bob", true)
	e.post(inc+"/relationship", alice, url.Values{"related_incident_id": {"INC-011"}, "reason": {"Same compromised supplier VPN account seen in both"}, "active": {"true"}})
	e.approveAll(bob)
	e.post(inc+"/checklist", alice, url.Values{"template_id": {"suspected-data-breach"}})

	e.postMultipart("/intake", alice, map[string]string{
		"reporter": "Service desk", "contact_route": "ext 4400", "description": "Staff report ransom note on shared drive",
		"affected": "Finance share", "immediate_impact": "Finance team cannot work", "source_channel": "phone",
		"received_at": time.Now().UTC().Format("2006-01-02T15:04"),
	}, nil)
	e.postMultipart(inc+"/evidence", alice, map[string]string{
		"description": "Ransom note screenshot", "collector": "bob", "source": "FS-02 console", "current_handler": "bob", "compartment": "GENERAL",
	}, map[string][2]string{"evidence": {"ransom-note.txt", "YOUR FILES ARE ENCRYPTED"}})
	return alice, bob
}

func TestDemoSeedRuns(t *testing.T) {
	e := newE2E(t)
	alice, _ := seedDemo(t, e)
	board := e.get("/incident/INC-014", alice)
	for _, want := range []string{"Payroll file share FS-02", "Initial access vector", "Incident commander", "Verify last good backup"} {
		if !strings.Contains(board, want) {
			t.Fatalf("board missing %q", want)
		}
	}
	if strings.Contains(board, "No operational roles assigned") {
		t.Fatal("seeded roles were not confirmed by the second person")
	}
}
