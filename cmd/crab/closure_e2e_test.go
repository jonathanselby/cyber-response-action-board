package main

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/data/postgres"
)

// T-228 and T-229: closure is refused while anything is outstanding and says
// what; one person cannot close; a closed incident is read-only; reopening
// needs a reason and a second Major, is never self-approved, keeps the closure
// in history, makes the incident writable, and requires a fresh stand-down
// before it can close again. Access that expired while closed stays expired.
func TestClosureAndReopening(t *testing.T) {
	start := time.Date(2026, 11, 9, 1, 0, 0, 0, time.UTC)
	clk := &testClock{now: start}
	e := newE2EWithClock(t, clk)
	alice, bob := e.bootstrap()
	e.declare(alice, bob, "INC-C", "Closure fixture", "Severity level 2")
	nina, _ := e.onboard(alice, bob, "nina", "Major", false)
	dave, _ := e.onboard(alice, bob, "dave", "Minor", false)
	e.post("/incident/INC-C/access/member", alice, url.Values{"human_id": {"nina"}, "active": {"true"}, "reason": {"second Major"}})
	e.approveAll(bob)
	e.post("/incident/INC-C/access/member", alice, url.Values{"human_id": {"dave"}, "active": {"true"}, "expires_at": {at(start.Add(2 * time.Hour))}, "reason": {"short engagement"}})
	e.approveAll(bob)
	nina, dave = e.signIn("nina", ""), e.signIn("dave", "")

	// Recovery milestones, each confirmed by the other Major.
	e.post("/incident/INC-C/close/stand-down", alice, url.Values{"reason": {"service restored"}})
	e.approveAll(nina)
	e.post("/incident/INC-C/close/pir", alice, url.Values{"executive_summary": {"Contained in four hours"}, "lessons": {"Rehearse isolation"}})
	e.approveAll(nina)
	e.post("/incident/INC-C/close/retention", alice, url.Values{"retention_class": {"incident-7y"}, "handoff_ref": {"Records team ticket R-1"}})
	e.approveAll(nina)

	// A pending decision blocks closure, by name, on the page and on submit.
	e.post("/incident/INC-C/decision", alice, url.Values{"decision": {"Rotate all service keys"}, "rationale": {"hygiene"}})
	if page := e.get("/incident/INC-C/close", alice); !strings.Contains(page, "1 pending decision proposal") {
		t.Error("Close-out does not name the pending decision as a blocker")
	}
	closure := url.Values{"reason": {"done"}, "shareable_summary": {"Payments outage resolved"}}
	if body := e.refused("/incident/INC-C/close/closure", alice, closure); !strings.Contains(body, "1 pending decision proposal") {
		t.Errorf("closure refusal does not name the blocker: %s", firstError(body))
	}
	e.approveAll(nina)
	if page := e.get("/incident/INC-C/close", alice); !strings.Contains(page, "Nothing outstanding") {
		t.Error("Close-out still shows blockers after the decision was confirmed")
	}

	// One person cannot close: the maker is never offered the decision.
	e.post("/incident/INC-C/close/closure", alice, closure)
	if n := e.approveAll(alice); n != 0 {
		t.Fatal("the maker confirmed their own closure")
	}
	e.approveAll(nina)
	if !strings.Contains(e.get("/incident/INC-C/close", alice), "this incident is now read-only") {
		t.Fatal("incident did not close")
	}
	before := e.ledgerCount()
	e.refused("/incident/INC-C/fact", alice, url.Values{"statement": {"after closure"}, "source": {"alice"}})
	if e.ledgerCount() != before {
		t.Fatal("a closed incident accepted a write")
	}

	// dave's access expires while the incident is closed.
	clk.Advance(3 * time.Hour)
	alice, nina = e.login("alice", e2eAlicePassword), e.signIn("nina", "")

	// Reopening: a reason is required, and it is never self-approved.
	e.refused("/incident/INC-C/close/reopen", alice, url.Values{"reason": {""}})
	e.post("/incident/INC-C/close/reopen", alice, url.Values{"reason": {"Attacker persistence found on a restored host"}})
	// No window can ever cover a reopening (domain test
	// TestNoWindowCoversReopening); here the maker is simply not offered it.
	if n := e.approveAll(alice); n != 0 {
		t.Error("the maker confirmed their own reopening")
	}
	if !strings.Contains(e.get("/approvals?incident=INC-C", nina), "Reopen the incident: Attacker persistence found") {
		t.Error("Approvals does not word the reopening for the confirmer")
	}
	e.approveAll(nina)

	page := e.get("/incident/INC-C/close", alice)
	for _, want := range []string{"REOPENED", "Attacker persistence found", "is superseded and kept in history", "stand-down not confirmed"} {
		if !strings.Contains(page, want) {
			t.Errorf("after reopening, Close-out missing %q", want)
		}
	}
	if strings.Contains(page, "this incident is now read-only") {
		t.Error("reopened incident still shown as closed")
	}
	e.post("/incident/INC-C/fact", alice, url.Values{"statement": {"Persistence mechanism removed"}, "source": {"alice"}})
	if e.reaches(dave, "/incident/INC-C") || e.reaches(e.signIn("dave", ""), "/incident/INC-C") {
		t.Error("access that expired while closed came back on reopening")
	}

	// The record holds: the ledger verifies and projections rebuild identically.
	rt := e.mcp()
	if result, err := rt.kernel.Verify(context.Background()); err != nil || !result.Valid {
		t.Fatalf("ledger verification failed: %+v %v", result, err)
	}
	snapshot := e.projectionSnapshot()
	if _, err := rt.kernel.Rebuild(context.Background(), postgres.RebuildProjections()...); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if e.projectionSnapshot() != snapshot {
		t.Fatal("projections differ after rebuild")
	}
	if w := e.do(http.MethodGet, "/incident/INC-C", alice, nil); w.Code != http.StatusOK {
		t.Fatalf("reopened incident not readable after rebuild: %d", w.Code)
	}
}
