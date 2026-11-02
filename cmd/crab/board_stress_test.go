package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"
)

// Command-board stress fixture (R-21, triage §3.15): 41 actions, 13 unknowns,
// 7 obligation clocks, 6 roles, 18 decisions, 4 compartments, 3 related
// incidents and 8 approvals waiting, on INC-STRESS. It is seeded through the
// browser runtime; only obligation clocks and action due times are written
// as projection fixtures, because the browser cannot set them directly.
const stressIncident = "INC-STRESS"

// StressCounts is what seedBoardStress creates.
var stressCounts = map[string]int{"actions": 41, "unknowns": 13, "clocks": 7, "roles": 6, "decisions": 18, "compartments": 4, "related": 3, "approvals": 8}

func (e *e2e) seedBoardStress(alice, bob string) {
	e.t.Helper()
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		e.t.Helper()
		if _, err := e.admin.Exec(ctx, sql, args...); err != nil {
			e.t.Fatal(err)
		}
	}
	inc := "/incident/" + stressIncident
	e.declareLive(alice, bob, stressIncident, "Ransomware across three sites", "Severity level 4")
	e.setMember(alice, bob, stressIncident, "bob", true)

	// Three related incidents.
	for i := 1; i <= 3; i++ {
		id := fmt.Sprintf("INC-REL%d", i)
		e.declareLive(alice, bob, id, fmt.Sprintf("Related site %d", i), "Severity level 2")
		e.setMember(alice, bob, id, "bob", true)
		e.post(inc+"/relationship", alice, url.Values{"related_incident_id": {id}, "reason": {"Same threat actor infrastructure"}, "active": {"true"}})
		e.approveAll(bob)
	}
	// Four compartments: GENERAL plus three restricted.
	for _, c := range []string{"LEGAL", "FORENSICS", "HR"} {
		e.post(inc+"/access/controller", alice, url.Values{"human_id": {"alice"}, "compartment_name": {c}, "active": {"true"}})
		e.approveAll(bob)
		e.post(inc+"/access/compartment-member", alice, url.Values{"human_id": {"alice"}, "compartment_name": {c}, "active": {"true"}, "reason": {"lead"}})
		e.approveAll(bob)
	}
	// Six operational roles.
	for i, role := range []string{"Incident commander", "Deputy commander", "Technical lead", "Communications lead", "Legal lead", "Scribe"} {
		who := "alice"
		if i%2 == 1 {
			who = "bob"
		}
		e.post(inc+"/role", alice, url.Values{"role_name": {role}, "human_id": {who}})
	}
	e.approveAll(bob)
	// 41 actions in three priorities.
	for i := 1; i <= 41; i++ {
		e.post(inc+"/action", alice, url.Values{"title": {fmt.Sprintf("Stress action %02d: contain host FIN-WS-%02d", i, i)}, "owner": {"bob"}, "priority": {fmt.Sprintf("P%d", 1+i%3)}, "source_ref": {"Runbook RB-07"}})
	}
	e.approveAll(bob)
	// Some actions are overdue and some due later (the browser has no due
	// field yet).
	now := e.now()
	exec(`UPDATE crab.incident_actions a SET due_at = $2::timestamptz + ((r.n % 5) - 2) * interval '1 hour'
		FROM (SELECT record_id, row_number() OVER (ORDER BY record_id) AS n FROM crab.incident_actions WHERE incident_id=$1) r
		WHERE a.incident_id=$1 AND a.record_id=r.record_id`, stressIncident, now)
	// 13 unknowns.
	for i := 1; i <= 13; i++ {
		e.post(inc+"/uncertainty", alice, url.Values{"kind": {"unknown"}, "statement": {fmt.Sprintf("Stress unknown %02d: was site %d reached?", i, i)}})
	}
	// 18 confirmed decisions.
	for i := 1; i <= 18; i++ {
		e.post(inc+"/decision", alice, url.Values{"decision": {fmt.Sprintf("Stress decision %02d: isolate VLAN %d", i, 100+i)}, "rationale": {"Contain spread"}})
	}
	e.approveAll(bob)
	// Seven obligation clocks: two overdue, five running.
	for i := 1; i <= 7; i++ {
		deadline := now.Add(time.Duration(i*6-12) * time.Hour)
		exec(`INSERT INTO crab.obligation_clocks(incident_id,clock_id,compartment,definition_id,name,lifecycle,status,trigger_at,source_record_id,source_field,original_definition_id,original_deadline,deadline,recipient,owner_text,authority_source,time_status,maker)
			VALUES($1,$2,'GENERAL','DEF','Stress clock '||$2,'firm','active',$3,'SRC','awareness','DEF',$4,$4,'Regulator','alice','policy','trusted','alice')`,
			stressIncident, fmt.Sprintf("C%d", i), now.Add(-24*time.Hour), deadline)
	}
	// Eight proposals waiting for bob.
	for i := 1; i <= 8; i++ {
		e.post(inc+"/decision", alice, url.Values{"decision": {fmt.Sprintf("Waiting decision %d: rebuild domain controller %d", i, i)}, "rationale": {"Recovery"}})
	}
}

// R-21 fixture: the stress incident is complete and its Board renders.
func TestBoardStressFixture(t *testing.T) {
	e := newE2E(t)
	alice, bob := e.bootstrap()
	e.seedBoardStress(alice, bob)
	ctx := context.Background()
	count := func(sql string) int {
		t.Helper()
		var n int
		if err := e.admin.QueryRow(ctx, sql, stressIncident).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	got := map[string]int{
		"actions":      count(`SELECT count(*) FROM crab.incident_actions WHERE incident_id=$1`),
		"unknowns":     count(`SELECT count(*) FROM crab.incident_uncertainties WHERE incident_id=$1`),
		"clocks":       count(`SELECT count(*) FROM crab.obligation_clocks WHERE incident_id=$1`),
		"roles":        count(`SELECT count(*) FROM crab.incident_operational_roles WHERE incident_id=$1`),
		"decisions":    count(`SELECT count(*) FROM crab.incident_decisions WHERE incident_id=$1`),
		"compartments": 1 + count(`SELECT count(DISTINCT compartment) FROM crab.compartment_controllers WHERE incident_id=$1`),
		"related":      count(`SELECT count(*) FROM crab.incident_relationships WHERE incident_id=$1`),
		"approvals":    count(`SELECT count(*) FROM crab.incident_proposals WHERE incident_id=$1`),
	}
	for k, want := range stressCounts {
		if got[k] != want {
			t.Errorf("stress fixture %s = %d, want %d", k, got[k], want)
		}
	}
	start := time.Now()
	w := e.do(http.MethodGet, "/incident/"+stressIncident, alice, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("stress Board code=%d", w.Code)
	}
	t.Logf("stress Board: %d bytes in %s", w.Body.Len(), time.Since(start))
}
