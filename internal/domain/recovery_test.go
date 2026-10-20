package domain

import (
	"errors"
	"testing"
	"time"
)

func TestRecoveryPhaseIsDerivedWithoutSecondStateMachine(t *testing.T) {
	state := FinaliseRecoveryState(RecoveryState{IncidentID: "INC-1"})
	if state.Phase != PhaseActiveResponse {
		t.Fatalf("initial phase=%q", state.Phase)
	}
	state.Updates = []RecoveryUpdateView{{Update: RecoveryUpdate{ItemID: "REC-1", Summary: "Restore payroll", OwnerHumanID: "alice", RestorationPriority: "P1"}}}
	state = FinaliseRecoveryState(state)
	if state.Phase != PhaseRecovery || state.UnresolvedCount != 1 {
		t.Fatalf("recovery phase=%q unresolved=%d", state.Phase, state.UnresolvedCount)
	}
	now := time.Now().UTC()
	state.StandDownAt = &now
	state = FinaliseRecoveryState(state)
	if state.Phase != PhaseStoodDown {
		t.Fatalf("stood-down phase=%q", state.Phase)
	}
	state.PIRAt = &now
	state.RetentionAt = &now
	state = FinaliseRecoveryState(state)
	if state.ClosureReady || state.Phase != PhaseStoodDown {
		t.Fatalf("unresolved recovery must block closure ready: %+v", state)
	}
	state.Updates[0].Update.HandoffRef = "JIRA-42"
	state = FinaliseRecoveryState(state)
	if !state.ClosureReady || state.Phase != PhaseClosureReady {
		t.Fatalf("handoff should permit closure: %+v", state)
	}
	state.ClosedAt = &now
	state = FinaliseRecoveryState(state)
	if state.Phase != PhaseClosed || state.ClosureReady {
		t.Fatalf("closed state=%+v", state)
	}
}

func TestRetentionHoldRequiresReasonAndNoDeletionMechanism(t *testing.T) {
	if err := ValidateRetention(RetentionDisposition{RetentionClass: "seven-years", HandoffRef: "RM-1", Hold: true}); err == nil {
		t.Fatal("active hold without reason accepted")
	}
	if err := ValidateRetention(RetentionDisposition{RetentionClass: "seven-years", HandoffRef: "RM-1", Hold: true, HoldReason: "litigation"}); err != nil {
		t.Fatal(err)
	}
}

func TestClosurePrerequisitesRequireStandDownPIRRetentionAndResolvedRecovery(t *testing.T) {
	now := time.Now().UTC()
	state := RecoveryState{
		IncidentID:  "INC-1",
		StandDownAt: &now,
		PIRAt:       &now,
		RetentionAt: &now,
		Updates: []RecoveryUpdateView{{Update: RecoveryUpdate{
			ItemID: "REC-1", Summary: "Restore", OwnerHumanID: "alice", RestorationPriority: "P1",
		}}},
	}
	if !errors.Is(ClosurePrerequisites(state), ErrClosurePrerequisites) {
		t.Fatal("unresolved recovery work did not block closure")
	}
	state.Updates[0].Update.Verified = true
	if err := ClosurePrerequisites(state); err != nil {
		t.Fatalf("verified recovery should permit closure: %v", err)
	}
}

func TestClosureOutputsCannotLeakPrivilegedSummaryIntoShareable(t *testing.T) {
	outputs := BuildClosureOutputs(Closure{
		Reason:            "PIR confirmed and records handed off",
		PrivilegedSummary: "Legal advice: privileged text",
		ShareableSummary:  "Incident closed after recovery verification",
	})
	if outputs.Privileged.Summary != "Legal advice: privileged text" {
		t.Fatalf("privileged summary=%q", outputs.Privileged.Summary)
	}
	if outputs.Shareable.Summary != "Incident closed after recovery verification" {
		t.Fatalf("shareable summary=%q", outputs.Shareable.Summary)
	}
	if outputs.Shareable.Summary == outputs.Privileged.Summary {
		t.Fatal("shareable output leaked privileged summary")
	}
}

func TestGeneratePIRDraftUsesExistingIncidentRecord(t *testing.T) {
	picture := OperatingPicture{
		Incident: IncidentState{IncidentID: "INC-1", Title: "Payroll incident", Summary: "Payroll unavailable", Severity: "High"},
		Timeline: []TimelineEntry{{RecordID: "FACT-1", Summary: "Encryption confirmed"}},
		Decisions: []DecisionView{{RecordID: "DEC-1", Decision: Decision{Decision: "Restore clean backup"}}},
		Actions: []ActionView{{RecordID: "ACT-1", Action: Action{Title: "Restore payroll"}}},
	}
	draft := GeneratePIRDraft(picture)
	if draft.ExecutiveSummary == "" || len(draft.Timeline) != 1 || len(draft.Decisions) != 1 || len(draft.Actions) != 1 {
		t.Fatalf("PIR draft=%+v", draft)
	}
	picture.Timeline[0].Summary = "changed"
	if draft.Timeline[0].Summary == "changed" {
		t.Fatal("PIR draft aliases mutable caller slice")
	}
}

func TestRecoveryMilestonesRequireGeneralCompartment(t *testing.T) {
	scope := Scope{IncidentID: "INC-1", Mode: "live", Compartment: "LEGAL"}
	_, err := ProposeStandDown(scope, "SD-1", "alice", time.Now().UTC(), StandDown{Reason: "response complete"})
	if err == nil {
		t.Fatal("restricted recovery milestone accepted")
	}
}
