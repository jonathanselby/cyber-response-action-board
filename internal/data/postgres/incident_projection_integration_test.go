package postgres

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

func slice3Kernel(t *testing.T, db *testDB) (*Kernel, Signer) {
	t.Helper()
	serverPub, serverPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyring := ledger.Keyring{"server": base64.StdEncoding.EncodeToString(serverPub)}
	kernel, err := OpenKernel(context.Background(), db.runtime, keyring)
	if err != nil {
		t.Fatal(err)
	}
	return kernel, Signer{KeyID: "server", PrivateKey: serverPriv}
}

func appendProposalAndDecision(t *testing.T, kernel *Kernel, signer Signer, proposal ledger.Record, outcome string, projection Projection) {
	t.Helper()
	ctx := context.Background()
	if _, err := kernel.Append(ctx, proposal, signer, projection); err != nil {
		t.Fatalf("append proposal %s: %v", proposal.RecordID, err)
	}
	version, err := domain.ProposalVersion(proposal)
	if err != nil {
		t.Fatal(err)
	}
	decided, err := domain.Decide(proposal, version, "bob", outcome, proposal.Timestamp.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kernel.Append(ctx, decided, signer, projection); err != nil {
		t.Fatalf("append decision %s: %v", proposal.RecordID, err)
	}
}

func TestIncidentProjectionBuildsSemanticOperatingPicture(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	kernel, signer := slice3Kernel(t, db)
	projection := IncidentProjection{}
	scope := domain.Scope{IncidentID: "INC-001", Mode: "live", Compartment: "GENERAL"}
	base := time.Date(2026, 10, 20, 3, 0, 0, 0, time.UTC)

	declaration, err := domain.ProposeIncidentDeclaration(scope, "DECLARE-1", "alice", base, domain.IncidentDeclaration{Title: "Payroll ransomware", Summary: "Encryption reported on payroll workstations", Severity: "High"})
	if err != nil {
		t.Fatal(err)
	}
	appendProposalAndDecision(t, kernel, signer, declaration, "confirmed", projection)

	fact, err := domain.RecordFact(scope, "FACT-1", "alice", base.Add(2*time.Minute), domain.Fact{Statement: "Three additional endpoints encrypted; severity should change to Critical", Source: "SOC"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kernel.Append(ctx, fact, signer, projection); err != nil {
		t.Fatal(err)
	}
	unknown, _ := domain.RecordUncertaintyItem(scope, "UNK-1", "alice", base.Add(3*time.Minute), domain.Uncertainty{Kind: "unknown", Statement: "Initial access vector"})
	hypothesis, _ := domain.RecordUncertaintyItem(scope, "HYP-1", "alice", base.Add(4*time.Minute), domain.Uncertainty{Kind: "hypothesis", Statement: "Credential stuffing preceded access"})
	if _, err := kernel.Append(ctx, unknown, signer, projection); err != nil {
		t.Fatal(err)
	}
	if _, err := kernel.Append(ctx, hypothesis, signer, projection); err != nil {
		t.Fatal(err)
	}

	action, _ := domain.ProposeAction(scope, "ACT-1", "alice", base.Add(5*time.Minute), domain.Action{Title: "Isolate affected endpoints", OwnerHumanID: "carol"})
	if _, err := kernel.Append(ctx, action, signer, projection); err != nil {
		t.Fatal(err)
	}
	store, _ := NewIncidentStore(db.runtime)
	picture, found, err := store.CommonOperatingPicture(ctx, "INC-001")
	if err != nil || !found {
		t.Fatalf("pending picture found=%v err=%v", found, err)
	}
	if len(picture.Pending) != 1 || len(picture.Actions) != 0 {
		t.Fatalf("proposal leaked into effective state pending=%d actions=%d", len(picture.Pending), len(picture.Actions))
	}
	version, _ := domain.ProposalVersion(action)
	confirmedAction, err := domain.Decide(action, version, "bob", "confirmed", base.Add(6*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kernel.Append(ctx, confirmedAction, signer, projection); err != nil {
		t.Fatal(err)
	}

	decision, _ := domain.ProposeDecision(scope, "DEC-1", "alice", base.Add(7*time.Minute), domain.Decision{Decision: "Contain before eradication", Rationale: "Preserve evidence"})
	appendProposalAndDecision(t, kernel, signer, decision, "confirmed", projection)
	role, _ := domain.ProposeOperationalRoleAssignment(scope, "ROLE-1", "alice", base.Add(9*time.Minute), domain.OperationalRoleAssignment{RoleName: "Technical Lead – Bafflewick", HumanID: "carol"})
	appendProposalAndDecision(t, kernel, signer, role, "confirmed", projection)

	picture, found, err = store.CommonOperatingPicture(ctx, "INC-001")
	if err != nil || !found {
		t.Fatalf("picture found=%v err=%v", found, err)
	}
	if picture.Incident.Severity != "High" {
		t.Fatalf("observational fact changed effective severity to %q", picture.Incident.Severity)
	}
	if len(picture.Facts) != 1 || len(picture.Unknowns) != 1 || len(picture.Hypotheses) != 1 || len(picture.Decisions) != 1 || len(picture.Actions) != 1 || len(picture.OperationalRoles) != 1 || len(picture.Pending) != 0 {
		t.Fatalf("unexpected semantic picture: %+v", picture)
	}
	if picture.OperationalRoles[0].Assignment.RoleName != "Technical Lead – Bafflewick" {
		t.Fatalf("organisation role label lost: %+v", picture.OperationalRoles[0])
	}

	if _, err := db.runtime.Exec(ctx, `TRUNCATE crab.incident_proposals, crab.incident_operational_roles, crab.incident_actions, crab.incident_decisions, crab.incident_uncertainties, crab.incident_facts, crab.incidents`); err != nil {
		t.Fatal(err)
	}
	if _, err := kernel.Rebuild(ctx, projection); err != nil {
		t.Fatal(err)
	}
	rebuilt, found, err := store.CommonOperatingPicture(ctx, "INC-001")
	if err != nil || !found || rebuilt.Incident.Severity != "High" || len(rebuilt.Facts) != 1 || len(rebuilt.Actions) != 1 || len(rebuilt.OperationalRoles) != 1 {
		t.Fatalf("rebuilt picture=%+v found=%v err=%v", rebuilt, found, err)
	}
}

func TestRejectedProposalNeverBecomesEffective(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	kernel, signer := slice3Kernel(t, db)
	projection := IncidentProjection{}
	scope := domain.Scope{IncidentID: "INC-REJECT", Mode: "exercise", Compartment: "GENERAL"}
	proposal, err := domain.ProposeAction(scope, "ACT-REJECT", "alice", time.Now().UTC(), domain.Action{Title: "Shut down payroll"})
	if err != nil {
		t.Fatal(err)
	}
	appendProposalAndDecision(t, kernel, signer, proposal, "rejected", projection)
	var actions, pending int
	if err := db.runtime.QueryRow(ctx, `SELECT count(*) FROM crab.incident_actions WHERE incident_id='INC-REJECT'`).Scan(&actions); err != nil {
		t.Fatal(err)
	}
	if err := db.runtime.QueryRow(ctx, `SELECT count(*) FROM crab.incident_proposals WHERE incident_id='INC-REJECT'`).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if actions != 0 || pending != 0 {
		t.Fatalf("rejected proposal actions=%d pending=%d", actions, pending)
	}
}
