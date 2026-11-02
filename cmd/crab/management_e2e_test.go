package main

import (
	"context"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

var managementPanel = regexp.MustCompile(`(?s)<section class="gas-panel gas-stack" aria-labelledby="management">.*?</section>`)

func (e *e2e) declareLive(maker, confirmer, id, title, severity string) {
	e.t.Helper()
	e.post("/incident", maker, url.Values{"incident_id": {id}, "mode": {"live"}, "title": {title}, "summary": {title + " summary"}, "severity": {severity}})
	if n := e.approveAll(confirmer); n == 0 {
		e.t.Fatalf("declaration %s had nothing to approve", id)
	}
}

// session signs a person in now: sessions end after eight hours, and this
// test moves the governed clock by days.
func (e *e2e) session(id string) string {
	e.t.Helper()
	switch id {
	case "alice":
		return e.login("alice", e2eAlicePassword)
	case "bob":
		return e.login("bob", e2eBobPassword)
	}
	return e.signIn(id, "")
}

// decideAfter proposes a decision in incidentID as maker, waits loop on the
// governed clock, then has the confirmer decide it.
func (e *e2e) decideAfter(maker, confirmer, incidentID, text string, loop time.Duration, outcome string) {
	e.t.Helper()
	e.post("/incident/"+incidentID+"/decision", e.session(maker), url.Values{"decision": {text}, "rationale": {"fixture"}})
	e.clock.Advance(loop)
	confirmerSession := e.session(confirmer)
	page := e.get("/approvals?incident="+incidentID, confirmerSession)
	for _, card := range approvalCard.FindAllString(page, -1) {
		if !strings.Contains(card, text) {
			continue
		}
		f := approvalForm.FindString(card)
		form := url.Values{}
		for _, m := range hiddenField.FindAllStringSubmatch(f, -1) {
			form.Set(m[1], htmlUnescape(m[2]))
		}
		if form.Get("record_id") == "" {
			continue
		}
		form.Set("outcome", outcome)
		e.post("/decide", confirmerSession, form)
		return
	}
	e.t.Fatalf("decision %q not waiting for %s", text, confirmer)
}

// T-232 (AC-91): the management summary counts only incidents the viewer can
// see, excludes exercises, and its median decision-loop time matches a
// hand-computed fixture.
func TestManagementSummary(t *testing.T) {
	t0 := time.Date(2026, 11, 2, 1, 0, 0, 0, time.UTC)
	clk := &testClock{now: t0}
	e := newE2EWithClock(t, clk)
	alice, bob := e.bootstrap()
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := e.admin.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}

	// alice can see L1, L2 and L3 (closed) and the exercise X; bob declared H,
	// which alice cannot see.
	e.declareLive(alice, bob, "INC-L1", "Payroll outage", "Severity level 4")
	e.declareLive(alice, bob, "INC-L2", "Phishing wave", "Severity level 2")
	e.declareLive(alice, bob, "INC-L3", "Old matter", "Severity level 1")
	e.declare(alice, bob, "INC-X", "Tabletop", "Severity level 4")
	e.declareLive(bob, alice, "INC-H", "Hidden matter", "Severity level 3")
	// carol, a Major, confirms decisions in every incident.
	e.onboard(alice, bob, "carol", "Major", false)
	for _, id := range []string{"INC-L1", "INC-L2", "INC-X"} {
		e.addMember(alice, bob, id, "carol")
	}
	e.addMember(bob, alice, "INC-H", "carol")

	// Actions: a critical one in L1, a P2 in L1, critical ones in H and X.
	carol := e.session("carol")
	for _, a := range []struct{ maker, incident, title, priority string }{
		{alice, "INC-L1", "Reset payroll admin credentials", "P1"},
		{alice, "INC-L1", "Review mailbox rules", "P2"},
		{bob, "INC-H", "Hidden critical action", "P1"},
		{alice, "INC-X", "Exercise critical action", "critical"},
	} {
		e.post("/incident/"+a.incident+"/action", a.maker, url.Values{"title": {a.title}, "owner": {"alice"}, "priority": {a.priority}, "source_ref": {"fixture"}})
		e.approveAll(carol)
	}
	var actions int
	if err := e.admin.QueryRow(ctx, `SELECT count(*) FROM crab.incident_actions`).Scan(&actions); err != nil || actions != 4 {
		t.Fatalf("fixture: %d confirmed actions (%v), want 4", actions, err)
	}
	// The browser has no due-date field yet; give every action a due time
	// that will have passed (projection fixture).
	exec(`UPDATE crab.incident_actions SET due_at=$1`, t0.Add(24*time.Hour))
	// L3 is closed (projection fixture; closure itself is tested in T-228).
	exec(`INSERT INTO crab.incident_recovery_state(incident_id, closure_record_id, closed_at) VALUES('INC-L3','CLOSE-FIXTURE',$1) ON CONFLICT (incident_id) DO UPDATE SET closure_record_id=EXCLUDED.closure_record_id, closed_at=EXCLUDED.closed_at`, t0)
	// Obligation clocks (projection fixture): overdue in L1 GENERAL counts;
	// waived, not yet due, LEGAL, hidden and exercise clocks do not.
	clock := func(id, incident, compartment, status string, deadline time.Time) {
		exec(`INSERT INTO crab.obligation_clocks(incident_id,clock_id,compartment,definition_id,name,lifecycle,status,trigger_at,source_record_id,source_field,original_definition_id,original_deadline,deadline,recipient,owner_text,authority_source,time_status,maker)
			VALUES($1,$2,$3,'DEF','Regulator '||$2,'firm',$4,$5,'SRC','awareness','DEF',$6,$6,'Regulator','alice','policy','trusted','alice')`, incident, id, compartment, status, t0, deadline)
	}
	clock("c-overdue", "INC-L1", "GENERAL", "active", t0.Add(48*time.Hour))
	clock("c-waived", "INC-L1", "GENERAL", "waived", t0.Add(48*time.Hour))
	clock("c-future", "INC-L2", "GENERAL", "active", t0.Add(30*24*time.Hour))
	clock("c-legal", "INC-L1", "LEGAL", "active", t0.Add(48*time.Hour))
	clock("c-hidden", "INC-H", "GENERAL", "active", t0.Add(48*time.Hour))
	clock("c-exercise", "INC-X", "GENERAL", "active", t0.Add(48*time.Hour))

	// A decision whose 5-hour loop ends before the 7-day window.
	e.decideAfter("alice", "carol", "INC-L1", "Old decision", 5*time.Hour, "confirmed")
	clk.Advance(8 * 24 * time.Hour)

	// In the window. Counted for alice: 10m, 30m and 2h, so the median is 30m.
	// Not counted: H's 50h (hidden), X's 40h (exercise), a rejected 1m.
	e.decideAfter("alice", "carol", "INC-L1", "Isolate payroll server", 10*time.Minute, "confirmed")
	e.decideAfter("alice", "carol", "INC-L2", "Block sender domain", 30*time.Minute, "confirmed")
	e.decideAfter("alice", "carol", "INC-L1", "Engage forensics", 2*time.Hour, "confirmed")
	e.decideAfter("alice", "carol", "INC-X", "Exercise decision", 40*time.Hour, "confirmed")
	e.decideAfter("alice", "carol", "INC-L2", "Rejected decision", time.Minute, "rejected")
	e.decideAfter("bob", "carol", "INC-H", "Hidden decision", 50*time.Hour, "confirmed")
	// 10m+30m+2h+40h+1m+50h is under four days: all six are in the window.

	panel := managementPanel.FindString(e.get("/", e.session("alice")))
	if panel == "" {
		t.Fatal("no management summary on alice's Command Summary")
	}
	for _, want := range []string{
		"<dt>Active incidents</dt><dd><strong>2</strong> — Severity level 2: 1, Severity level 4: 1</dd>",
		"<dt>Overdue critical actions</dt><dd><strong>1</strong></dd>",
		"<dt>Overdue obligations</dt><dd><strong>1</strong></dd>",
		"<strong>30m</strong> <span class=\"gas-muted\">over 3 confirmed decisions</span>",
		"Decision-loop time is the time from proposal to confirmation",
	} {
		if !strings.Contains(panel, want) {
			t.Errorf("alice's summary missing %q:\n%s", want, panel)
		}
	}
	for _, hidden := range []string{"Severity level 3", "INC-H", "Hidden"} {
		if strings.Contains(panel, hidden) {
			t.Errorf("alice's summary reveals %q", hidden)
		}
	}
	// bob sees only H: one active incident, one overdue critical action, one
	// overdue obligation and his own decision loop.
	bobPanel := managementPanel.FindString(e.get("/", e.session("bob")))
	for _, want := range []string{"<strong>1</strong> — Severity level 3: 1", "<strong>50h 00m</strong> <span class=\"gas-muted\">over 1 confirmed decision</span>"} {
		if !strings.Contains(bobPanel, want) {
			t.Errorf("bob's summary missing %q:\n%s", want, bobPanel)
		}
	}
}
