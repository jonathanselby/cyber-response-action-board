package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

func confirmedObligationDefinition(t *testing.T, kernel *Kernel, signer Signer, id string, def domain.ObligationDefinition, at time.Time) {
	t.Helper()
	proposal, err := domain.ProposeObligationDefinition(id, "alice", at, def)
	if err != nil {
		t.Fatal(err)
	}
	appendProposalAndDecision(t, kernel, signer, proposal, "confirmed", ObligationProjection{})
}

func slice5Definition() domain.ObligationDefinition {
	return domain.ObligationDefinition{
		Name:                   "Regulator notification",
		TriggerField:           "flagged_at",
		Duration:               3,
		Unit:                   domain.ObligationUnitBusinessDays,
		Calendar:               domain.BusinessCalendar{Timezone: "Australia/Sydney", Weekend: []string{"Saturday", "Sunday"}, Holidays: []string{"2026-10-26"}, Adjustment: "next_business_day"},
		ApplicabilityCondition: "Organisation has determined the configured regulator rule applies",
		Recipient:              "Configured regulator",
		Owner:                  "Legal lead",
		AuthoritySource:        "Regulatory response plan v4.2",
		WarningBeforeMinutes:   []int{1440, 120},
	}
}

func TestObligationTriggerMovesProvisionalToFirmWithoutChangingDeadline(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	kernel, signer := slice3Kernel(t, db)
	projection := ObligationProjection{}
	base := time.Date(2026, 10, 20, 4, 30, 0, 0, time.UTC)
	confirmedObligationDefinition(t, kernel, signer, "DEF-1", slice5Definition(), base)
	loc, _ := time.LoadLocation("Australia/Sydney")
	triggerAt := time.Date(2026, 10, 23, 10, 0, 0, 0, loc)
	proposal, err := domain.ProposeObligationTrigger(domain.Scope{IncidentID: "INC-1", Mode: "live", Compartment: "GENERAL"}, "CLOCK-1", "alice", base.Add(2*time.Minute), domain.ObligationTrigger{DefinitionID: "DEF-1", SourceRecordID: "FACT-1", SourceField: "flagged_at", TriggerAt: triggerAt, ApplicabilityReason: "Legal lead determined configured rule applies"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kernel.Append(ctx, proposal, signer, projection); err != nil {
		t.Fatal(err)
	}
	store, _ := NewObligationStore(db.runtime)
	clocks, err := store.Clocks(ctx, "INC-1", base)
	if err != nil || len(clocks) != 1 || clocks[0].Lifecycle != domain.ObligationLifecycleProvisional {
		t.Fatalf("provisional clocks=%+v err=%v", clocks, err)
	}
	provisionalDeadline := clocks[0].Deadline
	version, _ := domain.ProposalVersion(proposal)
	confirmed, err := domain.Decide(proposal, version, "bob", "confirmed", base.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kernel.Append(ctx, confirmed, signer, projection); err != nil {
		t.Fatal(err)
	}
	clocks, err = store.Clocks(ctx, "INC-1", base)
	if err != nil || len(clocks) != 1 || clocks[0].Lifecycle != domain.ObligationLifecycleFirm || !clocks[0].Deadline.Equal(provisionalDeadline) {
		t.Fatalf("firm clocks=%+v err=%v", clocks, err)
	}
	var waits int
	if err := db.runtime.QueryRow(ctx, `SELECT count(*) FROM crab.timed_wait_list WHERE incident_id='INC-1' AND clock_id='CLOCK-1'`).Scan(&waits); err != nil {
		t.Fatal(err)
	}
	if waits != 3 {
		t.Fatalf("timed waits=%d want 3", waits)
	}
}

func TestRejectedTriggerRemovesActiveClockButKeepsLedgerHistory(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	kernel, signer := slice3Kernel(t, db)
	projection := ObligationProjection{}
	base := time.Date(2026, 10, 20, 4, 40, 0, 0, time.UTC)
	confirmedObligationDefinition(t, kernel, signer, "DEF-1", slice5Definition(), base)
	loc, _ := time.LoadLocation("Australia/Sydney")
	proposal, err := domain.ProposeObligationTrigger(domain.Scope{IncidentID: "INC-R", Mode: "live", Compartment: "GENERAL"}, "CLOCK-R", "alice", base.Add(2*time.Minute), domain.ObligationTrigger{DefinitionID: "DEF-1", SourceRecordID: "FACT-R", SourceField: "flagged_at", TriggerAt: time.Date(2026, 10, 23, 10, 0, 0, 0, loc), ApplicabilityReason: "Pending legal determination"})
	if err != nil {
		t.Fatal(err)
	}
	appendProposalAndDecision(t, kernel, signer, proposal, "rejected", projection)
	var clocks, waits, ledgerEntries int
	if err := db.runtime.QueryRow(ctx, `SELECT count(*) FROM crab.obligation_clocks WHERE incident_id='INC-R'`).Scan(&clocks); err != nil { t.Fatal(err) }
	if err := db.runtime.QueryRow(ctx, `SELECT count(*) FROM crab.timed_wait_list WHERE incident_id='INC-R'`).Scan(&waits); err != nil { t.Fatal(err) }
	if err := db.runtime.QueryRow(ctx, `SELECT count(*) FROM crab.ledger_entries`).Scan(&ledgerEntries); err != nil { t.Fatal(err) }
	if clocks != 0 || waits != 0 || ledgerEntries != 4 {
		t.Fatalf("clocks=%d waits=%d ledger=%d", clocks, waits, ledgerEntries)
	}
}

func TestObligationExtensionAndWaiverPreserveOriginalDeadline(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	kernel, signer := slice3Kernel(t, db)
	projection := ObligationProjection{}
	base := time.Date(2026, 10, 20, 4, 50, 0, 0, time.UTC)
	confirmedObligationDefinition(t, kernel, signer, "DEF-1", slice5Definition(), base)
	loc, _ := time.LoadLocation("Australia/Sydney")
	trigger, _ := domain.ProposeObligationTrigger(domain.Scope{IncidentID: "INC-C", Mode: "live", Compartment: "GENERAL"}, "CLOCK-C", "alice", base.Add(2*time.Minute), domain.ObligationTrigger{DefinitionID: "DEF-1", SourceRecordID: "FACT-C", SourceField: "flagged_at", TriggerAt: time.Date(2026, 10, 23, 10, 0, 0, 0, loc), ApplicabilityReason: "Rule applies"})
	appendProposalAndDecision(t, kernel, signer, trigger, "confirmed", projection)
	store, _ := NewObligationStore(db.runtime)
	before, _ := store.Clocks(ctx, "INC-C", base)
	original := before[0].OriginalDeadline
	extended := original.Add(24 * time.Hour)
	change, err := domain.ProposeObligationChange(domain.Scope{IncidentID: "INC-C", Mode: "live", Compartment: "GENERAL"}, "CHANGE-1", "alice", base.Add(4*time.Minute), domain.ObligationChange{ClockID: "CLOCK-C", Kind: "extend", AuthoritySource: "Regulator email REF-7", Reason: "Written extension granted", NewDeadline: &extended})
	if err != nil { t.Fatal(err) }
	appendProposalAndDecision(t, kernel, signer, change, "confirmed", projection)
	after, _ := store.Clocks(ctx, "INC-C", base)
	if !after[0].OriginalDeadline.Equal(original) || !after[0].Deadline.Equal(extended) || after[0].LastChangeKind != "extend" {
		t.Fatalf("extended clock=%+v", after[0])
	}
	waive, err := domain.ProposeObligationChange(domain.Scope{IncidentID: "INC-C", Mode: "live", Compartment: "GENERAL"}, "CHANGE-2", "alice", base.Add(6*time.Minute), domain.ObligationChange{ClockID: "CLOCK-C", Kind: "waive", AuthoritySource: "Regulator letter REF-8", Reason: "Obligation waived"})
	if err != nil { t.Fatal(err) }
	appendProposalAndDecision(t, kernel, signer, waive, "confirmed", projection)
	after, _ = store.Clocks(ctx, "INC-C", base)
	if after[0].Status != domain.ObligationStatusWaived || !after[0].OriginalDeadline.Equal(original) {
		t.Fatalf("waived clock=%+v", after[0])
	}
	var waits int
	if err := db.runtime.QueryRow(ctx, `SELECT count(*) FROM crab.timed_wait_list WHERE incident_id='INC-C' AND clock_id='CLOCK-C'`).Scan(&waits); err != nil { t.Fatal(err) }
	if waits != 0 { t.Fatalf("waived clock retained %d wakeups", waits) }
}

func TestExerciseClockIsSimulatedAndRebuildRestoresTimedWaits(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	kernel, signer := slice3Kernel(t, db)
	projection := ObligationProjection{}
	base := time.Date(2026, 10, 20, 5, 0, 0, 0, time.UTC)
	confirmedObligationDefinition(t, kernel, signer, "DEF-X", slice5Definition(), base)
	loc, _ := time.LoadLocation("Australia/Sydney")
	trigger, _ := domain.ProposeObligationTrigger(domain.Scope{IncidentID: "EX-1", Mode: "exercise", Compartment: "GENERAL"}, "CLOCK-X", "alice", base.Add(2*time.Minute), domain.ObligationTrigger{DefinitionID: "DEF-X", SourceRecordID: "INJECT-1", SourceField: "flagged_at", TriggerAt: time.Date(2026, 10, 23, 10, 0, 0, 0, loc), ApplicabilityReason: "Exercise inject simulates applicability"})
	appendProposalAndDecision(t, kernel, signer, trigger, "confirmed", projection)
	store, _ := NewObligationStore(db.runtime)
	clocks, _ := store.Clocks(ctx, "EX-1", base)
	if len(clocks) != 1 || !clocks[0].Simulated { t.Fatalf("exercise clock=%+v", clocks) }
	if _, err := db.runtime.Exec(ctx, `TRUNCATE crab.timed_wait_list, crab.obligation_clocks, crab.obligation_proposals, crab.obligation_definitions`); err != nil { t.Fatal(err) }
	if _, err := kernel.Rebuild(ctx, projection); err != nil { t.Fatal(err) }
	clocks, _ = store.Clocks(ctx, "EX-1", base)
	if len(clocks) != 1 || !clocks[0].Simulated { t.Fatalf("rebuilt exercise clock=%+v", clocks) }
	var waits int
	if err := db.runtime.QueryRow(ctx, `SELECT count(*) FROM crab.timed_wait_list WHERE incident_id='EX-1' AND clock_id='CLOCK-X'`).Scan(&waits); err != nil { t.Fatal(err) }
	if waits != 3 { t.Fatalf("rebuilt waits=%d", waits) }
}

func TestTimedWaitListIsBoundedAtReadBoundary(t *testing.T) {
	db := setupTestDB(t)
	store, err := NewObligationStore(db.runtime)
	if err != nil { t.Fatal(err) }
	if _, err := store.DueWakeups(context.Background(), time.Now().UTC(), 0); err == nil { t.Fatal("zero timed-wait bound accepted") }
	if _, err := store.DueWakeups(context.Background(), time.Now().UTC(), 1001); err == nil { t.Fatal("oversized timed-wait bound accepted") }
}
