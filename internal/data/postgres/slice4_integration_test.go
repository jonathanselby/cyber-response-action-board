package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

func slice4Action(title, status string) domain.Action {
	return domain.Action{
		Title:        title,
		OwnerHumanID: "carol",
		Status:       status,
		Priority:     "P1",
		SourceRef:    "RUNBOOK-RANSOMWARE-4.2",
	}
}

func TestPendingActionProposalCannotBeOverwritten(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	kernel, signer := slice3Kernel(t, db)
	scope := domain.Scope{IncidentID: "INC-PENDING", Mode: "live", Compartment: "GENERAL"}
	first, err := domain.ProposeAction(scope, "ACT-1", "alice", time.Date(2026, 10, 20, 4, 0, 0, 0, time.UTC), slice4Action("Isolate affected endpoints", domain.ActionStatusNotStarted))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kernel.Append(ctx, first, signer, IncidentProjection{}); err != nil {
		t.Fatal(err)
	}
	secondAction := slice4Action("Isolate affected endpoints", domain.ActionStatusInProgress)
	second, err := domain.ProposeAction(scope, "ACT-1", "alice", first.Timestamp.Add(time.Minute), secondAction)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kernel.Append(ctx, second, signer, IncidentProjection{}); err == nil {
		t.Fatal("unresolved Action proposal was overwritten")
	}
	expectedVersion, err := domain.ProposalVersion(first)
	if err != nil {
		t.Fatal(err)
	}
	var storedVersion string
	if err := db.runtime.QueryRow(ctx, `SELECT version FROM crab.incident_proposals WHERE incident_id=$1 AND record_type=$2 AND record_id=$3`, scope.IncidentID, domain.TypeAction, "ACT-1").Scan(&storedVersion); err != nil {
		t.Fatal(err)
	}
	if storedVersion != expectedVersion {
		t.Fatalf("pending proposal version changed from %s to %s", expectedVersion, storedVersion)
	}
	var ledgerEntries int
	if err := db.runtime.QueryRow(ctx, `SELECT count(*) FROM crab.ledger_entries`).Scan(&ledgerEntries); err != nil {
		t.Fatal(err)
	}
	if ledgerEntries != 1 {
		t.Fatalf("rejected overwrite committed ledger entries=%d", ledgerEntries)
	}
}

func TestActionLifecycleKeepsOneCurrentProjectionAndLedgerHistory(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	kernel, signer := slice3Kernel(t, db)
	projection := IncidentProjection{}
	scope := domain.Scope{IncidentID: "INC-ACTION", Mode: "live", Compartment: "GENERAL"}
	base := time.Date(2026, 10, 20, 4, 10, 0, 0, time.UTC)

	initial, err := domain.ProposeAction(scope, "ACT-1", "alice", base, slice4Action("Preserve endpoint images", domain.ActionStatusNotStarted))
	if err != nil {
		t.Fatal(err)
	}
	appendProposalAndDecision(t, kernel, signer, initial, "confirmed", projection)

	completedAction := slice4Action("Preserve endpoint images", domain.ActionStatusComplete)
	completedAction.EvidenceRefs = []string{"EVIDENCE-42"}
	completed, err := domain.ProposeAction(scope, "ACT-1", "alice", base.Add(2*time.Minute), completedAction)
	if err != nil {
		t.Fatal(err)
	}
	appendProposalAndDecision(t, kernel, signer, completed, "confirmed", projection)

	var count int
	if err := db.runtime.QueryRow(ctx, `SELECT count(*) FROM crab.incident_actions WHERE incident_id=$1 AND record_id=$2`, scope.IncidentID, "ACT-1").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("current Action projection rows=%d", count)
	}
	var status string
	if err := db.runtime.QueryRow(ctx, `SELECT status FROM crab.incident_actions WHERE incident_id=$1 AND record_id=$2`, scope.IncidentID, "ACT-1").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != domain.ActionStatusComplete {
		t.Fatalf("current Action status=%q", status)
	}
	var ledgerEntries int
	if err := db.runtime.QueryRow(ctx, `SELECT count(*) FROM crab.ledger_entries`).Scan(&ledgerEntries); err != nil {
		t.Fatal(err)
	}
	if ledgerEntries != 4 {
		t.Fatalf("Action lifecycle ledger entries=%d want 4", ledgerEntries)
	}
}

func TestTimelineKeepsOccurredFlaggedRecordedAndAwarenessDistinctAcrossRebuild(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	kernel, signer := slice3Kernel(t, db)
	projection := IncidentProjection{}
	scope := domain.Scope{IncidentID: "INC-TIME", Mode: "live", Compartment: "GENERAL"}
	occurred := time.Date(2026, 10, 20, 1, 0, 0, 0, time.UTC)
	flagged := occurred.Add(10 * time.Minute)
	aware := flagged.Add(2 * time.Minute)
	recorded := flagged.Add(5 * time.Minute)

	fact, err := domain.RecordFact(scope, "FACT-TIME", "alice", recorded, domain.Fact{
		Statement:  "Payroll files encrypted",
		Source:     "SOC",
		OccurredAt: &occurred,
		FlaggedAt:  &flagged,
		Awareness:  []domain.AwarenessTime{{Subject: "Incident Major", At: aware}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kernel.Append(ctx, fact, signer, projection); err != nil {
		t.Fatal(err)
	}
	assertTimelineTimes := func() {
		var gotOccurred, gotFlagged, gotRecorded time.Time
		var awarenessJSON []byte
		if err := db.runtime.QueryRow(ctx, `SELECT occurred_at,flagged_at,recorded_at,awareness FROM crab.incident_timeline WHERE incident_id=$1 AND record_id=$2`, scope.IncidentID, "FACT-TIME").Scan(&gotOccurred, &gotFlagged, &gotRecorded, &awarenessJSON); err != nil {
			t.Fatal(err)
		}
		if !gotOccurred.Equal(occurred) || !gotFlagged.Equal(flagged) || !gotRecorded.Equal(recorded) {
			t.Fatalf("timeline times occurred=%v flagged=%v recorded=%v", gotOccurred, gotFlagged, gotRecorded)
		}
		if len(awarenessJSON) == 0 || string(awarenessJSON) == "[]" {
			t.Fatal("timeline awareness missing")
		}
	}
	assertTimelineTimes()
	if _, err := kernel.Rebuild(ctx, projection); err != nil {
		t.Fatal(err)
	}
	assertTimelineTimes()
}
