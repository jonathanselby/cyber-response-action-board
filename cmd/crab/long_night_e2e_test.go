package main

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/data/postgres"
)

// onboardUntil is onboard with a chosen account end for an external person.
func (e *e2e) onboardUntil(maker, confirmer, id, role string, accountEnd time.Time) (session, totpKey string) {
	e.t.Helper()
	e.post("/access/person", maker, url.Values{"human_id": {id}, "display_name": {strings.ToUpper(id[:1]) + id[1:]}, "role": {role}, "external": {"true"}, "account_expires_at": {accountEnd.Format("2006-01-02T15:04")}})
	e.approveAll(confirmer)
	e.post("/access/person/reset", maker, url.Values{"human_id": {id}, "reason": {"onboarding"}})
	code := e.confirmReset(confirmer, id)
	begin := e.do(http.MethodPost, "/enrol/begin", "", url.Values{"human_id": {id}, "code": {code}})
	m := totpSecret.FindStringSubmatch(begin.Body.String())
	if m == nil {
		e.t.Fatalf("no authenticator key offered to %s", id)
	}
	done := e.do(http.MethodPost, "/enrol/complete", "", url.Values{"human_id": {id}, "code": {code}, "password": {"Onboard-" + id + "-26!"}, "totp": {totpAt(e.t, m[1], e.now())}})
	for _, c := range done.Result().Cookies() {
		if c.Name == sessionName && c.Value != "" {
			return c.Value, m[1]
		}
	}
	e.t.Fatalf("onboarding %s failed: %s", id, firstError(done.Body.String()))
	return "", ""
}

// at formats a governed time for a datetime-local field.
func at(t time.Time) string { return t.UTC().Format("2006-01-02T15:04") }

// T-227: Bafflewick phase 10, "the long night". A 72-hour incident on the
// governed clock, run only through the browser runtime: day and night shifts,
// people joining and leaving, role changes, external forensics and legal,
// communications, handovers, temporary and expiring access, and a change of
// Major. It needs no shared account, no SQL and no break-glass, and history
// survives verification and rebuild.
func TestTheLongNight(t *testing.T) {
	start := time.Date(2026, 11, 6, 22, 0, 0, 0, time.UTC)
	clk := &testClock{now: start}
	e := newE2EWithClock(t, clk)
	step := func(hours float64, what string) {
		t.Helper()
		clk.Advance(time.Duration(hours * float64(time.Hour)))
		t.Logf("H+%02.0f %s", clk.Now().Sub(start).Hours(), what)
	}

	// H+0 — first report becomes an incident; the day team forms.
	alice, bob := e.bootstrap()
	e.declare(alice, bob, "INC-LN", "Long night: payment platform ransomware", "Severity level 4")
	nina, _ := e.onboard(alice, bob, "nina", "Major", false)
	dave, _ := e.onboard(alice, bob, "dave", "Minor", false)
	cora, _ := e.onboard(alice, bob, "cora", "Minor", false)
	for _, id := range []string{"bob", "nina", "dave", "cora"} {
		e.post("/incident/INC-LN/access/member", alice, url.Values{"human_id": {id}, "active": {"true"}, "reason": {"response team"}})
		e.approveAll(bob)
	}
	bob = e.login("bob", e2eBobPassword) // fresh session after joining
	e.post("/incident/INC-LN/role", alice, url.Values{"role_name": {"Incident commander"}, "human_id": {"alice"}})
	e.approveAll(nina)
	e.post("/incident/INC-LN/uncertainty", dave, url.Values{"kind": {"unknown"}, "statement": {"Initial access vector unknown"}})

	// H+1 — external forensics for six hours, read into FORENSICS.
	step(1, "external forensic specialist joins for six hours")
	jane, janeKey := e.onboardUntil(alice, bob, "jane", "Minor", clk.Now().Add(5*24*time.Hour))
	e.post("/incident/INC-LN/access/member", alice, url.Values{"human_id": {"jane"}, "active": {"true"}, "expires_at": {at(clk.Now().Add(6 * time.Hour))}, "reason": {"forensic imaging"}})
	e.approveAll(bob)
	e.post("/incident/INC-LN/access/controller", alice, url.Values{"human_id": {"alice"}, "compartment_name": {"FORENSICS"}, "active": {"true"}})
	e.approveAll(bob)
	e.post("/incident/INC-LN/access/compartment-member", alice, url.Values{"human_id": {"jane"}, "compartment_name": {"FORENSICS"}, "active": {"true"}, "expires_at": {at(clk.Now().Add(6 * time.Hour))}, "reason": {"imaging"}})
	e.approveAll(bob)
	jane = e.signIn("jane", janeKey)
	e.post("/incident/INC-LN/fact", jane, url.Values{"statement": {"Encryptor binary recovered from host PAY-07"}, "source": {"jane"}})

	// H+7 — forensics access has expired; her work remains.
	step(6, "forensics access expires")
	if e.reaches(jane, "/incident/INC-LN") {
		t.Error("H+7: jane still sees INC-LN after her access expired")
	}
	if !strings.Contains(e.get("/incident/INC-LN", nina), "Encryptor binary recovered") {
		t.Error("H+7: jane's fact disappeared when her access ended")
	}

	// H+9 — day sessions have reached their eight-hour limit.
	step(2, "day sessions reach the eight-hour session limit")
	if e.reaches(alice, "/incident/INC-LN") {
		t.Error("H+9: a session survived past the eight-hour limit")
	}
	alice, bob, nina = e.login("alice", e2eAlicePassword), e.login("bob", e2eBobPassword), e.signIn("nina", "")
	dave, cora = e.signIn("dave", ""), e.signIn("cora", "")

	// H+12 — night shift: handover and change of incident commander. A
	// decision alice proposed before handing over stays decidable.
	step(3, "shift change to nights; handover")
	e.post("/incident/INC-LN/handover", alice, url.Values{"next_objectives": {"Confirm backup integrity\nBrief insurer"}, "communication_commitments": {"Staff update at 08:00"}})
	e.post("/incident/INC-LN/role", alice, url.Values{"role_name": {"Incident commander"}, "human_id": {"nina"}})
	e.approveAll(bob)
	board := e.get("/incident/INC-LN", nina)
	if !commanderIs(board, "nina") {
		t.Error("H+12: the Board does not show nina as incident commander")
	}
	for _, want := range []string{"Confirm backup integrity", "Staff update at 08:00", "Initial access vector unknown"} {
		if !strings.Contains(board, want) {
			t.Errorf("H+12: handover view missing %q", want)
		}
	}

	// H+13 — the Major who handed over steps down to Minor. Her pending
	// proposal is still decided by another Major; she can no longer propose.
	step(1, "day lead steps down from Major to Minor")
	e.post("/incident/INC-LN/decision", alice, url.Values{"decision": {"Keep payments offline overnight"}, "rationale": {"containment holding"}})
	e.post("/access/person/role", bob, url.Values{"human_id": {"alice"}, "role": {"Minor"}, "remove": {"false"}})
	e.approveAll(alice) // the other Custodian confirms; alice keeps her Custodian duty
	e.refused("/incident/INC-LN/decision", alice, url.Values{"decision": {"Restore payments"}, "rationale": {"too early"}})
	if n := e.approveAll(nina); n == 0 {
		t.Error("H+13: alice's earlier proposal could not be decided by another Major after she stepped down")
	}
	if !strings.Contains(e.get("/incident/INC-LN", nina), "Keep payments offline overnight") {
		t.Error("H+13: confirmed decision missing from the operating picture")
	}

	// H+20 — a responder leaves: removed from the incident, then suspended.
	step(7, "a responder leaves")
	alice, bob, nina, dave = e.login("alice", e2eAlicePassword), e.login("bob", e2eBobPassword), e.signIn("nina", ""), e.signIn("dave", "")
	if !e.reaches(dave, "/incident/INC-LN") {
		t.Fatal("H+20: dave's fresh session cannot see INC-LN before he leaves")
	}
	// The incident sponsor (alice) removes access; a Custodian confirms.
	e.post("/incident/INC-LN/access/member", alice, url.Values{"human_id": {"dave"}, "active": {"false"}})
	e.approveAll(bob)
	if e.reaches(dave, "/incident/INC-LN") {
		t.Error("H+20: dave still sees INC-LN after leaving")
	}
	e.post("/access/person/status", bob, url.Values{"human_id": {"dave"}, "status": {"suspended"}, "reason": {"left the response"}})
	e.approveAll(alice)

	// H+24 — external legal counsel for the legal workstream; read into LEGAL.
	step(4, "external legal counsel joins; LEGAL compartment")
	cora = e.signIn("cora", "")
	lex, lexKey := e.onboardUntil(bob, alice, "lex", "Minor", clk.Now().Add(16*time.Hour))
	e.post("/incident/INC-LN/access/member", alice, url.Values{"human_id": {"lex"}, "active": {"true"}, "reason": {"privileged advice"}})
	e.approveAll(bob)
	e.post("/incident/INC-LN/access/controller", alice, url.Values{"human_id": {"nina"}, "compartment_name": {"LEGAL"}, "active": {"true"}})
	e.approveAll(bob)
	nina = e.signIn("nina", "")
	e.post("/incident/INC-LN/access/compartment-member", nina, url.Values{"human_id": {"nina"}, "compartment_name": {"LEGAL"}, "active": {"true"}, "reason": {"controller"}})
	e.approveAll(bob)
	e.post("/incident/INC-LN/access/compartment-member", nina, url.Values{"human_id": {"lex"}, "compartment_name": {"LEGAL"}, "active": {"true"}, "reason": {"counsel"}})
	e.approveAll(bob)
	lex = e.signIn("lex", lexKey)
	if !e.reaches(lex, "/incident/INC-LN/evidence?compartment=LEGAL") {
		t.Error("H+24: counsel cannot reach LEGAL")
	}
	if e.reaches(cora, "/incident/INC-LN/evidence?compartment=LEGAL") {
		t.Error("H+24: communications can reach LEGAL without read-in")
	}

	// H+30 — communications workstream drafts the customer notice.
	step(6, "communications drafts a customer notice")
	cora, nina = e.signIn("cora", ""), e.signIn("nina", "")
	e.post("/incident/INC-LN/communications/draft", cora, url.Values{"kind": {"holding_statement"}, "subject": {"Payment delays"}, "body": {"We are investigating a disruption to card payments."}, "audience": {"customers"}})
	if !strings.Contains(e.get("/incident/INC-LN/communications", nina), "Payment delays") {
		t.Error("H+30: communications draft not visible to the incident commander")
	}

	// H+36 — day shift again: handover back, commander changes to bob.
	step(6, "shift change to days; commander changes")
	bob, nina = e.login("bob", e2eBobPassword), e.signIn("nina", "")
	e.post("/incident/INC-LN/handover", nina, url.Values{"next_objectives": {"Restore payments in stages"}, "communication_commitments": {"Customer notice approval by 10:00"}})
	e.post("/incident/INC-LN/role", nina, url.Values{"role_name": {"Incident commander"}, "human_id": {"bob"}})
	e.approveAll(bob)
	lex = e.signIn("lex", lexKey)
	if !e.reaches(lex, "/incident/INC-LN") {
		t.Fatal("H+36: counsel's fresh session cannot see INC-LN before the account ends")
	}

	// H+41 — counsel's account ended at H+40; her live session (from H+36,
	// inside its eight-hour limit) and sign-in both stop.
	step(5, "external counsel's account ends")
	if e.reaches(lex, "/incident/INC-LN") {
		t.Error("H+41: counsel still reaches INC-LN after the account ended")
	}
	if w := e.do(http.MethodPost, "/login", "", url.Values{"human_id": {"lex"}, "password": {"Onboard-lex-26!"}, "totp": {totpAt(t, lexKey, clk.Now())}}); w.Code != http.StatusUnauthorized {
		t.Errorf("H+41: counsel signed in after the account ended (code %d)", w.Code)
	}

	// H+72 — end of the scenario: the record holds up.
	step(31, "end of scenario: verify and rebuild")
	bob = e.login("bob", e2eBobPassword)
	board = e.get("/incident/INC-LN", bob)
	if !commanderIs(board, "bob") {
		t.Error("H+72: the Board does not show bob as incident commander")
	}
	for _, want := range []string{"Encryptor binary recovered", "Keep payments offline overnight", "Restore payments in stages"} {
		if !strings.Contains(board, want) {
			t.Errorf("H+72: operating picture missing %q", want)
		}
	}
	rt := e.mcp()
	if result, err := rt.kernel.Verify(context.Background()); err != nil || !result.Valid {
		t.Fatalf("H+72: ledger verification failed: %+v %v", result, err)
	}
	before := e.projectionSnapshot()
	if _, err := rt.kernel.Rebuild(context.Background(), postgres.RebuildProjections()...); err != nil {
		t.Fatalf("H+72: rebuild failed: %v", err)
	}
	if after := e.projectionSnapshot(); after != before {
		t.Fatal("H+72: projections differ after rebuild from the ledger")
	}
	if strings.Contains(e.get("/incident/INC-LN", bob), "BREAK-GLASS") {
		t.Error("the scenario used break-glass")
	}
}

// supportingTables hold durable supporting state that is not rebuilt from the
// ledger (EDD §10.2), or the ledger itself.
var supportingTables = map[string]bool{
	"ledger_entries": true, "schema_migrations": true, "sessions": true, "local_credentials": true,
	"totp_credentials": true, "passkey_credentials": true, "break_glass_credentials": true,
	"blobs": true, "blob_chunks": true, "intake_key_secrets": true,
	"notification_routes": true, "notice_cursor": true,
}

// projectionSnapshot fingerprints every rebuildable projection table.
func (e *e2e) projectionSnapshot() string {
	e.t.Helper()
	ctx := context.Background()
	rows, err := e.admin.Query(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema='crab' AND table_type='BASE TABLE' ORDER BY table_name`)
	if err != nil {
		e.t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			e.t.Fatal(err)
		}
		if !supportingTables[name] {
			tables = append(tables, name)
		}
	}
	rows.Close()
	var out strings.Builder
	for _, table := range tables {
		var sum *string
		if err := e.admin.QueryRow(ctx, `SELECT md5(string_agg(t::text, '|' ORDER BY t::text)) FROM crab.`+table+` t`).Scan(&sum); err != nil {
			e.t.Fatal(err)
		}
		digest := "empty"
		if sum != nil {
			digest = *sum
		}
		out.WriteString(table + "=" + digest + "\n")
	}
	return out.String()
}

// commanderIs reports whether the Board's role table shows who as the current
// incident commander.
func commanderIs(board, who string) bool {
	return regexp.MustCompile(`(?s)<th scope="row">Incident commander</th><td>` + regexp.QuoteMeta(who) + `\b`).MatchString(board)
}
