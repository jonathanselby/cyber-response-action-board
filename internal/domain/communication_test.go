package domain

import (
	"strings"
	"testing"
	"time"
)

func TestCommunicationIssueBindsExactDraft(t *testing.T) {
	draft := CommunicationDraft{Kind: CommunicationRegulatorNotice, Subject: "Notice", Body: "Approved material notice", Audience: []string{"regulator"}, Channels: []string{"regulator-webhook"}, SourceRefs: []string{"FACT-1"}}
	digest, err := CommunicationDraftDigest(draft)
	if err != nil {
		t.Fatal(err)
	}
	issue, err := CommunicationIssueFromDraft("DRAFT-1", digest, draft)
	if err != nil {
		t.Fatal(err)
	}
	if issue.DraftRecordID != "DRAFT-1" || issue.DraftSHA256 != digest || issue.Body != draft.Body {
		t.Fatalf("issue not exact draft binding: %+v", issue)
	}
	changed := draft
	changed.Body = "changed"
	changedDigest, _ := CommunicationDraftDigest(changed)
	if changedDigest == digest {
		t.Fatal("draft digest did not change with approved content")
	}
}

func TestCommunicationIssueRejectsContentNotMatchingBoundDraft(t *testing.T) {
	draft := CommunicationDraft{Kind: CommunicationRegulatorNotice, Subject: "Notice", Body: "Approved material notice", Audience: []string{"regulator"}, Channels: []string{"regulator-webhook"}}
	digest, err := CommunicationDraftDigest(draft)
	if err != nil {
		t.Fatal(err)
	}
	issue, err := CommunicationIssueFromDraft("DRAFT-1", digest, draft)
	if err != nil {
		t.Fatal(err)
	}
	issue.Body = "forged different body"
	if err := ValidateCommunicationIssue(issue); err == nil {
		t.Fatal("communication issue accepted content that did not match its bound draft digest")
	}
}

func TestSituationReportDraftUsesCurrentOperatingPicture(t *testing.T) {
	now := time.Date(2026, 10, 20, 6, 0, 0, 0, time.UTC)
	picture := OperatingPicture{
		Incident:         IncidentState{IncidentID: "INC-1", Mode: "live", Title: "Bafflewick", Summary: "Identity service unavailable", Severity: "High"},
		Facts:            []FactView{{RecordID: "FACT-1", Fact: Fact{Statement: "Identity service is unavailable"}}},
		Unknowns:         []UncertaintyView{{RecordID: "UNK-1", Uncertainty: Uncertainty{Kind: "unknown", Statement: "Extent of customer impact"}}},
		Decisions:        []DecisionView{{RecordID: "DEC-1", Decision: Decision{Decision: "Use alternate authentication"}}},
		Actions:          []ActionView{{RecordID: "ACT-1", Action: Action{Title: "Validate backups", OwnerHumanID: "alice", Status: "in_progress"}}},
		OperationalRoles: []OperationalRoleView{{RecordID: "ROLE-1", Assignment: OperationalRoleAssignment{RoleName: "Incident Commander", HumanID: "bob"}}},
	}
	draft, err := BuildSituationReportDraft(picture, "CAD-1", []string{"incident-team"}, []string{"local"}, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Bafflewick", "Identity service is unavailable", "Extent of customer impact", "Use alternate authentication", "Validate backups", "Incident Commander"} {
		if !strings.Contains(draft.Body, expected) {
			t.Fatalf("situation report missing current CRAB state %q: %s", expected, draft.Body)
		}
	}
	if draft.Kind != CommunicationSituationReport || draft.CadenceID != "CAD-1" || len(draft.SourceRefs) != 5 {
		t.Fatalf("unexpected situation report draft: %+v", draft)
	}
}

func TestSituationReportExerciseIsVisiblyMarked(t *testing.T) {
	picture := OperatingPicture{Incident: IncidentState{IncidentID: "INC-X", Mode: "exercise", Title: "Exercise", Summary: "Simulated", Severity: "Medium"}}
	draft, err := BuildSituationReportDraft(picture, "CAD-X", []string{"exercise-team"}, []string{"local"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(draft.Body, "EXERCISE — NOT A LIVE INCIDENT") {
		t.Fatalf("exercise marker missing: %s", draft.Body)
	}
}

func TestIncomingEnquiryRequiresAssignmentWithoutCreatingChat(t *testing.T) {
	enquiry := IncomingEnquiry{Source: "media", Subject: "Request for comment", Message: "Please respond", ReceivedAt: time.Now()}
	if err := ValidateIncomingEnquiry(enquiry); err == nil {
		t.Fatal("unassigned material enquiry accepted")
	}
	enquiry.AssignedToHumanID = "comms-lead"
	if err := ValidateIncomingEnquiry(enquiry); err != nil {
		t.Fatal(err)
	}
}
