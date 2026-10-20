package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

func slice3Principal(humanID string, role authz.Role, incidentID string, member bool) authz.Principal {
	p := authz.Principal{
		Authenticated: true,
		HumanID:       humanID,
		Role:          role,
		Incidents:     map[string]bool{},
		Compartments:  map[string]map[string]bool{},
		Outputs:       map[authz.Action]bool{},
	}
	if member {
		p.Incidents[incidentID] = true
		p.Compartments[incidentID] = map[string]bool{"GENERAL": true}
	}
	return p
}

func TestIncidentServiceEndToEndThroughLedgerAndPostgres(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	kernel, signer := slice3Kernel(t, db)
	store, err := NewIncidentStore(db.runtime)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewIncidentRuntime(kernel, signer, store)
	if err != nil {
		t.Fatal(err)
	}
	service := app.IncidentService{Backend: runtime}
	base := time.Date(2026, 10, 20, 3, 30, 0, 0, time.UTC)
	scope := domain.Scope{IncidentID: "INC-APP", Mode: "live", Compartment: "GENERAL"}

	aliceUnassigned := slice3Principal("alice", authz.RoleMajor, scope.IncidentID, false)
	declaration, err := service.ProposeIncidentDeclaration(ctx, aliceUnassigned, scope, "DECLARE-APP", domain.IncidentDeclaration{
		Title: "Payroll ransomware",
		Summary: "Encryption confirmed on payroll endpoints",
		Severity: "High",
	}, base)
	if err != nil {
		t.Fatal(err)
	}
	version, err := domain.ProposalVersion(declaration)
	if err != nil {
		t.Fatal(err)
	}
	bobUnassigned := slice3Principal("bob", authz.RoleMajor, scope.IncidentID, false)
	if _, err := service.Decide(ctx, bobUnassigned, scope.IncidentID, domain.TypeIncidentDeclaration, declaration.RecordID, version, "confirmed", base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	carolMinor := slice3Principal("carol", authz.RoleMinor, scope.IncidentID, true)
	if _, err := service.RecordFact(ctx, carolMinor, scope, "FACT-APP", domain.Fact{
		Statement: "Five endpoints encrypted; severity should change to Critical",
		Source: "SOC",
	}, base.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}

	alice := slice3Principal("alice", authz.RoleMajor, scope.IncidentID, true)
	action, err := service.ProposeAction(ctx, alice, scope, "ACT-APP", domain.Action{Title: "Isolate affected endpoints", OwnerHumanID: "carol"}, base.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	actionVersion, _ := domain.ProposalVersion(action)
	bob := slice3Principal("bob", authz.RoleMajor, scope.IncidentID, true)
	if _, err := service.Decide(ctx, bob, scope.IncidentID, domain.TypeAction, action.RecordID, actionVersion, "confirmed", base.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}

	picture, found, err := service.CommonOperatingPicture(ctx, carolMinor, scope.IncidentID, "GENERAL")
	if err != nil || !found {
		t.Fatalf("authorised picture found=%v err=%v", found, err)
	}
	if picture.Incident.Title != "Payroll ransomware" || picture.Incident.Severity != "High" {
		t.Fatalf("incident header=%+v", picture.Incident)
	}
	if len(picture.Facts) != 1 || len(picture.Actions) != 1 || len(picture.Pending) != 0 {
		t.Fatalf("operating picture facts=%d actions=%d pending=%d", len(picture.Facts), len(picture.Actions), len(picture.Pending))
	}

	outsider := slice3Principal("mallory", authz.RoleMajor, scope.IncidentID, false)
	if _, _, err := service.CommonOperatingPicture(ctx, outsider, scope.IncidentID, "GENERAL"); !errors.Is(err, app.ErrAuthorisationDenied) {
		t.Fatalf("unassigned Major read err=%v", err)
	}
}
