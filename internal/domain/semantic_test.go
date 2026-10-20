package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func scope() Scope {
	return Scope{IncidentID: "INC-001", Mode: "live", Compartment: "GENERAL"}
}

func TestRecordFactIsImmediateAndCannotCarryGovernedState(t *testing.T) {
	record, err := RecordFact(scope(), "FACT-1", "alice", time.Date(2026, 10, 20, 2, 20, 0, 0, time.UTC), Fact{
		Statement: "Observed encryption on two workstations; severity should change to Critical",
		Source:    "service desk",
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.WriteSemantic != "record" || record.Status != "recorded" || record.Confirmer != "" {
		t.Fatalf("immediate record envelope=%+v", record)
	}
	if record.RecordType != TypeFact {
		t.Fatalf("record type=%q", record.RecordType)
	}
	var raw map[string]any
	if err := json.Unmarshal(record.Data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["severity"]; ok {
		t.Fatal("fact payload unexpectedly carries controlled severity field")
	}
}

func TestGovernedProposalCapturesExactPayloadVersion(t *testing.T) {
	proposal, err := ProposeAction(scope(), "ACT-1", "alice", time.Date(2026, 10, 20, 2, 20, 0, 0, time.UTC), Action{
		Title: "Isolate affected endpoints", OwnerHumanID: "bob", Status: ActionStatusNotStarted, Priority: "P1", SourceRef: "DEC-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	version, err := ProposalVersion(proposal)
	if err != nil {
		t.Fatal(err)
	}
	if version == "" {
		t.Fatal("proposal version missing")
	}
	if _, err := Decide(proposal, "wrong-version", "carol", "confirmed", time.Date(2026, 10, 20, 2, 21, 0, 0, time.UTC)); err == nil {
		t.Fatal("changed proposal version accepted")
	}
	confirmed, err := Decide(proposal, version, "carol", "confirmed", time.Date(2026, 10, 20, 2, 21, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.Status != "confirmed" || confirmed.Maker != "alice" || confirmed.Confirmer != "carol" {
		t.Fatalf("confirmed envelope=%+v", confirmed)
	}
	if confirmed.MakerSignature.MissingReason == "" {
		t.Fatal("final decision lost provenance of maker proposal signature")
	}
}

func TestDecideEnforcesDifferentHuman(t *testing.T) {
	proposal, err := ProposeDecision(scope(), "DEC-1", "alice", time.Now().UTC(), Decision{Decision: "Contain before eradication", Rationale: "Preserve evidence"})
	if err != nil {
		t.Fatal(err)
	}
	version, _ := ProposalVersion(proposal)
	if _, err := Decide(proposal, version, "alice", "confirmed", time.Now().UTC()); err == nil {
		t.Fatal("maker self-confirmation accepted")
	}
}

func TestUnknownAndHypothesisRemainDistinct(t *testing.T) {
	unknown, err := RecordUncertaintyItem(scope(), "UNK-1", "alice", time.Now().UTC(), Uncertainty{Kind: "unknown", Statement: "Initial access vector"})
	if err != nil {
		t.Fatal(err)
	}
	hypothesis, err := RecordUncertaintyItem(scope(), "HYP-1", "alice", time.Now().UTC(), Uncertainty{Kind: "hypothesis", Statement: "Credential stuffing preceded access"})
	if err != nil {
		t.Fatal(err)
	}
	u, err := DecodePayload[Uncertainty](unknown)
	if err != nil {
		t.Fatal(err)
	}
	h, err := DecodePayload[Uncertainty](hypothesis)
	if err != nil {
		t.Fatal(err)
	}
	if u.Kind != "unknown" || h.Kind != "hypothesis" {
		t.Fatalf("uncertainty kinds unknown=%q hypothesis=%q", u.Kind, h.Kind)
	}
}

func TestFactKeepsOccurrenceFlaggingRecordingAndAwarenessDistinct(t *testing.T) {
	occurred := time.Date(2026, 10, 20, 1, 0, 0, 0, time.UTC)
	flagged := occurred.Add(10 * time.Minute)
	recorded := flagged.Add(5 * time.Minute)
	aware := flagged.Add(2 * time.Minute)
	record, err := RecordFact(scope(), "FACT-TIME", "alice", recorded, Fact{
		Statement:  "Payroll files encrypted",
		OccurredAt: &occurred,
		FlaggedAt:  &flagged,
		Awareness:  []AwarenessTime{{Subject: "Incident Major", At: aware}},
	})
	if err != nil {
		t.Fatal(err)
	}
	fact, err := DecodePayload[Fact](record)
	if err != nil {
		t.Fatal(err)
	}
	if fact.OccurredAt == nil || !fact.OccurredAt.Equal(occurred) || fact.FlaggedAt == nil || !fact.FlaggedAt.Equal(flagged) || record.Timestamp != recorded || len(fact.Awareness) != 1 || !fact.Awareness[0].At.Equal(aware) {
		t.Fatalf("time concepts collapsed record=%+v fact=%+v", record, fact)
	}
}

func TestSemanticProposalSchemasRemainDedicated(t *testing.T) {
	now := time.Now().UTC()
	declaration, err := ProposeIncidentDeclaration(scope(), "INC-001", "alice", now, IncidentDeclaration{Title: "Payroll ransomware", Summary: "Encryption reported in payroll workstations", Severity: "High"})
	if err != nil {
		t.Fatal(err)
	}
	action, err := ProposeAction(scope(), "ACT-1", "alice", now, Action{
		Title: "Preserve endpoint images", OwnerHumanID: "bob", Status: ActionStatusNotStarted, Priority: "P1", SourceRef: "RUNBOOK-IR",
	})
	if err != nil {
		t.Fatal(err)
	}
	role, err := ProposeOperationalRoleAssignment(scope(), "ROLE-1", "alice", now, OperationalRoleAssignment{RoleName: "Technical Lead", HumanID: "bob"})
	if err != nil {
		t.Fatal(err)
	}
	if declaration.RecordType == action.RecordType || action.RecordType == role.RecordType || declaration.RecordType == role.RecordType {
		t.Fatal("semantic commands collapsed into generic item type")
	}
}
