package main

import (
	"context"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/jonathanselby/cyber-response-action-board/internal/data/postgres"
)

var (
	indicatorRow = regexp.MustCompile(`(?s)<tr><th scope="row"><span class="gas-muted">[A-Z]+</span><br><code class="crab-indicator">.*?</tr>`)
	observableID = regexp.MustCompile(`(OBS-[0-9a-f]+) · version`)
)

// indicatorRowFor returns the Indicators table row showing value.
func indicatorRowFor(page, value string) string {
	for _, row := range indicatorRow.FindAllString(page, -1) {
		if strings.Contains(row, `<code class="crab-indicator">`+value+`</code>`) {
			return row
		}
	}
	return ""
}

func (e *e2e) addMember(sponsor, confirmer, incidentID, human string) {
	e.t.Helper()
	e.post("/incident/"+incidentID+"/access/member", sponsor, url.Values{"human_id": {human}, "active": {"true"}, "reason": {"responder"}})
	e.approveAll(confirmer)
}

func (e *e2e) recordIndicator(cookie, incidentID, compartment, kind, value string) {
	e.t.Helper()
	e.post("/incident/"+incidentID+"/observables", cookie, url.Values{"compartment": {compartment}, "type": {kind}, "value": {value}, "status": {"suspected"}, "source": {"EDR alert"}})
}

// T-231 (AC-90): observables are recorded, versioned and compartment-scoped;
// values render as text only; a related incident appears only when the
// viewer may read both.
func TestObservables(t *testing.T) {
	e := newE2E(t)
	alice, bob := e.bootstrap()
	ctx := context.Background()
	e.declare(alice, bob, "INC-A", "Payroll outage", "Severity level 4")
	e.declare(bob, alice, "INC-B", "Supplier compromise", "Severity level 3")
	carol, _ := e.onboard(alice, bob, "carol", "Minor", false)
	dave, _ := e.onboard(alice, bob, "dave", "Minor", false)
	e.addMember(alice, bob, "INC-A", "carol")
	e.addMember(bob, alice, "INC-B", "carol")
	e.post("/incident/INC-B/access/controller", bob, url.Values{"human_id": {"bob"}, "compartment_name": {"FORENSICS"}, "active": {"true"}})
	e.approveAll(alice)
	e.post("/incident/INC-B/access/compartment-member", bob, url.Values{"human_id": {"bob"}, "compartment_name": {"FORENSICS"}, "active": {"true"}, "reason": {"lead"}})
	e.approveAll(alice)

	// Values are validated per type and stored in one normalised form.
	e.recordIndicator(alice, "INC-A", "GENERAL", "ip", "2001:DB8::1")
	e.recordIndicator(bob, "INC-B", "GENERAL", "ip", "2001:0db8:0:0:0:0:0:1")
	e.recordIndicator(alice, "INC-A", "GENERAL", "domain", "evil.example")
	e.recordIndicator(bob, "INC-B", "FORENSICS", "domain", "Evil.Example.")
	e.recordIndicator(alice, "INC-A", "GENERAL", "url", `https://Evil.Example/login?next=<script>`)
	for value, kind := range map[string]string{"999.1.1.1": "ip", "not a domain": "domain", "abc123": "hash"} {
		if msg := firstError(e.refused("/incident/INC-A/observables", alice, url.Values{"compartment": {"GENERAL"}, "type": {kind}, "value": {value}, "status": {"suspected"}, "source": {"x"}})); msg == "" {
			t.Errorf("invalid %s %q accepted", kind, value)
		}
	}

	// Rendered as text, never as a link.
	page := e.get("/incident/INC-A/evidence", alice)
	if strings.Contains(page, `href="https://evil.example`) || strings.Contains(page, "<script>") {
		t.Fatal("an indicator value was rendered as a link or markup")
	}
	if !strings.Contains(page, "https://evil.example/login?next=%3Cscript%3E") && !strings.Contains(page, "https://evil.example/login?next=&lt;script&gt;") {
		t.Fatal("URL indicator not shown as text")
	}

	// Related incidents: alice cannot read INC-B, so nothing about it shows.
	if strings.Contains(page, "INC-B") {
		t.Fatal("alice's evidence page reveals INC-B")
	}
	// carol can read INC-B GENERAL, so the IP shows INC-B; the domain's match
	// in INC-B is in FORENSICS, which carol cannot read, so it shows nothing.
	carolPage := e.get("/incident/INC-A/evidence", carol)
	if row := indicatorRowFor(carolPage, "2001:db8::1"); row == "" || !strings.Contains(row, "INC-B") {
		t.Fatalf("carol does not see the related incident for the shared IP: %q", row)
	}
	if row := indicatorRowFor(carolPage, "evil.example"); row == "" || strings.Contains(row, "INC-B") {
		t.Fatalf("carol sees a related incident through a compartment she cannot read: %q", row)
	}
	// bob, in FORENSICS but not INC-A, sees no related incident either way.
	if strings.Contains(e.get("/incident/INC-B/evidence?compartment=FORENSICS", bob), "INC-A") {
		t.Fatal("bob's evidence page reveals INC-A")
	}

	// Versioning: a status change is a new version; history is kept.
	m := observableID.FindStringSubmatch(indicatorRowFor(page, "2001:db8::1"))
	if m == nil {
		t.Fatal("observable ID not shown")
	}
	before := e.ledgerCount()
	if msg := e.refused("/observable/"+m[1]+"/status", dave, url.Values{"status": {"benign"}, "source": {"guess"}}); e.ledgerCount() != before {
		t.Fatalf("a non-member changed an observable: %s", firstError(msg))
	}
	e.post("/observable/"+m[1]+"/status", alice, url.Values{"status": {"confirmed"}, "source": {"Firewall logs"}, "note": {"C2 beacon"}, "back": {"/incident/INC-A/evidence"}})
	row := indicatorRowFor(e.get("/incident/INC-A/evidence", alice), "2001:db8::1")
	if !strings.Contains(row, "CONFIRMED") || !strings.Contains(row, "version 2") || !strings.Contains(row, "Firewall logs") {
		t.Fatalf("status change not shown as version 2: %q", row)
	}
	var versions int
	var firstStatus string
	if err := e.admin.QueryRow(ctx, `SELECT count(*), min(status) FILTER (WHERE version=1) FROM crab.observable_versions WHERE observable_id=$1`, m[1]).Scan(&versions, &firstStatus); err != nil {
		t.Fatal(err)
	}
	if versions != 2 || firstStatus != "suspected" {
		t.Fatalf("history: %d versions, first status %q", versions, firstStatus)
	}

	// Observables rebuild from the ledger.
	snap := e.projectionSnapshot()
	if _, err := e.mcp().kernel.Rebuild(ctx, postgres.RebuildProjections()...); err != nil {
		t.Fatal(err)
	}
	if e.projectionSnapshot() != snap {
		t.Fatal("projections did not rebuild identically")
	}
}
