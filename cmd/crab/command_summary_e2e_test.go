package main

import (
	"net/url"
	"strings"
	"testing"
)

// T-198: the Command Summary names only incidents the user may see, and its
// counts come from the same scoped listings.
func TestCommandSummaryShowsOnlyAuthorisedWork(t *testing.T) {
	e := newE2E(t)
	alice, bob := e.bootstrap()
	e.declare(alice, bob, "INC-A", "Payroll outage", "High")
	e.declare(bob, alice, "INC-B", "Supplier compromise", "High")

	// A pending decision inside INC-B that only bob's side can see.
	e.post("/incident/INC-B/decision", bob, url.Values{"decision": {"Isolate supplier VPN"}, "rationale": {"Contain"}})

	aliceHome := e.get("/", alice)
	for _, want := range []string{"Command Summary", "What needs your attention now.", "INC-A", "Payroll outage", "Only items you are authorised to see are shown."} {
		if !strings.Contains(aliceHome, want) {
			t.Fatalf("alice summary missing %q", want)
		}
	}
	for _, hidden := range []string{"INC-B", "Supplier compromise", "Isolate supplier VPN"} {
		if strings.Contains(aliceHome, hidden) {
			t.Fatalf("alice summary reveals %q", hidden)
		}
	}
	if approvals := e.get("/approvals", alice); strings.Contains(approvals, "INC-B") || strings.Contains(approvals, "Isolate supplier VPN") {
		t.Fatal("alice approvals reveal INC-B")
	}
	if !strings.Contains(e.get("/", bob), "Supplier compromise") {
		t.Fatal("bob should see his own incident")
	}
}
