package domain

import (
	"testing"
	"time"
)

func TestFirstReportDoesNotDeclareIncident(t *testing.T) {
	now := time.Date(2026, 10, 21, 1, 0, 0, 0, time.UTC)
	r, err := RecordFirstReport("FR-1", "alice", now, FirstReport{ReceivedAt: now, TimeConfidence: "trusted", Description: "supplier reports outage", SourceChannel: "phone", TriageStatus: "new"})
	if err != nil {
		t.Fatal(err)
	}
	if r.IncidentMode != "system" || r.IncidentID != "" || r.RecordType != TypeFirstReport {
		t.Fatalf("record=%+v", r)
	}
}

func TestSeverityChangeRequiresReasonAndMakerChecker(t *testing.T) {
	now := time.Date(2026, 10, 21, 1, 0, 0, 0, time.UTC)
	scope := Scope{IncidentID: "INC-1", Mode: "live", Compartment: "GENERAL"}
	proposal, err := ProposeSeverityChange(scope, "SEV-1", "alice", now, SeverityChange{Severity: "SEV-1", ReasonEvidence: "customer-facing outage confirmed"})
	if err != nil {
		t.Fatal(err)
	}
	version, err := ProposalVersion(proposal)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decide(proposal, version, "alice", "confirmed", now.Add(time.Minute)); err == nil {
		t.Fatal("expected same-human confirmation refusal")
	}
	if _, err := Decide(proposal, version, "bob", "confirmed", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
}

func TestResourceRequestRequiresAuthorityLink(t *testing.T) {
	now := time.Date(2026, 10, 21, 1, 0, 0, 0, time.UTC)
	scope := Scope{IncidentID: "INC-1", Mode: "live", Compartment: "GENERAL"}
	_, err := RecordResourceRequest(scope, "RR-1", "alice", now, ResourceRequest{Category: "forensics", Request: "two responders", OwnerHumanID: "alice", Priority: "P1", Status: "requested"})
	if err == nil {
		t.Fatal("expected authority source requirement")
	}
}
