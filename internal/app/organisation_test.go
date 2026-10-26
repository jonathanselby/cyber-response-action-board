package app

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

type fakeOrganisationBackend struct {
	appended []ledger.Record
	current  map[string][]domain.OrganisationView
}

func (f *fakeOrganisationBackend) AppendOrganisation(_ context.Context, r ledger.Record) error {
	f.appended = append(f.appended, r)
	return nil
}
func (f *fakeOrganisationBackend) PendingOrganisation(context.Context, string) (ledger.Record, bool, error) {
	return ledger.Record{}, false, nil
}
func (f *fakeOrganisationBackend) PendingOrganisationList(context.Context) ([]domain.OrganisationView, error) {
	return nil, nil
}
func (f *fakeOrganisationBackend) CurrentOrganisation(_ context.Context, t string) ([]domain.OrganisationView, error) {
	return f.current[t], nil
}

// T-202 / T-203: only Majors propose policy; contacts are readable by Majors
// and Custodians only; severity must come from the confirmed scale.
func TestOrganisationServiceAuthority(t *testing.T) {
	raw, _ := json.Marshal(domain.EmergencyConfiguration{Organisation: "Org", Contacts: []domain.EmergencyContact{{Name: "A", Role: "R", Primary: "1"}}})
	scale, _ := json.Marshal(domain.DefaultSeverityScale())
	backend := &fakeOrganisationBackend{current: map[string][]domain.OrganisationView{
		domain.TypeEmergencyConfiguration: {{Payload: raw}},
		domain.TypeSeverityScale:          {{Payload: scale}},
	}}
	s := OrganisationService{Backend: backend}
	ctx := context.Background()
	major := authz.Principal{Authenticated: true, HumanID: "alice", Role: authz.RoleMajor}
	minor := authz.Principal{Authenticated: true, HumanID: "mina", Role: authz.RoleMinor}
	custodianMinor := authz.Principal{Authenticated: true, HumanID: "cara", Role: authz.RoleMinor, Custodian: true}

	if _, err := s.ProposeSeverityScale(ctx, minor, "S-1", domain.DefaultSeverityScale(), time.Now()); !errors.Is(err, ErrAuthorisationDenied) {
		t.Fatalf("Minor proposed policy: %v", err)
	}
	if _, err := s.ProposeSeverityScale(ctx, major, "S-2", domain.DefaultSeverityScale(), time.Now()); err != nil || len(backend.appended) != 1 {
		t.Fatalf("Major proposal: %v", err)
	}
	if _, _, err := s.EmergencyConfiguration(ctx, minor); !errors.Is(err, ErrAuthorisationDenied) {
		t.Fatalf("Minor read contacts: %v", err)
	}
	if c, view, err := s.EmergencyConfiguration(ctx, custodianMinor); err != nil || view == nil || c.Organisation != "Org" {
		t.Fatalf("Custodian read: %v", err)
	}
	if err := s.CheckSeverity(ctx, "Severity level 3"); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckSeverity(ctx, "P1"); err == nil {
		t.Fatal("value outside the scale accepted")
	}
	if err := (OrganisationService{Backend: &fakeOrganisationBackend{}}).CheckSeverity(ctx, "anything"); !errors.Is(err, ErrSeverityScaleMissing) {
		t.Fatalf("missing scale: %v", err)
	}
}
