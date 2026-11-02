package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/data/postgres"
)

var (
	intakeConfirmForm = regexp.MustCompile(`(?s)<form[^>]*action="/access/intake-key/confirm".*?</form>`)
	shownSecret       = regexp.MustCompile(`<code>([a-z0-9-]+\.[a-z2-7]{32})</code>`)
	reportIDField     = regexp.MustCompile(`"report_id":"([^"]+)"`)
)

// confirmIntakeKey confirms the pending intake key change whose Approvals
// summary contains summary, as confirmer, and returns the page shown.
func (e *e2e) confirmIntakeKey(confirmer, summary string) (int, string) {
	e.t.Helper()
	page := e.get("/approvals", confirmer)
	var f string
	for _, card := range approvalCard.FindAllString(page, -1) {
		if strings.Contains(htmlUnescape(card), summary) {
			f = intakeConfirmForm.FindString(card)
		}
	}
	if f == "" {
		e.t.Fatalf("no intake key change %q waiting in Approvals for this person", summary)
	}
	form := url.Values{}
	for _, m := range hiddenField.FindAllStringSubmatch(f, -1) {
		form.Set(m[1], htmlUnescape(m[2]))
	}
	w := e.do(http.MethodPost, "/access/intake-key/confirm", confirmer, form)
	return w.Code, w.Body.String()
}

func (e *e2e) issueIntakeKey(maker, confirmer, name, accountable string, expires time.Time) string {
	e.t.Helper()
	e.post("/access/intake-key", maker, url.Values{"action": {"issue"}, "key_name": {name}, "accountable_human": {accountable}, "expires_at": {at(expires)}, "reason": {"SIEM forwarding"}})
	code, body := e.confirmIntakeKey(confirmer, "Issue intake key "+name)
	m := shownSecret.FindStringSubmatch(body)
	if code != http.StatusOK || m == nil {
		e.t.Fatalf("issuing %s: code=%d %s", name, code, firstError(body))
	}
	return m[1]
}

type intakeResponse struct {
	status   int
	body     string
	reportID string
}

func (e *e2e) submit(secret, body string) intakeResponse {
	e.t.Helper()
	w := e.api(http.MethodPost, "/api/v1/first-reports", secret, body)
	r := intakeResponse{status: w.Code, body: w.Body.String()}
	if m := reportIDField.FindStringSubmatch(r.body); m != nil {
		r.reportID = m[1]
	}
	return r
}

// T-230 (AC-92): scoped machine intake through the real runtime.
func TestMachineIntakeKey(t *testing.T) {
	clk := &testClock{now: time.Date(2026, 11, 2, 3, 0, 0, 0, time.UTC)}
	e := newE2EWithClock(t, clk)
	alice, bob := e.bootstrap()
	ctx := context.Background()
	count := func(sql string, args ...any) int {
		t.Helper()
		var n int
		if err := e.admin.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// Issue: one Custodian proposes, the same one cannot confirm, another does.
	e.post("/access/intake-key", alice, url.Values{"action": {"issue"}, "key_name": {"siem-prod"}, "accountable_human": {"alice"}, "expires_at": {at(e.now().Add(30 * 24 * time.Hour))}, "reason": {"SIEM forwarding"}})
	if _, body := e.confirmIntakeKeyMaybe(alice, "Issue intake key siem-prod"); shownSecret.MatchString(body) {
		t.Fatal("the proposing Custodian confirmed their own intake key")
	}
	code, body := e.confirmIntakeKey(bob, "Issue intake key siem-prod")
	m := shownSecret.FindStringSubmatch(body)
	if code != http.StatusOK || m == nil {
		t.Fatalf("confirm issue code=%d %s", code, firstError(body))
	}
	s1 := m[1]
	if n := count(`SELECT count(*) FROM crab.ledger_entries WHERE convert_from(entry_bytes,'UTF8') LIKE $1`, "%"+strings.SplitN(s1, ".", 2)[1]+"%"); n != 0 {
		t.Fatal("intake key secret written to the ledger")
	}
	if !strings.Contains(e.get("/access", alice), "siem-prod") {
		t.Fatal("Access admin does not list the intake key")
	}

	// A valid key creates an ordinary first report, attributed to the key and
	// its accountable person, and nothing else happens.
	incidents := count(`SELECT count(*) FROM crab.incidents`)
	before := e.ledgerCount()
	created := e.submit(s1, `{"description":"EDR: credential dumping on FIN-WS-22","affected":"FIN-WS-22","external_ref":"edr-4411"}`)
	if created.status != http.StatusCreated || created.reportID == "" {
		t.Fatalf("submit status=%d %s", created.status, created.body)
	}
	if e.ledgerCount() != before+1 {
		t.Fatal("a submission did not commit exactly one record")
	}
	var maker, channel, key string
	if err := e.admin.QueryRow(ctx, `SELECT maker, payload->>'source_channel', payload->>'intake_key' FROM crab.first_reports WHERE report_id=$1`, created.reportID).Scan(&maker, &channel, &key); err != nil {
		t.Fatal(err)
	}
	if maker != "alice" || channel != "intake key siem-prod" || key != "siem-prod" {
		t.Fatalf("report attributed to maker=%q channel=%q key=%q", maker, channel, key)
	}
	if intake := e.get("/intake", alice); !strings.Contains(intake, "credential dumping") || !strings.Contains(intake, "intake key siem-prod") {
		t.Fatal("machine report not shown for triage with its source")
	}
	if count(`SELECT count(*) FROM crab.incidents`) != incidents {
		t.Fatal("a machine report created an incident without human triage")
	}

	// A repeated external_ref returns the original report and writes nothing.
	before = e.ledgerCount()
	repeat := e.submit(s1, `{"description":"EDR: credential dumping on FIN-WS-22 (retry)","external_ref":"edr-4411"}`)
	if repeat.status != http.StatusOK || repeat.reportID != created.reportID || e.ledgerCount() != before {
		t.Fatalf("repeat external_ref: status=%d report=%q (want %q), ledger +%d", repeat.status, repeat.reportID, created.reportID, e.ledgerCount()-before)
	}
	if bad := e.submit(s1, `{"description":"x","attachments":["a"]}`); bad.status != http.StatusBadRequest {
		t.Fatalf("attachment over the API accepted: %d", bad.status)
	}

	// The key can do nothing else, on any surface.
	refusal := e.submit("nokey.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", `{"description":"probe"}`)
	if refusal.status != http.StatusNotFound {
		t.Fatalf("unknown key status=%d", refusal.status)
	}
	identical := func(step string, got intakeResponse) {
		t.Helper()
		if got.status != refusal.status || got.body != refusal.body {
			t.Fatalf("%s: refusal differs: %d %q, want %d %q", step, got.status, got.body, refusal.status, refusal.body)
		}
	}
	for _, path := range []string{"/api/v1/incidents"} {
		if w := e.api(http.MethodGet, path, s1, ""); w.Code != http.StatusNotFound {
			t.Fatalf("intake key read %s: %d", path, w.Code)
		}
	}
	if w := e.api(http.MethodPost, "/api/v1/incidents", s1, `{"incident_id":"INC-K","mode":"live","title":"t","summary":"s","severity":"Severity level 2"}`); w.Code != http.StatusNotFound {
		t.Fatalf("intake key declared an incident: %d", w.Code)
	}
	if w := e.do(http.MethodGet, "/approvals", s1, nil); w.Code == http.StatusOK {
		t.Fatal("intake key opened the browser as a session")
	}
	if _, ok, _ := e.mcp().principal(ctx, s1); ok {
		t.Fatal("intake key became a principal for MCP")
	}
	identical("a person's session token on the intake endpoint", e.submit(alice, `{"description":"probe"}`))
	identical("wrong secret", e.submit("siem-prod.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", `{"description":"probe"}`))
	identical("malformed key", e.submit("garbage", `{"description":"probe"}`))

	// Rotation: the overlap is bounded, both secrets work during it, then
	// only the new one does.
	if msg := e.refused("/access/intake-key", alice, url.Values{"action": {"rotate"}, "key_name": {"siem-prod"}, "overlap_hours": {"25"}, "reason": {"too long"}}); !strings.Contains(msg, "between 0 and 24 hours") {
		t.Fatal("a 25-hour rotation overlap was not refused")
	}
	e.post("/access/intake-key", alice, url.Values{"action": {"rotate"}, "key_name": {"siem-prod"}, "overlap_hours": {"1"}, "reason": {"quarterly rotation"}})
	code, body = e.confirmIntakeKey(bob, "Rotate intake key siem-prod")
	m = shownSecret.FindStringSubmatch(body)
	if code != http.StatusOK || m == nil || !strings.Contains(body, "previous secret keeps working until") {
		t.Fatalf("confirm rotation code=%d %s", code, firstError(body))
	}
	s2 := m[1]
	for i, s := range []string{s1, s2} {
		if r := e.submit(s, fmt.Sprintf(`{"description":"during overlap %d"}`, i)); r.status != http.StatusCreated {
			t.Fatalf("secret %d refused during overlap: %d", i+1, r.status)
		}
	}
	clk.Advance(61 * time.Minute)
	identical("rotated-out secret", e.submit(s1, `{"description":"after overlap"}`))
	if r := e.submit(s2, `{"description":"after overlap"}`); r.status != http.StatusCreated {
		t.Fatalf("new secret refused after overlap: %d", r.status)
	}

	// Expiry.
	short := e.issueIntakeKey(alice, bob, "short-lived", "alice", e.now().Add(2*time.Hour))
	if r := e.submit(short, `{"description":"before expiry"}`); r.status != http.StatusCreated {
		t.Fatalf("short-lived key refused before expiry: %d", r.status)
	}
	clk.Advance(3 * time.Hour)
	identical("expired key", e.submit(short, `{"description":"after expiry"}`))

	// The key is bound to its accountable person: if they cannot act, neither
	// can the key.
	e.onboard(alice, bob, "carol", "Minor", false)
	carols := e.issueIntakeKey(alice, bob, "carol-script", "carol", e.now().Add(24*time.Hour))
	if r := e.submit(carols, `{"description":"carol's script"}`); r.status != http.StatusCreated {
		t.Fatalf("carol's key refused: %d", r.status)
	}
	e.post("/access/person/status", alice, url.Values{"human_id": {"carol"}, "status": {"suspended"}, "reason": {"left"}})
	e.approveAll(bob)
	identical("key of a suspended accountable person", e.submit(carols, `{"description":"after suspension"}`))

	// Revocation is immediate and removes the secrets.
	e.post("/access/intake-key", alice, url.Values{"action": {"revoke"}, "key_name": {"siem-prod"}, "reason": {"SIEM decommissioned"}})
	if code, body := e.confirmIntakeKey(bob, "Revoke intake key siem-prod"); code != http.StatusOK || !strings.Contains(body, "revoked") {
		t.Fatalf("confirm revoke code=%d %s", code, firstError(body))
	}
	identical("revoked key", e.submit(s2, `{"description":"after revocation"}`))
	if n := count(`SELECT count(*) FROM crab.intake_key_secrets WHERE key_name='siem-prod'`); n != 0 {
		t.Fatalf("revoked key still has %d secrets", n)
	}

	// Rate bound: beyond it, 429 and nothing written. No queue.
	flood := e.issueIntakeKey(alice, bob, "flood", "alice", e.now().Add(24*time.Hour))
	for i := 0; i < 60; i++ {
		if r := e.submit(flood, fmt.Sprintf(`{"description":"burst %d"}`, i)); r.status != http.StatusCreated {
			t.Fatalf("submission %d within the bound refused: %d", i+1, r.status)
		}
	}
	before = e.ledgerCount()
	if r := e.submit(flood, `{"description":"burst 61"}`); r.status != http.StatusTooManyRequests || e.ledgerCount() != before {
		t.Fatalf("over-rate submission: status=%d, ledger +%d", r.status, e.ledgerCount()-before)
	}
	clk.Advance(time.Minute + time.Second)
	if r := e.submit(flood, `{"description":"after the window"}`); r.status != http.StatusCreated {
		t.Fatalf("submission after the rate window refused: %d", r.status)
	}

	// Intake key state rebuilds from the ledger.
	snap := e.projectionSnapshot()
	if _, err := e.mcp().kernel.Rebuild(ctx, postgres.RebuildProjections()...); err != nil {
		t.Fatal(err)
	}
	if e.projectionSnapshot() != snap {
		t.Fatal("projections did not rebuild identically")
	}
}

// confirmIntakeKeyMaybe is confirmIntakeKey for a person who may not be
// offered the confirmation at all.
func (e *e2e) confirmIntakeKeyMaybe(confirmer, summary string) (int, string) {
	e.t.Helper()
	page := e.get("/approvals", confirmer)
	for _, card := range approvalCard.FindAllString(page, -1) {
		if strings.Contains(htmlUnescape(card), summary) {
			if f := intakeConfirmForm.FindString(card); f != "" {
				form := url.Values{}
				for _, m := range hiddenField.FindAllStringSubmatch(f, -1) {
					form.Set(m[1], htmlUnescape(m[2]))
				}
				w := e.do(http.MethodPost, "/access/intake-key/confirm", confirmer, form)
				return w.Code, w.Body.String()
			}
		}
	}
	return 0, ""
}
