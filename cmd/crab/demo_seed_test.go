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
