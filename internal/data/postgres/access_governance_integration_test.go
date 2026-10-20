package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/access"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

func TestAccessGovernanceSponsorControllerCustodianAndExpiry(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	keys := newAccessKeys(t)
	kernel, err := OpenKernel(ctx, db.runtime, keys.keyring)
	if err != nil {
		t.Fatal(err)
	}
	signer := Signer{KeyID: "server", PrivateKey: keys.serverPriv}
	base := time.Date(2026, 10, 20, 5, 30, 0, 0, time.UTC)
	scopeGeneral := domain.Scope{IncidentID: "INC-ACCESS", Mode: "live", Compartment: "GENERAL"}

	declaration, err := domain.ProposeIncidentDeclaration(scopeGeneral, "DECLARE-ACCESS", "sponsor", base, domain.IncidentDeclaration{Title: "Access fixture", Summary: "Scoped-access integration fixture", Severity: "High"})
	if err != nil {
		t.Fatal(err)
	}
	appendProposalAndDecision(t, kernel, signer, declaration, "confirmed", IncidentProjection{})

	projection := AccessProjection{}
	appendLegacy := func(recordType, recordID string, payload any) {
		t.Helper()
		record := confirmedAccessRecord(t, keys, recordType, recordID, "", "system", "SYSTEM", payload)
		if _, err := kernel.Append(ctx, record, signer, projection); err != nil {
			t.Fatal(err)
		}
	}
	appendLegacy(access.RecordRoleEligibility, "ROLE-SPONSOR", map[string]any{"human_id": "sponsor", "role": "Major", "active": true})
	appendLegacy(access.RecordRoleEligibility, "ROLE-EXTERNAL", map[string]any{"human_id": "external", "role": "Minor", "active": true})
	appendLegacy(access.RecordCustodianDesignation, "CUSTODIAN-1", map[string]any{"human_id": "custodian", "active": true})

	store, err := NewAccessStore(db.runtime)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewAccessRuntime(kernel, signer, store)
	if err != nil {
		t.Fatal(err)
	}
	service := app.AccessService{Backend: runtime}
	sponsorPrincipal, _ := store.PrincipalAt(ctx, "sponsor", true, base)
	custodianPrincipal, _ := store.PrincipalAt(ctx, "custodian", true, base)

	sponsorProposal, err := service.ProposeSponsorDesignation(ctx, sponsorPrincipal, scopeGeneral, "SPONSOR-1", access.IncidentSponsorDesignation{HumanID: "sponsor", IncidentID: scopeGeneral.IncidentID, Active: true}, base.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	version, _ := domain.ProposalVersion(sponsorProposal)
	if _, err := service.Decide(ctx, custodianPrincipal, scopeGeneral.IncidentID, access.RecordIncidentSponsor, sponsorProposal.RecordID, version, "confirmed", base.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}

	expires := base.Add(3 * time.Hour)
	membershipProposal, err := service.ProposeIncidentMembership(ctx, sponsorPrincipal, scopeGeneral, "MEMBER-SPONSOR", access.IncidentMembershipChange{HumanID: "sponsor", IncidentID: scopeGeneral.IncidentID, Active: true, ExpiresAt: &expires, Reason: "Incident lead"}, base.Add(4*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	version, _ = domain.ProposalVersion(membershipProposal)
	if _, err := service.Decide(ctx, custodianPrincipal, scopeGeneral.IncidentID, access.RecordIncidentMembership, membershipProposal.RecordID, version, "confirmed", base.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}

	externalExpiry := base.Add(2 * time.Hour)
	externalProposal, err := service.ProposeIncidentMembership(ctx, sponsorPrincipal, scopeGeneral, "MEMBER-EXTERNAL", access.IncidentMembershipChange{HumanID: "external", IncidentID: scopeGeneral.IncidentID, Active: true, ExpiresAt: &externalExpiry, Reason: "External forensic responder"}, base.Add(6*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	version, _ = domain.ProposalVersion(externalProposal)
	if _, err := service.Decide(ctx, custodianPrincipal, scopeGeneral.IncidentID, access.RecordIncidentMembership, externalProposal.RecordID, version, "confirmed", base.Add(7*time.Minute)); err != nil {
		t.Fatal(err)
	}

	scopeLegal := domain.Scope{IncidentID: scopeGeneral.IncidentID, Mode: "live", Compartment: "LEGAL"}
	controllerProposal, err := service.ProposeControllerDesignation(ctx, sponsorPrincipal, scopeLegal, "CONTROLLER-1", access.CompartmentControllerDesignation{HumanID: "sponsor", IncidentID: scopeLegal.IncidentID, Compartment: "LEGAL", Active: true}, base.Add(8*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	version, _ = domain.ProposalVersion(controllerProposal)
	if _, err := service.Decide(ctx, custodianPrincipal, scopeGeneral.IncidentID, access.RecordCompartmentController, controllerProposal.RecordID, version, "confirmed", base.Add(9*time.Minute)); err != nil {
		t.Fatal(err)
	}

	legalExpiry := base.Add(time.Hour)
	legalProposal, err := service.ProposeCompartmentMembership(ctx, sponsorPrincipal, scopeLegal, "LEGAL-EXTERNAL", access.CompartmentMembershipChange{HumanID: "external", IncidentID: scopeLegal.IncidentID, Compartment: "LEGAL", Active: true, ExpiresAt: &legalExpiry, Reason: "Counsel-supervised forensic work"}, base.Add(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	version, _ = domain.ProposalVersion(legalProposal)
	if _, err := service.Decide(ctx, custodianPrincipal, scopeGeneral.IncidentID, access.RecordCompartmentMembership, legalProposal.RecordID, version, "confirmed", base.Add(11*time.Minute)); err != nil {
		t.Fatal(err)
	}

	active, err := store.PrincipalAt(ctx, "external", true, base.Add(30*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if d := authz.Authorise(active, authz.ActionRead, authz.ObjectScope{IncidentID: scopeGeneral.IncidentID, Compartment: "LEGAL"}); !d.Allowed {
		t.Fatalf("active external responder denied LEGAL: %+v", d)
	}
	options, err := app.SelectiveReleaseCompartments(active, scopeGeneral.IncidentID)
	if err != nil || len(options) != 2 || options[0] != "GENERAL" || options[1] != "LEGAL" {
		t.Fatalf("selective release options=%v err=%v", options, err)
	}

	legalExpired, _ := store.PrincipalAt(ctx, "external", true, base.Add(90*time.Minute))
	if !legalExpired.Incidents[scopeGeneral.IncidentID] || legalExpired.Compartments[scopeGeneral.IncidentID]["LEGAL"] {
		t.Fatalf("restricted expiry did not narrow access: %+v", legalExpired)
	}
	incidentExpired, _ := store.PrincipalAt(ctx, "external", true, base.Add(150*time.Minute))
	if incidentExpired.Incidents[scopeGeneral.IncidentID] || incidentExpired.Compartments[scopeGeneral.IncidentID] != nil {
		t.Fatalf("incident expiry did not remove live access: %+v", incidentExpired)
	}

	adminView, err := service.Administration(ctx, custodianPrincipal, base.Add(150*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(adminView.Sponsors) != 1 || len(adminView.Controllers) != 1 || len(adminView.IncidentMemberships) != 2 {
		t.Fatalf("metadata-only access view=%+v", adminView)
	}
	foundExpired := false
	for _, membership := range adminView.IncidentMemberships {
		if membership.HumanID == "external" && membership.Status == "expired" {
			foundExpired = true
		}
	}
	if !foundExpired {
		t.Fatalf("expired external grant not visible in access metadata: %+v", adminView.IncidentMemberships)
	}
}
