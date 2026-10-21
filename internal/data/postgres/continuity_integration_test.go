package postgres

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

func TestContinuityProductFlowThroughLedgerAndPostgres(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	kernel, signer := slice3Kernel(t, db)

	incidentStore, err := NewIncidentStore(db.runtime)
	if err != nil {
		t.Fatal(err)
	}
	incidentRuntime, err := NewIncidentRuntime(kernel, signer, incidentStore)
	if err != nil {
		t.Fatal(err)
	}
	incidentService := app.IncidentService{Backend: incidentRuntime}

	continuityStore, err := NewContinuityStore(db.runtime)
	if err != nil {
		t.Fatal(err)
	}
	blobStore, err := NewBlobStore(db.runtime, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	continuityRuntime, err := NewContinuityRuntime(kernel, signer, continuityStore, blobStore)
	if err != nil {
		t.Fatal(err)
	}
	continuityService := app.ContinuityService{Backend: continuityRuntime, Incident: incidentService}

	base := time.Date(2026, 10, 21, 1, 30, 0, 0, time.UTC)
	aliceUnassigned := slice3Principal("alice", authz.RoleMajor, "INC-CONT", false)
	bobUnassigned := slice3Principal("bob", authz.RoleMajor, "INC-CONT", false)

	if _, err := continuityService.CaptureFirstReport(ctx, aliceUnassigned, "REPORT-1", domain.FirstReport{
		Reporter:        "Service desk",
		ContactRoute:    "phone 100",
		ReceivedAt:      base,
		TimeConfidence:  "trusted",
		Description:     "Payroll endpoints encrypting files",
		Affected:        "Payroll",
		ImmediateImpact: "Payroll unavailable",
		SourceChannel:   "phone",
	}, []app.FirstReportAttachmentInput{{Filename: "photo.txt", DeclaredMediaType: "text/plain", Reader: strings.NewReader("phone evidence")}}, base); err != nil {
		t.Fatal(err)
	}
	reports, err := continuityService.FirstReports(ctx, aliceUnassigned)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || len(reports[0].Report.Attachments) != 1 || reports[0].Report.Attachments[0].SHA256 == "" {
		t.Fatalf("first report=%+v", reports)
	}

	scope := domain.Scope{IncidentID: "INC-CONT", Mode: "live", Compartment: "GENERAL"}
	declaration, err := incidentService.ProposeIncidentDeclaration(ctx, aliceUnassigned, scope, "DECLARE-CONT", domain.IncidentDeclaration{Title: "Payroll ransomware", Summary: "Encryption reported on payroll endpoints", Severity: "High"}, base.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	declarationVersion, err := domain.ProposalVersion(declaration)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := incidentService.Decide(ctx, bobUnassigned, scope.IncidentID, domain.TypeIncidentDeclaration, declaration.RecordID, declarationVersion, "confirmed", base.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}

	if _, err := continuityService.TriageFirstReport(ctx, aliceUnassigned, "TRIAGE-1", domain.FirstReportDisposition{ReportID: "REPORT-1", Decision: "associate", IncidentID: scope.IncidentID, Reason: "Report matches declared payroll incident"}, base.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	reports, err = continuityService.FirstReports(ctx, aliceUnassigned)
	if err != nil || reports[0].Disposition == nil || reports[0].Disposition.IncidentID != scope.IncidentID {
		t.Fatalf("associated report=%+v err=%v", reports, err)
	}

	alice := slice3Principal("alice", authz.RoleMajor, scope.IncidentID, true)
	bob := slice3Principal("bob", authz.RoleMajor, scope.IncidentID, true)
	if _, err := continuityService.RecordScopeAssessment(ctx, alice, scope, "SCOPE-1", domain.ScopeAssessment{
		AffectedAssets:   []string{"PAY-01", "PAY-02"},
		AffectedServices: []string{"Payroll"},
		Impact:           "Payroll processing unavailable",
		Confidence:       "high",
		ReasonEvidence:   "EDR and service-desk reports agree",
	}, base.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}

	proposal, err := continuityService.ProposeSeverityChange(ctx, alice, scope, "SEVERITY-1", domain.SeverityChange{Severity: "Critical", ReasonEvidence: "Encryption spread confirmed"}, base.Add(5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	pending, err := continuityRuntime.PendingSeverityChanges(ctx, scope.IncidentID, scope.Compartment)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].RecordID != proposal.RecordID {
		t.Fatalf("pending severity=%+v", pending)
	}
	version, err := domain.ProposalVersion(pending[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := continuityService.GenerateHandover(ctx, alice, scope, "HANDOVER-1", []string{"Restore payroll processing"}, []string{"Update executive team at 03:00Z"}, base.Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	handoversWithPending, err := continuityService.Handovers(ctx, alice, scope.IncidentID, scope.Compartment)
	if err != nil || len(handoversWithPending) != 1 || len(handoversWithPending[0].Handover.PendingDecisions) != 1 || handoversWithPending[0].Handover.PendingDecisions[0] != domain.TypeSeverityChange+"/SEVERITY-1" {
		t.Fatalf("handover pending decisions=%+v err=%v", handoversWithPending, err)
	}
	if _, err := continuityService.DecideSeverity(ctx, bob, scope.IncidentID, proposal.RecordID, version, "confirmed", base.Add(7*time.Minute)); err != nil {
		t.Fatal(err)
	}
	pending, err = continuityRuntime.PendingSeverityChanges(ctx, scope.IncidentID, scope.Compartment)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending after decision=%+v err=%v", pending, err)
	}

	if _, err := continuityService.RecordResourceRequest(ctx, alice, scope, "RESOURCE-1", domain.ResourceRequest{Category: "people", Request: "Two additional IR analysts", OwnerHumanID: "alice", Priority: "P1", Status: "requested", AuthoritySource: "incident lead", Notes: "For overnight containment"}, base.Add(8*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := continuityService.RecordResourceRequest(ctx, alice, scope, "RESOURCE-1", domain.ResourceRequest{Category: "people", Request: "Two additional IR analysts", OwnerHumanID: "bob", Priority: "P1", Status: "assigned", AuthoritySource: "incident lead", Notes: "Bob coordinating surge"}, base.Add(9*time.Minute)); err != nil {
		t.Fatal(err)
	}

	scopes, err := continuityService.ScopeAssessments(ctx, alice, scope.IncidentID, scope.Compartment)
	if err != nil || len(scopes) != 1 || scopes[0].Assessment.Impact == "" {
		t.Fatalf("scopes=%+v err=%v", scopes, err)
	}
	severity, err := continuityService.SeverityChanges(ctx, alice, scope.IncidentID, scope.Compartment)
	if err != nil || len(severity) != 1 || severity[0].Change.Severity != "Critical" || severity[0].Confirmer != "bob" {
		t.Fatalf("severity=%+v err=%v", severity, err)
	}
	handovers, err := continuityService.Handovers(ctx, alice, scope.IncidentID, scope.Compartment)
	if err != nil || len(handovers) != 1 || len(handovers[0].Handover.NextObjectives) != 1 {
		t.Fatalf("handovers=%+v err=%v", handovers, err)
	}
	resources, err := continuityService.ResourceRequests(ctx, alice, scope.IncidentID, scope.Compartment)
	if err != nil || len(resources) != 1 || resources[0].Request.OwnerHumanID != "bob" || resources[0].Request.Status != "assigned" {
		t.Fatalf("resources=%+v err=%v", resources, err)
	}
	picture, found, err := incidentService.CommonOperatingPicture(ctx, alice, scope.IncidentID, scope.Compartment)
	if err != nil || !found || picture.Incident.Severity != "Critical" {
		t.Fatalf("picture=%+v found=%v err=%v", picture.Incident, found, err)
	}
}
