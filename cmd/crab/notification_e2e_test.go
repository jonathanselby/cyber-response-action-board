package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// notifyCapture records what the test gateway and webhook received.
type notifyCapture struct {
	mu       sync.Mutex
	sms      []map[string]string // {"to","text"}
	teams    []string
	failHook atomic.Bool
}

func (c *notifyCapture) smsTo(to string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for _, m := range c.sms {
		if m["to"] == to {
			out = append(out, m["text"])
		}
	}
	return out
}

func (c *notifyCapture) all() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	raw, _ := json.Marshal(struct {
		SMS   []map[string]string
		Teams []string
	}{c.sms, c.teams})
	return string(raw)
}

func (c *notifyCapture) smsCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.sms)
}

func notificationFixture(t *testing.T) *notifyCapture {
	t.Helper()
	c := &notifyCapture{}
	c.failHook.Store(true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch r.URL.Path {
		case "/sms":
			var m map[string]string
			_ = json.Unmarshal(body, &m)
			c.mu.Lock()
			c.sms = append(c.sms, m)
			c.mu.Unlock()
		case "/teams":
			var m map[string]string
			_ = json.Unmarshal(body, &m)
			c.mu.Lock()
			c.teams = append(c.teams, m["text"])
			c.mu.Unlock()
		case "/regulator":
			if c.failHook.Load() {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	cfg := fmt.Sprintf(`{"channels":[
 {"name":"ops-log","type":"local_file","target":%q,"destination_id":"ops-log"},
 {"name":"regulator","type":"webhook","target":%q,"destination_id":"regulator-portal"},
 {"name":"teams","type":"webhook","target":%q,"destination_id":"ir-channel","body_shape":"text"},
 {"name":"sms","type":"sms-http","target":%q,"destination_id":"sms-gateway","address_pattern":"^\\+61[0-9]{9}$","body_template":"{\"to\":\"{to}\",\"text\":\"{message}\"}"}],
 "notices":{"escalation_enabled":true,"escalation_to_members":true,"escalation_shared_channels":["teams"],"decision_window_minutes":15}}`,
		filepath.Join(dir, "ops.log"), srv.URL+"/regulator", srv.URL+"/teams", srv.URL+"/sms")
	path := filepath.Join(dir, "notify.json")
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CRAB_NOTIFICATION_CONFIG_FILE", path)
	return c
}

var (
	draftIDField = regexp.MustCompile(`name="draft_id" value="([^"]+)"`)
	commIDField  = regexp.MustCompile(`name="communication_id" value="([^"]+)"`)
)

const (
	carolPhone = "+61400000111"
	bobPhone   = "+61400000222"
	erinPhone  = "+61400000333"
)

// T-233, T-234, T-251, T-252: notifications end to end.
func TestNotifications(t *testing.T) {
	capture := notificationFixture(t)
	clk := &testClock{now: time.Now().UTC().Truncate(time.Second)}
	e := newE2EWithClock(t, clk)
	alice, bob := e.bootstrap()
	ctx := context.Background()
	rt := e.served
	pass := func() {
		t.Helper()
		if _, err := rt.NoticePass(ctx); err != nil {
			t.Fatal(err)
		}
	}
	pass() // the first pass records where notices start

	e.declareLive(alice, bob, "INC-N", "Ransomware on payroll", "Severity level 3")
	carol, _ := e.onboard(alice, bob, "carol", "Minor", false)
	e.onboard(alice, bob, "dave", "Minor", false)
	erin, _ := e.onboard(alice, bob, "erin", "Major", false)
	for _, who := range []string{"bob", "carol", "dave", "erin"} {
		e.addMember(alice, bob, "INC-N", who)
	}
	e.post("/incident/INC-N/access/sponsor", alice, url.Values{"human_id": {"bob"}, "active": {"true"}})
	e.approveAll(bob)

	// T-252: routes. A person sets their own; a Custodian sets anyone's. The
	// address must match the channel; the ledger gets only a masked hint.
	if body := e.refused("/account/route", carol, url.Values{"channel": {"sms"}, "address": {"0400000111"}}); !strings.Contains(body, "not accepted") {
		t.Fatalf("bad address not refused clearly: %s", firstError(body))
	}
	e.post("/account/route", carol, url.Values{"channel": {"sms"}, "address": {carolPhone}})
	e.post("/access/route", alice, url.Values{"human_id": {"bob"}, "channel": {"sms"}, "address": {bobPhone}})
	e.refused("/access/route", carol, url.Values{"human_id": {"erin"}, "channel": {"sms"}, "address": {erinPhone}})
	if page := e.get("/account", carol); !strings.Contains(page, "…111") || strings.Contains(page, carolPhone) {
		t.Fatal("account page must show the masked hint only")
	}
	e.refused("/account/route", carol, url.Values{"channel": {"ops-log"}, "address": {carolPhone}}) // a fixed channel is not a route
	pass()

	// Event 1: assignment, to the owner only, content-free.
	e.post("/incident/INC-N/action", alice, url.Values{"title": {"Rebuild payroll server"}, "owner": {"carol"}, "priority": {"P1"}, "source_ref": {"Runbook"}})
	e.post("/incident/INC-N/role", alice, url.Values{"role_name": {"Comms lead"}, "human_id": {"dave"}})
	e.approveAll(bob)
	pass()
	got := capture.smsTo(carolPhone)
	if len(got) != 1 || got[0] != "CRAB: you have a new assignment on INC-N. Sign in to CRAB." {
		t.Fatalf("carol's assignment notice = %q", got)
	}
	pass() // idempotent: nothing twice
	if n := len(capture.smsTo(carolPhone)); n != 1 {
		t.Fatalf("assignment notified %d times", n)
	}

	// Ordinary activity notifies nobody.
	before := capture.smsCount()
	e.post("/incident/INC-N/fact", carol, url.Values{"statement": {"Payroll share encrypted"}, "source": {"EDR"}})
	// A confirmed status change to an action does not re-notify its owner.
	e.post("/incident/INC-N/action/status", alice, url.Values{"action_id": {actionIDFor(e, alice, "INC-N", "Rebuild payroll server")}, "status": {"in_progress"}})
	e.approveAll(bob)
	pass()
	if capture.smsCount() != before {
		t.Fatalf("ordinary changes notified: %s", capture.all())
	}

	// Event 2: decision needed, scope-safe count, debounced per window. A
	// restricted proposal erin may not read is never counted.
	e.post("/incident/INC-N/access/controller", alice, url.Values{"human_id": {"bob"}, "compartment_name": {"FORENSICS"}, "active": {"true"}})
	e.approveAll(bob)
	e.post("/incident/INC-N/access/compartment-member", bob, url.Values{"human_id": {"bob"}, "compartment_name": {"FORENSICS"}, "active": {"true"}, "reason": {"lead"}})
	e.approveAll(alice)
	e.post("/incident/INC-N/access/compartment-member", bob, url.Values{"human_id": {"alice"}, "compartment_name": {"FORENSICS"}, "active": {"true"}, "reason": {"second"}})
	e.approveAll(alice)
	pass()
	e.post("/access/route", alice, url.Values{"human_id": {"erin"}, "channel": {"sms"}, "address": {erinPhone}})
	// A restricted proposal: a communication issue in FORENSICS.
	e.post("/incident/INC-N/communications/draft", bob, url.Values{"compartment": {"FORENSICS"}, "kind": {"executive_update"}, "subject": {"Image the domain controller"}, "body": {"Restricted line"}, "audience": {"Board"}})
	restricted := draftIDField.FindStringSubmatch(e.get("/incident/INC-N/communications?compartment=FORENSICS", bob))[1]
	e.post("/incident/INC-N/communications/issue", bob, url.Values{"compartment": {"FORENSICS"}, "draft_id": {restricted}})
	e.post("/incident/INC-N/decision", alice, url.Values{"decision": {"Pay no ransom"}, "rationale": {"Policy"}})
	pass()
	if got := capture.smsTo(erinPhone); len(got) != 1 || got[0] != "CRAB: 1 item on INC-N is waiting for your decision. Sign in to CRAB." {
		t.Fatalf("erin's decision notice = %q (the restricted proposal must not be counted)", got)
	}
	clk.Advance(time.Minute)
	e.post("/incident/INC-N/decision", alice, url.Values{"decision": {"Engage forensics firm"}, "rationale": {"Retainer"}})
	clk.Advance(4 * time.Minute)
	pass()
	if n := len(capture.smsTo(erinPhone)); n != 1 {
		t.Fatalf("decision notices inside the window: %d", n)
	}
	clk.Advance(11 * time.Minute)
	pass()
	if got := capture.smsTo(erinPhone); len(got) != 2 || got[1] != "CRAB: 2 items on INC-N are waiting for your decision. Sign in to CRAB." {
		t.Fatalf("erin's second decision notice = %q", got)
	}
	// Signing in again keeps everyone's session fresh on the moved clock.
	alice, bob, carol, erin = e.session("alice"), e.session("bob"), e.signIn("carol", ""), e.signIn("erin", "")
	e.approveAll(erin)

	// No route: recorded, nothing sent, the item stays on their summary.
	e.post("/incident/INC-N/action", alice, url.Values{"title": {"Call insurer"}, "owner": {"dave"}, "priority": {"P2"}, "source_ref": {"Runbook"}})
	e.approveAll(bob)
	pass()
	comms := e.get("/incident/INC-N/communications", alice)
	if !strings.Contains(comms, "Nothing sent: no notification route") {
		t.Fatal("no-route notice not shown")
	}
	if !strings.Contains(e.get("/incident/INC-N", e.signIn("dave", "")), "Call insurer") {
		t.Fatal("the item must stay visible to dave")
	}

	// Event 3: deadline, to the clock owner, from the timed wait list.
	insertDueClock(t, e, "INC-N", "CLK-1", "carol", clk.Now().Add(-time.Minute))
	pass()
	if got := capture.smsTo(carolPhone); len(got) != 2 || got[1] != "CRAB: a deadline you own on INC-N needs attention. Sign in to CRAB." {
		t.Fatalf("deadline notice = %q", got)
	}

	// Event 5: temporary access expiring soon, to the Sponsor who can renew it.
	e.onboard(alice, bob, "frank", "Minor", false)
	e.post("/incident/INC-N/access/member", alice, url.Values{"human_id": {"frank"}, "active": {"true"}, "reason": {"contractor"}, "expires_at": {clk.Now().Add(3 * time.Hour).Format("2006-01-02T15:04")}})
	e.approveAll(bob)
	pass()
	if got := capture.smsTo(bobPhone); !containsText(got, "CRAB: temporary access you can renew on INC-N expires soon. Sign in to CRAB.") {
		t.Fatalf("bob's access notice missing: %q", got)
	}

	// Event 4: confirmed severity change, to members and, content-free with
	// no incident ID, to the shared destination.
	e.post("/incident/INC-N/severity", alice, url.Values{"severity": {"Severity level 1"}, "reason_evidence": {"Spread to finance"}})
	e.approveAll(bob)
	pass()
	if got := capture.smsTo(carolPhone); !containsText(got, "CRAB: an incident you are on on INC-N has a confirmed severity or phase change. Sign in to CRAB.") {
		t.Fatalf("carol's escalation notice missing: %q", got)
	}
	capture.mu.Lock()
	teams := append([]string(nil), capture.teams...)
	capture.mu.Unlock()
	if len(teams) != 1 || teams[0] != "CRAB: an incident has a confirmed severity or phase change. Sign in to CRAB." {
		t.Fatalf("shared destination got %q", teams)
	}

	// Content: no notice ever carries a title, a compartment or a link.
	for _, leak := range []string{"Ransomware", "Rebuild payroll", "FORENSICS", "Image the domain", "http", "Pay no ransom"} {
		if strings.Contains(capture.all(), leak) {
			t.Fatalf("a notice carried %q: %s", leak, capture.all())
		}
	}

	// Exercise incidents: notices are suppressed before I/O.
	e.declare(alice, bob, "INC-X", "Tabletop", "Severity level 4")
	e.addMember(alice, bob, "INC-X", "bob")
	e.addMember(alice, bob, "INC-X", "carol")
	pass()
	before = capture.smsCount()
	e.post("/incident/INC-X/action", alice, url.Values{"title": {"Practise call tree"}, "owner": {"carol"}, "priority": {"P2"}, "source_ref": {"Runbook"}})
	e.approveAll(bob)
	pass()
	if capture.smsCount() != before || !strings.Contains(e.get("/incident/INC-X/communications", alice), "SUPPRESSED") {
		t.Fatal("exercise notice was sent or not recorded as suppressed")
	}

	// T-233/T-234: an issued communication's delivery, failure and resend.
	e.post("/incident/INC-N/communications/draft", alice, url.Values{"kind": {"regulator_notification"}, "subject": {"Initial notice"}, "body": {"We are investigating."}, "audience": {"Regulator"}, "channels": {"ops-log\nregulator"}})
	draftID := draftIDField.FindStringSubmatch(e.get("/incident/INC-N/communications", alice))[1]
	e.post("/incident/INC-N/communications/issue", alice, url.Values{"draft_id": {draftID}})
	e.approveAll(bob)
	commID := commIDField.FindStringSubmatch(e.get("/incident/INC-N/communications", alice))[1]
	e.post("/access/output", alice, url.Values{"human_id": {"alice"}, "action": {"notify"}, "active": {"true"}})
	e.approveAll(bob)
	e.post("/incident/INC-N/communications/notify", alice, url.Values{"communication_id": {commID}, "channels": {"ops-log\nregulator"}})
	page := e.get("/incident/INC-N/communications", alice)
	if !strings.Contains(page, "Failed — not retried automatically") || !strings.Contains(page, "Resend to failed: regulator") {
		t.Fatal("failed delivery not shown with resend")
	}
	if !strings.Contains(e.get("/incident/INC-N", alice), "not retried automatically") {
		t.Fatal("failed notification not on the Board")
	}
	e.refused("/incident/INC-N/communications/resend", carol, url.Values{"communication_id": {commID}}) // no notify permission
	capture.failHook.Store(false)
	e.post("/incident/INC-N/communications/resend", alice, url.Values{"communication_id": {commID}})
	page = e.get("/incident/INC-N/communications", alice)
	if !strings.Contains(page, "resend") || strings.Contains(page, "Resend to failed") {
		t.Fatal("resend not recorded as a linked request")
	}
	if strings.Contains(e.get("/incident/INC-N", alice), "not retried automatically") {
		t.Fatal("a resent failure must leave the Board")
	}
	e.refused("/incident/INC-N/communications/resend", alice, url.Values{"communication_id": {commID}}) // nothing left to resend

	// T-252: no address in the ledger, projections or outcomes; disabling a
	// person deletes their route and the audit records remain.
	for _, phone := range []string{carolPhone, bobPhone, erinPhone} {
		var n int
		if err := e.admin.QueryRow(ctx, `SELECT
			(SELECT count(*) FROM crab.ledger_entries WHERE position($1 in convert_from(entry_bytes,'UTF8')) > 0) +
			(SELECT count(*) FROM crab.notices WHERE position($1 in detail) > 0) +
			(SELECT count(*) FROM crab.notification_outcomes WHERE position($1 in detail) > 0)`, phone).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("address %s found in %d ledger or projection rows", phone, n)
		}
	}
	var hints int
	if err := e.admin.QueryRow(ctx, `SELECT count(*) FROM crab.ledger_entries WHERE position('…111' in convert_from(entry_bytes,'UTF8')) > 0`).Scan(&hints); err != nil || hints == 0 {
		t.Fatalf("masked hint not audited: %d %v", hints, err)
	}
	e.post("/access/person/status", alice, url.Values{"human_id": {"carol"}, "status": {"disabled"}, "reason": {"left the organisation"}})
	e.approveAll(bob)
	var routes int
	if err := e.admin.QueryRow(ctx, `SELECT count(*) FROM crab.notification_routes WHERE human_id='carol'`).Scan(&routes); err != nil || routes != 0 {
		t.Fatalf("disabled person's route kept: %d %v", routes, err)
	}
	if err := e.admin.QueryRow(ctx, `SELECT count(*) FROM crab.ledger_entries WHERE position('notification_route_changed' in convert_from(entry_bytes,'UTF8')) > 0 AND position('"carol"' in convert_from(entry_bytes,'UTF8')) > 0`).Scan(&hints); err != nil || hints == 0 {
		t.Fatalf("route audit for carol lost: %d %v", hints, err)
	}
	_ = erin
}

func containsText(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

var actionRowID = regexp.MustCompile(`name="action_id" value="([^"]+)"`)

// actionIDFor finds the record ID of an action on the incident board.
func actionIDFor(e *e2e, cookie, incidentID, title string) string {
	e.t.Helper()
	page := e.get("/incident/"+incidentID, cookie)
	i := strings.Index(page, title)
	if i < 0 {
		e.t.Fatalf("action %q not on the board", title)
	}
	m := actionRowID.FindStringSubmatch(page[i:])
	if m == nil {
		e.t.Fatalf("action %q has no status form", title)
	}
	return m[1]
}

// insertDueClock puts an active obligation clock and its due wake-up into the
// projections directly, standing in for an obligation trigger.
func insertDueClock(t *testing.T, e *e2e, incidentID, clockID, owner string, wake time.Time) {
	t.Helper()
	ctx := context.Background()
	if _, err := e.admin.Exec(ctx, `INSERT INTO crab.obligation_clocks(incident_id,clock_id,compartment,definition_id,name,lifecycle,status,trigger_at,source_record_id,source_field,original_definition_id,original_deadline,deadline,recipient,owner_text,authority_source,time_status,maker)
		VALUES($1,$2,'GENERAL','DEF-1','Regulator notification','firm','active',$3,'SRC-1','awareness','DEF-1',$4,$4,'Regulator',$5,'Statute','trusted','alice')`, incidentID, clockID, wake.Add(-time.Hour), wake.Add(time.Hour), owner); err != nil {
		t.Fatal(err)
	}
	if _, err := e.admin.Exec(ctx, `INSERT INTO crab.timed_wait_list(incident_id,clock_id,wake_at,wake_kind,lifecycle,simulated) VALUES($1,$2,$3,'obligation_warning','firm',false)`, incidentID, clockID, wake); err != nil {
		t.Fatal(err)
	}
}
