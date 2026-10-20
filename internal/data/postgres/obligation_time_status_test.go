package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

func TestPendingObligationRuntimePreservesUntrustedTimeStatus(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	kernel, signer := slice3Kernel(t, db)
	base := time.Date(2026, 10, 20, 5, 40, 0, 0, time.UTC)
	confirmedObligationDefinition(t, kernel, signer, "DEF-TIME", slice5Definition(), base)
	loc, _ := time.LoadLocation("Australia/Sydney")
	proposal, err := domain.ProposeObligationTrigger(domain.Scope{IncidentID: "INC-TIME-STATUS", Mode: "live", Compartment: "GENERAL"}, "CLOCK-TIME", "alice", base.Add(time.Minute), domain.ObligationTrigger{
		DefinitionID:        "DEF-TIME",
		SourceRecordID:      "FACT-TIME",
		SourceField:         "flagged_at",
		TriggerAt:           time.Date(2026, 10, 23, 10, 0, 0, 0, loc),
		ApplicabilityReason: "Source time is provisional pending independent verification",
	})
	if err != nil {
		t.Fatal(err)
	}
	proposal.TimeStatus = "untrusted"
	if _, err := kernel.Append(ctx, proposal, signer, ObligationProjection{}); err != nil {
		t.Fatal(err)
	}
	store, err := NewObligationStore(db.runtime)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewObligationRuntime(kernel, signer, store)
	if err != nil {
		t.Fatal(err)
	}
	pending, found, err := runtime.PendingObligation(ctx, "INC-TIME-STATUS", domain.TypeObligationTrigger, "CLOCK-TIME")
	if err != nil || !found {
		t.Fatalf("pending found=%v err=%v", found, err)
	}
	if pending.TimeStatus != "untrusted" {
		t.Fatalf("pending time status=%q", pending.TimeStatus)
	}
	version, err := domain.ProposalVersion(pending)
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := domain.Decide(pending, version, "bob", "confirmed", base.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.AppendObligation(ctx, confirmed); err != nil {
		t.Fatal(err)
	}
	clocks, err := store.Clocks(ctx, "INC-TIME-STATUS", base)
	if err != nil || len(clocks) != 1 {
		t.Fatalf("clocks=%+v err=%v", clocks, err)
	}
	if clocks[0].Lifecycle != domain.ObligationLifecycleFirm || clocks[0].TimeStatus != "untrusted" {
		t.Fatalf("confirmed clock=%+v", clocks[0])
	}
}
