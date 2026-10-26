package main

import (
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

var selfForm = regexp.MustCompile(`(?s)<form[^>]*action="/decide">\s*<input type="hidden" name="_form" value="decide">\s*<input type="hidden" name="self" value="true">.*?</form>`)

// selfDecide submits the self-approval form for the first proposal offered.
func (e *e2e) selfDecide(cookie, reason string) *http.Response {
	e.t.Helper()
	page := e.get("/approvals", cookie)
	f := selfForm.FindString(page)
	if f == "" {
		return nil
	}
	form := url.Values{}
	for _, m := range hiddenField.FindAllStringSubmatch(f, -1) {
		form.Set(m[1], htmlUnescape(m[2]))
	}
	form.Set("self_reason", reason)
	form.Set("outcome", "confirmed")
	return e.do(http.MethodPost, "/decide", cookie, form).Result()
}

func windowForm(id string, from, to time.Time, humans string, kinds ...string) url.Values {
	return url.Values{"window_id": {id}, "active": {"true"}, "starts_at": {from.UTC().Format("2006-01-02T15:04")}, "ends_at": {to.UTC().Format("2006-01-02T15:04")}, "humans": {humans}, "kinds": kinds, "reason": {"Only one Major on call overnight"}}
}

// T-208 / T-209: windows are dual-controlled and bounded; inside one, a named
// Major may self-approve a covered kind with a reason, and it is flagged.
func TestSinglePersonWindow(t *testing.T) {
	e := newE2E(t)
	alice, bob := e.bootstrap()
	e.grantOutputs(alice, bob, "alice")
	e.declare(alice, bob, "INC-W", "Window test", "Severity level 2")
	now := time.Now().UTC()

	// T-208: shape and policy refusals.
	for name, form := range map[string]url.Values{
		"too long":      windowForm("w-long", now, now.Add(25*time.Hour), "alice", "incident.severity_change"),
		"excluded kind": windowForm("w-access", now, now.Add(time.Hour), "alice", "security.incident_membership"),
		"no people":     windowForm("w-empty", now, now.Add(time.Hour), "", "incident.severity_change"),
		"non-Major":     windowForm("w-nobody", now, now.Add(time.Hour), "mallory", "incident.severity_change"),
		"in the past":   windowForm("w-past", now.Add(-2*time.Hour), now.Add(-time.Hour), "alice", "incident.severity_change"),
	} {
		if w := e.do(http.MethodPost, "/organisation/window", alice, form); w.Code == http.StatusSeeOther {
			t.Fatalf("%s window accepted", name)
		}
	}
	// An upcoming window does not permit anything yet.
	e.post("/organisation/window", alice, windowForm("w-later", now.Add(2*time.Hour), now.Add(3*time.Hour), "alice", "incident.severity_change"))
	if n := e.approveAll(alice); n != 0 {
		t.Fatal("maker confirmed their own window (T-208)")
	}
	e.approveAll(bob)

	e.post("/incident/INC-W/severity", alice, url.Values{"severity": {"Severity level 4"}, "reason_evidence": {"spread to payments"}})
	if resp := e.selfDecide(alice, "night shift"); resp != nil {
		t.Fatal("self-approval offered before the window opens")
	}
	// Forged self-approval without a window is refused by the application.
	if body := firstError(e.refused("/decide", alice, url.Values{"self": {"true"}, "self_reason": {"x"}, "kind": {"declaration"}, "scope_id": {"INC-W"}, "record_type": {"incident.declaration"}, "record_id": {"x"}, "version": {"y"}, "outcome": {"confirmed"}})); !strings.Contains(body, "single-person window") {
		t.Fatalf("self-approval of an excluded kind not refused clearly: %s", body)
	}

	// Open window now for alice covering severity changes only.
	e.post("/organisation/window", alice, windowForm("w-now", now, now.Add(4*time.Hour), "alice", "incident.severity_change", "incident.decision"))
	e.approveAll(bob)
	if !strings.Contains(e.get("/", alice), "Single-person window open") {
		t.Fatal("open window not stated on Command Summary and status line")
	}
	// Reason is required.
	if w := e.do(http.MethodPost, "/decide", alice, url.Values{"self": {"true"}, "kind": {"severity"}, "scope_id": {"INC-W"}, "record_type": {"incident.severity_change"}, "record_id": {"x"}, "version": {"y"}, "outcome": {"confirmed"}}); w.Code == http.StatusSeeOther {
		t.Fatal("self-approval without a reason accepted")
	}
	// Covered kinds: the severity change and a decision.
	e.post("/incident/INC-W/decision", alice, url.Values{"decision": {"Rebuild payroll servers"}, "rationale": {"x"}})
	for i := 0; i < 2; i++ {
		resp := e.selfDecide(alice, "Only Major on call overnight; bob unreachable")
		if resp == nil || resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("covered self-approval %d failed: %v", i, resp)
		}
	}
	// Uncovered kind: an action proposal stays waiting.
	e.post("/incident/INC-W/action", alice, url.Values{"title": {"Call insurer"}, "owner": {"alice"}, "priority": {"P1"}, "source_ref": {"policy"}})
	if resp := e.selfDecide(alice, "again"); resp != nil {
		t.Fatal("an uncovered kind (action) was offered for self-approval")
	}
	// bob is not named: he cannot self-approve his own severity proposal.
	e.setMember(alice, bob, "INC-W", "bob", true)
	e.post("/incident/INC-W/severity", bob, url.Values{"severity": {"Severity level 5"}, "reason_evidence": {"x"}})
	if resp := e.selfDecide(bob, "x"); resp != nil {
		t.Fatal("a Major not named in the window was offered self-approval")
	}

	// T-209 flags: Board, timeline, Approvals, Command Summary, Brief.
	board := e.get("/incident/INC-W", alice)
	if !strings.Contains(board, "Severity level 4") || !strings.Contains(board, "SELF-APPROVAL") {
		t.Fatal("self-approved severity not flagged on the Board")
	}
	if a := e.get("/approvals", alice); !strings.Contains(a, "Self-approvals in the last 24 hours") || !strings.Contains(a, "SELF-APPROVAL") {
		t.Fatal("self-approval missing from the single-approval dashboard")
	}
	if !strings.Contains(e.get("/", alice), "self-approved decision") {
		t.Fatal("self-approval missing from Command Summary")
	}
	if _, err := exec.LookPath("pdftotext"); err == nil {
		w := e.do(http.MethodGet, "/incident/INC-W/brief.pdf", alice, nil)
		path := filepath.Join(t.TempDir(), "brief.pdf")
		_ = os.WriteFile(path, w.Body.Bytes(), 0o600)
		out, err := exec.Command("pdftotext", "-layout", path, "-").Output()
		if err != nil || !strings.Contains(string(out), "SELF-APPROVED") || !strings.Contains(string(out), "Rebuild payroll servers") {
			t.Fatalf("Brief does not flag the self-approved decision: %v", err)
		}
	}
}
