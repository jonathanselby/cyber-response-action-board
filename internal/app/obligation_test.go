package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

type memoryObligationBackend struct {
	proposals       map[string]ledger.Record
	appended        []ledger.Record
	clocks          []domain.ObligationClockView
	lastCompartment string
}

func obligationProposalKey(scopeID, recordType, recordID string) string {
	return scopeID + "\x00" + recordType + "\x00" + recordID
}

func (m *memoryObligationBackend) AppendObligation(_ context.Context, record ledger.Record) error {
	if m.proposals == nil {
		m.proposals = map[string]ledger.Record{}
	}
	scopeID := record.IncidentID
	key := obligationProposalKey(scopeID, record.RecordType, record.RecordID)
	if record.Status == "proposed" {
		m.proposals[key] = record
	} else {
		delete(m.proposals, key)
	}
	m.appended = append(m.appended, record)
	return nil
}

func (m *memoryObligationBackend) PendingObligation(_ context.Context, scopeID, recordType, recordID string) (ledger.Record, bool, error) {
	record, ok := m.proposals[obligationProposalKey(scopeID, recordType, recordID)]
	return record, ok, nil
}

func (m *memoryObligationBackend) ObligationClocks(_ context.Context, _ string, compartment string, _ time.Time) ([]domain.ObligationClockView, error) {
	m.lastCompartment = compartment
	return m.clocks, nil
}

func appObligationDefinition() domain.ObligationDefinition {
	return domain.ObligationDefinition{
		Name:                   "Customer notification",
		TriggerField:           "flagged_at",
		Duration:               24,
		Unit:                   domain.ObligationUnitElapsedHours,
		Calendar:               domain.BusinessCalendar{Timezone: "Australia/Sydney"},
		ApplicabilityCondition: "Legal lead determines rule applies",
		Recipient:              "Affected customers",
		Owner:                  "Legal lead",
		AuthoritySource:        "Response plan",
	}
}

func TestObligationServiceUsesNormalMakerCheckerAndRoleBoundary(t *testing.T) {
	backend := &memoryObligationBackend{}
	service := ObligationService{Backend: backend}
	now := time.Now().UTC()
	scope := domain.Scope{IncidentID: "INC-1", Mode: "live", Compartment: "GENERAL"}
	trigger := domain.ObligationTrigger{DefinitionID: "DEF-1", SourceRecordID: "FACT-1", SourceField: "flagged_at", TriggerAt: now, ApplicabilityReason: "Legal lead determined applicability"}

	minor := principal(authz.RoleMinor, true)
	if _, err := service.ProposeTrigger(context.Background(), minor, scope, "CLOCK-1", trigger, now); !errors.Is(err, ErrAuthorisationDenied) {
		t.Fatalf("Minor trigger proposal err=%v", err)
	}
	major := principal(authz.RoleMajor, true)
	proposal, err := service.ProposeTrigger(context.Background(), major, scope, "CLOCK-1", trigger, now)
	if err != nil {
		t.Fatal(err)
	}
	version, err := domain.ProposalVersion(proposal)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Decide(context.Background(), major, "INC-1", domain.TypeObligationTrigger, "CLOCK-1", version, "confirmed", now.Add(time.Minute)); err == nil {
		t.Fatal("obligation maker self-confirmation accepted")
	}
	confirmer := major
	confirmer.HumanID = "bob"
	decided, err := service.Decide(context.Background(), confirmer, "INC-1", domain.TypeObligationTrigger, "CLOCK-1", version, "confirmed", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if decided.Maker != "alice" || decided.Confirmer != "bob" || len(backend.proposals) != 0 {
		t.Fatalf("decided=%+v pending=%d", decided, len(backend.proposals))
	}
}

func TestObligationDefinitionRequiresMajorAndDifferentConfirmer(t *testing.T) {
	backend := &memoryObligationBackend{}
	service := ObligationService{Backend: backend}
	now := time.Now().UTC()
	minor := principal(authz.RoleMinor, true)
	if _, err := service.ProposeDefinition(context.Background(), minor, "DEF-1", appObligationDefinition(), now); !errors.Is(err, ErrAuthorisationDenied) {
		t.Fatalf("Minor definition proposal err=%v", err)
	}
	major := principal(authz.RoleMajor, true)
	proposal, err := service.ProposeDefinition(context.Background(), major, "DEF-1", appObligationDefinition(), now)
	if err != nil {
		t.Fatal(err)
	}
	version, _ := domain.ProposalVersion(proposal)
	confirmer := major
	confirmer.HumanID = "bob"
	if _, err := service.Decide(context.Background(), confirmer, "", domain.TypeObligationDefinition, "DEF-1", version, "confirmed", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
}

func TestObligationClockReadPassesAuthorisedCompartment(t *testing.T) {
	backend := &memoryObligationBackend{clocks: []domain.ObligationClockView{{ClockID: "CLOCK-1"}}}
	service := ObligationService{Backend: backend}
	major := principal(authz.RoleMajor, true)
	major.Compartments["INC-1"]["LEGAL"] = true
	clocks, err := service.Clocks(context.Background(), major, "INC-1", "LEGAL", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(clocks) != 1 || backend.lastCompartment != "LEGAL" {
		t.Fatalf("clocks=%+v compartment=%q", clocks, backend.lastCompartment)
	}
}
