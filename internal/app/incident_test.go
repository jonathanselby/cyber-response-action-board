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

type memoryIncidentBackend struct {
	appended    []ledger.Record
	proposals   map[string]ledger.Record
	picture     domain.OperatingPicture
	declaration bool
}

func proposalKey(incidentID, recordType, recordID string) string {
	return incidentID + "\x00" + recordType + "\x00" + recordID
}

func (m *memoryIncidentBackend) AppendIncident(_ context.Context, record ledger.Record) error {
	m.appended = append(m.appended, record)
	if m.proposals == nil {
		m.proposals = map[string]ledger.Record{}
	}
	key := proposalKey(record.IncidentID, record.RecordType, record.RecordID)
	if record.Status == "proposed" {
		m.proposals[key] = record
	} else if record.Status == "confirmed" || record.Status == "rejected" {
		delete(m.proposals, key)
	}
	return nil
}

func (m *memoryIncidentBackend) PendingProposalRecord(_ context.Context, incidentID, recordType, recordID string) (ledger.Record, bool, error) {
	record, ok := m.proposals[proposalKey(incidentID, recordType, recordID)]
	return record, ok, nil
}
func (m *memoryIncidentBackend) CommonOperatingPicture(context.Context, string) (domain.OperatingPicture, bool, error) {
	return m.picture, true, nil
}
func (m *memoryIncidentBackend) DeclarationExists(context.Context, string) (bool, error) {
	return m.declaration, nil
}

func principal(role authz.Role, member bool) authz.Principal {
	p := authz.Principal{Authenticated: true, HumanID: "alice", Role: role, Incidents: map[string]bool{}, Compartments: map[string]map[string]bool{}, Outputs: map[authz.Action]bool{}}
	if member {
		p.Incidents["INC-1"] = true
		p.Compartments["INC-1"] = map[string]bool{"GENERAL": true}
	}
	return p
}

func TestIncidentServiceRecordAndProposeRoleBoundaries(t *testing.T) {
	backend := &memoryIncidentBackend{}
	service := IncidentService{Backend: backend}
	scope := domain.Scope{IncidentID: "INC-1", Mode: "live", Compartment: "GENERAL"}
	now := time.Now().UTC()

	minor := principal(authz.RoleMinor, true)
	if _, err := service.RecordFact(context.Background(), minor, scope, "F-1", domain.Fact{Statement: "Endpoint encrypted"}, now); err != nil {
		t.Fatalf("Minor record denied: %v", err)
	}
	if _, err := service.ProposeAction(context.Background(), minor, scope, "A-1", domain.Action{Title: "Isolate endpoint"}, now); !errors.Is(err, ErrAuthorisationDenied) {
		t.Fatalf("Minor propose err=%v", err)
	}
	observer := principal(authz.RoleObserver, true)
	if _, err := service.RecordFact(context.Background(), observer, scope, "F-2", domain.Fact{Statement: "Observation"}, now); !errors.Is(err, ErrAuthorisationDenied) {
		t.Fatalf("Observer record err=%v", err)
	}
	unassigned := principal(authz.RoleMajor, false)
	if _, err := service.RecordFact(context.Background(), unassigned, scope, "F-3", domain.Fact{Statement: "Observation"}, now); !errors.Is(err, ErrAuthorisationDenied) {
		t.Fatalf("unassigned Major record err=%v", err)
	}
}

func TestMajorMayProposeNewIncidentWithoutExistingMembership(t *testing.T) {
	backend := &memoryIncidentBackend{}
	service := IncidentService{Backend: backend}
	major := principal(authz.RoleMajor, false)
	scope := domain.Scope{IncidentID: "INC-NEW", Mode: "exercise", Compartment: "GENERAL"}
	proposal, err := service.ProposeIncidentDeclaration(context.Background(), major, scope, "DECLARE-1", domain.IncidentDeclaration{Title: "Exercise", Summary: "Tabletop", Severity: "Medium"}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Status != "proposed" || len(backend.appended) != 1 {
		t.Fatalf("proposal=%+v appended=%d", proposal, len(backend.appended))
	}
	minor := principal(authz.RoleMinor, false)
	if _, err := service.ProposeIncidentDeclaration(context.Background(), minor, domain.Scope{IncidentID: "INC-OTHER", Mode: "live", Compartment: "GENERAL"}, "DECLARE-2", domain.IncidentDeclaration{Title: "X", Summary: "Y", Severity: "Low"}, time.Now().UTC()); !errors.Is(err, ErrAuthorisationDenied) {
		t.Fatalf("Minor declaration err=%v", err)
	}
}

func TestDecideReloadsStoredProposalAndEnforcesMakerChecker(t *testing.T) {
	backend := &memoryIncidentBackend{proposals: map[string]ledger.Record{}}
	service := IncidentService{Backend: backend}
	scope := domain.Scope{IncidentID: "INC-1", Mode: "live", Compartment: "GENERAL"}
	major := principal(authz.RoleMajor, true)
	proposal, err := service.ProposeAction(context.Background(), major, scope, "A-1", domain.Action{Title: "Isolate endpoint"}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	version, _ := domain.ProposalVersion(proposal)
	if _, err := service.Decide(context.Background(), major, "INC-1", domain.TypeAction, "A-1", version, "confirmed", time.Now().UTC()); err == nil {
		t.Fatal("maker self-confirmation accepted")
	}
	confirmer := major
	confirmer.HumanID = "bob"
	decided, err := service.Decide(context.Background(), confirmer, "INC-1", domain.TypeAction, "A-1", version, "confirmed", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if decided.Maker != "alice" || decided.Confirmer != "bob" || len(backend.proposals) != 0 {
		t.Fatalf("decided=%+v pending=%d", decided, len(backend.proposals))
	}
}
