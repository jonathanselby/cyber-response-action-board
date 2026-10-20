package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

func TestSuggestedObligationDefinitionIsInactiveUntilConfirmed(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	kernel, signer := slice3Kernel(t, db)
	proposal, err := domain.ProposeObligationDefinition("DEF-SUGGESTED", "alice", time.Now().UTC(), slice5Definition())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kernel.Append(ctx, proposal, signer, ObligationProjection{}); err != nil {
		t.Fatal(err)
	}
	var definitions, pending int
	if err := db.runtime.QueryRow(ctx, `SELECT count(*) FROM crab.obligation_definitions WHERE definition_id='DEF-SUGGESTED'`).Scan(&definitions); err != nil {
		t.Fatal(err)
	}
	if err := db.runtime.QueryRow(ctx, `SELECT count(*) FROM crab.obligation_proposals WHERE record_id='DEF-SUGGESTED'`).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if definitions != 0 || pending != 1 {
		t.Fatalf("suggested definition effective=%d pending=%d", definitions, pending)
	}
}

func TestObligationClockPreservesUntrustedTimeStatus(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	kernel, signer := slice3Kernel(t, db)
	base := time.Date(2026, 10, 20, 5, 10, 0, 0, time.UTC)
	confirmedObligationDefinition(t, kernel, signer, "DEF-U", slice5Definition(), base)
	loc, _ := time.LoadLocation("Australia/Sydney")
	proposal, err := domain.ProposeObligationTrigger(domain.Scope{IncidentID: "INC-U", Mode: "live", Compartment: "GENERAL"}, "CLOCK-U", "alice", base.Add(time.Minute), domain.ObligationTrigger{
		DefinitionID:        "DEF-U",
		SourceRecordID:      "FACT-U",
		SourceField:         "flagged_at",
		TriggerAt:           time.Date(2026, 10, 23, 10, 0, 0, 0, loc),
		ApplicabilityReason: "Clock started from an untrusted source time pending verification",
	})
	if err != nil {
		t.Fatal(err)
	}
	proposal.TimeStatus = "untrusted"
	if _, err := kernel.Append(ctx, proposal, signer, ObligationProjection{}); err != nil {
		t.Fatal(err)
	}
	store, _ := NewObligationStore(db.runtime)
	clocks, err := store.Clocks(ctx, "INC-U", base)
	if err != nil || len(clocks) != 1 {
		t.Fatalf("clocks=%+v err=%v", clocks, err)
	}
	if clocks[0].TimeStatus != "untrusted" {
		t.Fatalf("time status=%q", clocks[0].TimeStatus)
	}
}

func TestObligationReplacementPreservesOriginalRuleAndDeadline(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	kernel, signer := slice3Kernel(t, db)
	projection := ObligationProjection{}
	base := time.Date(2026, 10, 20, 5, 20, 0, 0, time.UTC)
	first := slice5Definition()
	confirmedObligationDefinition(t, kernel, signer, "DEF-OLD", first, base)
	replacement := slice5Definition()
	replacement.Name = "Replacement regulator notification"
	replacement.Duration = 48
	replacement.Unit = domain.ObligationUnitElapsedHours
	replacement.Calendar.Holidays = nil
	confirmedObligationDefinition(t, kernel, signer, "DEF-NEW", replacement, base.Add(2*time.Minute))
	loc, _ := time.LoadLocation("Australia/Sydney")
	triggerAt := time.Date(2026, 10, 23, 10, 0, 0, 0, loc)
	trigger, err := domain.ProposeObligationTrigger(domain.Scope{IncidentID: "INC-REP", Mode: "live", Compartment: "GENERAL"}, "CLOCK-REP", "alice", base.Add(4*time.Minute), domain.ObligationTrigger{DefinitionID: "DEF-OLD", SourceRecordID: "FACT-REP", SourceField: "flagged_at", TriggerAt: triggerAt, ApplicabilityReason: "Old rule applied when clock started"})
	if err != nil {
		t.Fatal(err)
	}
	appendProposalAndDecision(t, kernel, signer, trigger, "confirmed", projection)
	store, _ := NewObligationStore(db.runtime)
	before, _ := store.Clocks(ctx, "INC-REP", base)
	originalDeadline := before[0].OriginalDeadline

	change, err := domain.ProposeObligationChange(domain.Scope{IncidentID: "INC-REP", Mode: "live", Compartment: "GENERAL"}, "CHANGE-REP", "alice", base.Add(6*time.Minute), domain.ObligationChange{ClockID: "CLOCK-REP", Kind: "replace", AuthoritySource: "Legal instruction REF-9", Reason: "Replacement rule supersedes current clock", ReplacementDefinitionID: "DEF-NEW"})
	if err != nil {
		t.Fatal(err)
	}
	appendProposalAndDecision(t, kernel, signer, change, "confirmed", projection)
	after, _ := store.Clocks(ctx, "INC-REP", base)
	want, err := domain.CalculateObligationDeadline(replacement, triggerAt)
	if err != nil {
		t.Fatal(err)
	}
	if after[0].DefinitionID != "DEF-NEW" || after[0].OriginalDefinitionID != "DEF-OLD" || !after[0].OriginalDeadline.Equal(originalDeadline) || !after[0].Deadline.Equal(want.Deadline) || after[0].LastChangeKind != "replace" {
		t.Fatalf("replacement clock=%+v", after[0])
	}
}

func TestObligationScopedReadsDoNotLeakOtherCompartmentsAndCOPExcludesWaived(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	kernel, signer := slice3Kernel(t, db)
	projection := ObligationProjection{}
	base := time.Date(2026, 10, 20, 5, 30, 0, 0, time.UTC)
	confirmedObligationDefinition(t, kernel, signer, "DEF-SCOPE", slice5Definition(), base)
	loc, _ := time.LoadLocation("Australia/Sydney")
	for _, item := range []struct{ id, compartment string }{{"CLOCK-G", "GENERAL"}, {"CLOCK-L", "LEGAL"}, {"CLOCK-H", "HR"}} {
		proposal, err := domain.ProposeObligationTrigger(domain.Scope{IncidentID: "INC-SCOPE", Mode: "live", Compartment: item.compartment}, item.id, "alice", base.Add(time.Minute), domain.ObligationTrigger{DefinitionID: "DEF-SCOPE", SourceRecordID: "FACT-" + item.id, SourceField: "flagged_at", TriggerAt: time.Date(2026, 10, 23, 10, 0, 0, 0, loc), ApplicabilityReason: "Configured rule applies"})
		if err != nil {
			t.Fatal(err)
		}
		appendProposalAndDecision(t, kernel, signer, proposal, "confirmed", projection)
	}
	store, _ := NewObligationStore(db.runtime)
	general, err := store.ClocksInCompartment(ctx, "INC-SCOPE", "GENERAL", base)
	if err != nil || len(general) != 1 || general[0].ClockID != "CLOCK-G" {
		t.Fatalf("GENERAL clocks=%+v err=%v", general, err)
	}
	legal, err := store.ClocksInCompartment(ctx, "INC-SCOPE", "LEGAL", base)
	if err != nil || len(legal) != 2 {
		t.Fatalf("LEGAL clocks=%+v err=%v", legal, err)
	}
	for _, clock := range legal {
		if clock.ClockID == "CLOCK-H" {
			t.Fatal("LEGAL clock read leaked HR clock")
		}
	}
	waive, err := domain.ProposeObligationChange(domain.Scope{IncidentID: "INC-SCOPE", Mode: "live", Compartment: "LEGAL"}, "CHANGE-L", "alice", base.Add(3*time.Minute), domain.ObligationChange{ClockID: "CLOCK-L", Kind: "waive", AuthoritySource: "Legal decision REF-10", Reason: "Waived"})
	if err != nil {
		t.Fatal(err)
	}
	appendProposalAndDecision(t, kernel, signer, waive, "confirmed", projection)
	active, err := store.ActiveClocksInCompartment(ctx, "INC-SCOPE", "LEGAL", base)
	if err != nil || len(active) != 1 || active[0].ClockID != "CLOCK-G" {
		t.Fatalf("active LEGAL COP clocks=%+v err=%v", active, err)
	}
}
