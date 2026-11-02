package main

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// T-236 (R-22): every approval says what, current and proposed, who proposed
// it (with EXTERNAL and role), the reason, and what happens on confirm and on
// reject; access approvals read in operator language.
func TestApprovalComprehension(t *testing.T) {
	clk := &testClock{now: time.Date(2026, 11, 2, 2, 17, 0, 0, time.UTC)}
	e := newE2EWithClock(t, clk)
	alice, bob := e.bootstrap()
	e.declare(alice, bob, "INC-004", "Payroll ransomware", "Severity level 4")
	e.setMember(alice, bob, "INC-004", "bob", true)
	e.post("/access/person", alice, url.Values{"human_id": {"jane"}, "display_name": {"Jane Doe"}, "organisation": {"Forensic Co"}, "role": {"Minor"}, "external": {"true"}, "account_expires_at": {at(e.now().Add(7 * 24 * time.Hour))}})
	e.approveAll(bob)
	e.post("/incident/INC-004/access/member", alice, url.Values{"human_id": {"jane"}, "active": {"true"}, "expires_at": {at(e.now().Add(6 * time.Hour))}, "reason": {"Forensic imaging"}})
	e.post("/incident/INC-004/severity", alice, url.Values{"severity": {"Severity level 3"}, "reason_evidence": {"Contained"}})
	e.post("/incident/INC-004/decision", alice, url.Values{"decision": {"Engage forensics"}, "rationale": {"Retainer"}})

	page := htmlUnescape(e.get("/approvals", bob))
	if want := "Give Jane Doe (EXTERNAL, Forensic Co, Minor) access to INC-004 until 2 Nov 2026 08:17 UTC"; !strings.Contains(page, want) {
		t.Errorf("access approval not in operator language; want %q", want)
	}
	cards := approvalCard.FindAllString(page, -1)
	if len(cards) < 3 {
		t.Fatalf("expected at least 3 approvals, found %d", len(cards))
	}
	for _, card := range cards {
		for _, want := range []string{"Proposed by <strong>", "If you confirm:", "If you reject:"} {
			if !strings.Contains(card, want) {
				t.Errorf("approval card missing %q:\n%s", want, card)
			}
		}
	}
	if !strings.Contains(page, "Severity stays as it is.") || !strings.Contains(page, "Their access stays as it is.") {
		t.Error("reject consequences are not specific to the change")
	}
	if !strings.Contains(page, "<td>Severity level 4</td>") {
		t.Error("severity approval does not show the current value")
	}
}

// F-14 (T-237): a form submitted after the session ended records nothing,
// says so, and gives back what was typed.
func TestExpiredSessionGivesBackInput(t *testing.T) {
	e := newE2E(t)
	alice, bob := e.bootstrap()
	e.declare(alice, bob, "INC-E", "Expiry", "Severity level 2")
	if _, err := e.admin.Exec(context.Background(), `DELETE FROM crab.sessions WHERE human_id='alice'`); err != nil {
		t.Fatal(err)
	}
	before := e.ledgerCount()
	w := e.do(http.MethodPost, "/incident/INC-E/fact", alice, url.Values{"_form": {"record-fact"}, "statement": {"Typed while the session ended"}, "source": {"EDR"}})
	body := htmlUnescape(w.Body.String())
	if w.Code != http.StatusUnauthorized || !strings.Contains(body, "nothing was recorded") || !strings.Contains(body, "Typed while the session ended") {
		t.Fatalf("expired session: code=%d, body does not say so or give back the text", w.Code)
	}
	if e.ledgerCount() != before {
		t.Fatal("an expired session wrote to the ledger")
	}
	if strings.Contains(body, "record-fact") {
		t.Fatal("routing fields shown back")
	}
}
