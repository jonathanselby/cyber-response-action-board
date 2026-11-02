package postgres

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

// T-228: pending proposals of any kind and obligation clocks still running
// block closure, by name; waived and already-passed clocks do not.
func TestClosureBlockersIncludePendingProposalsAndRunningClocks(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	kernel, err := OpenKernel(ctx, db.runtime, ledger.Keyring{"server": base64.StdEncoding.EncodeToString(pub)})
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewRecoveryStore(db.runtime)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewRecoveryRuntime(kernel, Signer{KeyID: "server", PrivateKey: priv}, store)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 9, 1, 0, 0, 0, time.UTC)
	// Projection rows stand in for confirmed state; this test is about which
	// open matters block closure.
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO crab.incidents(incident_id,mode,title,summary,severity,declaration_record_id,declared_at) VALUES('INC-B','live','Blockers','fixture','Severity level 2','DECLARE-B',$1)`, []any{now}},
		{`INSERT INTO crab.incident_proposals(incident_id,record_id,record_type,compartment,version,maker,payload,proposed_at,mode) VALUES('INC-B','D-1','incident.decision','GENERAL','v1','alice','{}',$1,'live')`, []any{now}},
		{`INSERT INTO crab.incident_proposals(incident_id,record_id,record_type,compartment,version,maker,payload,proposed_at,mode) VALUES('INC-B','D-2','incident.decision','GENERAL','v1','alice','{}',$1,'live')`, []any{now}},
		{`INSERT INTO crab.access_proposals(scope_id,record_id,record_type,compartment,version,maker,payload,proposed_at,mode,time_status) VALUES('INC-B','M-1','security.incident_membership','GENERAL','v1','alice','{}',$1,'live','trusted')`, []any{now}},
	} {
		if _, err := db.admin.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatal(err)
		}
	}
	clock := func(id, status string, deadline time.Time, compartment ...string) {
		t.Helper()
		c := "GENERAL"
		if len(compartment) > 0 {
			c = compartment[0]
		}
		if _, err := db.admin.Exec(ctx, `INSERT INTO crab.obligation_clocks(incident_id,clock_id,compartment,definition_id,name,lifecycle,status,trigger_at,source_record_id,source_field,original_definition_id,original_deadline,deadline,recipient,owner_text,authority_source,time_status,maker)
			VALUES('INC-B',$1,$5,'DEF','Regulator notice '||$1,'firm',$2,$3,'SRC','awareness','DEF',$4,$4,'Regulator','alice','policy','trusted','alice')`, id, status, now.Add(-time.Hour), deadline, c); err != nil {
			t.Fatal(err)
		}
	}
	clock("running", "active", now.Add(48*time.Hour))
	clock("waived", "waived", now.Add(48*time.Hour))
	clock("passed", "active", now.Add(-time.Minute))
	clock("privileged", "active", now.Add(24*time.Hour), "LEGAL")
	if _, err := db.admin.Exec(ctx, `INSERT INTO crab.incident_proposals(incident_id,record_id,record_type,compartment,version,maker,payload,proposed_at,mode) VALUES('INC-B','D-L','incident.decision','LEGAL','v1','alice','{}',$1,'live')`, now); err != nil {
		t.Fatal(err)
	}

	service := app.RecoveryService{Backend: runtime}
	member := authz.Principal{Authenticated: true, HumanID: "alice", Role: authz.RoleMajor, Incidents: map[string]bool{"INC-B": true}, Compartments: map[string]map[string]bool{"INC-B": {"GENERAL": true}}}
	blockers, err := service.ClosureBlockers(ctx, member, "INC-B", now)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(blockers, "\n")
	for _, want := range []string{"stand-down not confirmed", "2 pending decision proposals", "1 pending incident membership proposal", "obligation clock Regulator notice running still running until 11 Nov 2026 01:00 UTC", "2 outstanding matters in compartments you cannot see"} {
		if !strings.Contains(joined, want) {
			t.Errorf("blockers missing %q:\n%s", want, joined)
		}
	}
	for _, unwanted := range []string{"waived", "notice passed", "privileged", "3 pending decision"} {
		if strings.Contains(joined, unwanted) {
			t.Errorf("blockers wrongly include %q:\n%s", unwanted, joined)
		}
	}
	// A LEGAL reader sees the privileged matters by name.
	legal := member
	legal.Compartments = map[string]map[string]bool{"INC-B": {"GENERAL": true, "LEGAL": true}}
	legalView, err := service.ClosureBlockers(ctx, legal, "INC-B", now)
	if err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join(legalView, "\n"); !strings.Contains(joined, "3 pending decision proposals") || !strings.Contains(joined, "Regulator notice privileged") || strings.Contains(joined, "cannot see") {
		t.Errorf("LEGAL reader view wrong:\n%s", joined)
	}
	outsider := authz.Principal{Authenticated: true, HumanID: "mallory", Role: authz.RoleMajor}
	if _, err := service.ClosureBlockers(ctx, outsider, "INC-B", now); !errors.Is(err, app.ErrAuthorisationDenied) {
		t.Fatalf("outsider read closure blockers err=%v", err)
	}
}
