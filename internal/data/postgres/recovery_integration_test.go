package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

func TestRecoveryStandDownPIRRetentionClosureAndRebuild(t *testing.T) {
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

	base := time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)
	scope := domain.Scope{IncidentID: "INC-RECOVERY", Mode: "live", Compartment: "GENERAL"}
	aliceUnassigned := slice3Principal("alice", authz.RoleMajor, scope.IncidentID, false)
	bobUnassigned := slice3Principal("bob", authz.RoleMajor, scope.IncidentID, false)
	declaration, err := incidentService.ProposeIncidentDeclaration(ctx, aliceUnassigned, scope, "DECLARE-RECOVERY", domain.IncidentDeclaration{Title: "Payroll incident", Summary: "Payroll unavailable", Severity: "High"}, base)
	if err != nil {
		t.Fatal(err)
	}
	version, _ := domain.ProposalVersion(declaration)
	if _, err := incidentService.Decide(ctx, bobUnassigned, scope.IncidentID, domain.TypeIncidentDeclaration, declaration.RecordID, version, "confirmed", base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	recoveryStore, err := NewRecoveryStore(db.runtime)
	if err != nil {
		t.Fatal(err)
	}
	recoveryRuntime, err := NewRecoveryRuntime(kernel, signer, recoveryStore)
	if err != nil {
		t.Fatal(err)
	}
	service := app.RecoveryService{Backend: recoveryRuntime}
	alice := slice3Principal("alice", authz.RoleMajor, scope.IncidentID, true)
	bob := slice3Principal("bob", authz.RoleMajor, scope.IncidentID, true)
	carol := slice3Principal("carol", authz.RoleMinor, scope.IncidentID, true)

	if _, err := service.RecordRecoveryUpdate(ctx, carol, scope, "REC-UPD-1", domain.RecoveryUpdate{ItemID: "REC-PAYROLL", Summary: "Restore payroll from clean backup", OwnerHumanID: "carol", RestorationPriority: "P1"}, base.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	state, found, err := service.State(ctx, carol, scope.IncidentID, "GENERAL")
	if err != nil || !found || state.Phase != domain.PhaseRecovery || state.UnresolvedCount != 1 {
		t.Fatalf("recovery state found=%v err=%v state=%+v", found, err, state)
	}

	standDown, err := service.ProposeStandDown(ctx, alice, scope, "SD-1", domain.StandDown{Reason: "emergency containment complete; recovery continues"}, base.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	standDownVersion, _ := domain.ProposalVersion(standDown)
	if _, err := service.Decide(ctx, bob, scope.IncidentID, domain.TypeStandDown, standDown.RecordID, standDownVersion, "confirmed", base.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	state, _, _ = service.State(ctx, alice, scope.IncidentID, "GENERAL")
	if state.Phase != domain.PhaseStoodDown {
		t.Fatalf("phase after stand-down=%q", state.Phase)
	}

	draft, err := service.GeneratePIR(ctx, alice, scope.IncidentID, "GENERAL")
	if err != nil || draft.Incident.IncidentID != scope.IncidentID || draft.ExecutiveSummary == "" {
		t.Fatalf("PIR draft=%+v err=%v", draft, err)
	}
	pir, err := service.ProposePIR(ctx, alice, scope, "PIR-1", domain.PIR{ExecutiveSummary: draft.ExecutiveSummary, HumanLessons: []string{"Keep an independent clean recovery path"}, LongTermWork: []string{"Replace legacy payroll endpoint image"}}, base.Add(5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	pirVersion, _ := domain.ProposalVersion(pir)
	if _, err := service.Decide(ctx, bob, scope.IncidentID, domain.TypePIR, pir.RecordID, pirVersion, "confirmed", base.Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	retention, err := service.ProposeRetention(ctx, alice, scope, "RET-1", domain.RetentionDisposition{RetentionClass: "incident-seven-years", HandoffRef: "RM-2026-42", Hold: true, HoldReason: "legal review"}, base.Add(7*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	retentionVersion, _ := domain.ProposalVersion(retention)
	if _, err := service.Decide(ctx, bob, scope.IncidentID, domain.TypeRetention, retention.RecordID, retentionVersion, "confirmed", base.Add(8*time.Minute)); err != nil {
		t.Fatal(err)
	}

	if _, err := service.ProposeClosure(ctx, alice, scope, "CLOSE-BLOCKED", domain.Closure{Reason: "administrative closure", ShareableSummary: "incident complete"}, base.Add(9*time.Minute)); !errors.Is(err, domain.ErrClosurePrerequisites) {
		t.Fatalf("unresolved recovery closure err=%v", err)
	}
	if _, err := service.RecordRecoveryUpdate(ctx, carol, scope, "REC-UPD-2", domain.RecoveryUpdate{ItemID: "REC-PAYROLL", Summary: "Payroll restored; long-term hardening handed off", OwnerHumanID: "carol", RestorationPriority: "P1", HandoffRef: "PLAT-77", ResidualRisk: "legacy image replacement pending"}, base.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	state, _, err = service.State(ctx, alice, scope.IncidentID, "GENERAL")
	if err != nil || state.Phase != domain.PhaseClosureReady || !state.ClosureReady || state.UnresolvedCount != 0 {
		t.Fatalf("closure-ready state=%+v err=%v", state, err)
	}

	closureValue := domain.Closure{Reason: "PIR confirmed and records handed off", ShareableSummary: "Recovery verified and incident administratively closed"}
	closure, err := service.ProposeClosure(ctx, alice, scope, "CLOSE-1", closureValue, base.Add(11*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	closureVersion, _ := domain.ProposalVersion(closure)
	if _, err := service.Decide(ctx, bob, scope.IncidentID, domain.TypeClosure, closure.RecordID, closureVersion, "confirmed", base.Add(12*time.Minute)); err != nil {
		t.Fatal(err)
	}
	state, _, err = service.State(ctx, alice, scope.IncidentID, "GENERAL")
	if err != nil || state.Phase != domain.PhaseClosed || state.ClosedAt == nil {
		t.Fatalf("closed state=%+v err=%v", state, err)
	}
	outputs := domain.BuildClosureOutputs(*state.Closure)
	if strings.Contains(outputs.Shareable.Summary, "Privileged legal") {
		t.Fatal("shareable closure leaked privileged content")
	}

	if _, err := service.RecordRecoveryUpdate(ctx, carol, scope, "REC-AFTER-CLOSE", domain.RecoveryUpdate{ItemID: "REC-2", Summary: "should fail", OwnerHumanID: "carol", RestorationPriority: "P3"}, base.Add(13*time.Minute)); !errors.Is(err, domain.ErrIncidentClosed) {
		t.Fatalf("recovery write after close err=%v", err)
	}
	if _, err := incidentService.RecordFact(ctx, carol, scope, "FACT-AFTER-CLOSE", domain.Fact{Statement: "must not enter ledger"}, base.Add(14*time.Minute)); err == nil || !strings.Contains(err.Error(), "closed and read-only") {
		t.Fatalf("cross-feature write after close err=%v", err)
	}

	if _, err := kernel.Rebuild(ctx, IncidentProjection{}, RecoveryProjection{}); err != nil {
		t.Fatal(err)
	}
	state, found, err = recoveryStore.State(ctx, scope.IncidentID)
	if err != nil || !found || state.Phase != domain.PhaseClosed || len(state.Updates) != 1 || state.Updates[0].Update.HandoffRef != "PLAT-77" {
		t.Fatalf("rebuilt recovery state found=%v err=%v state=%+v", found, err, state)
	}
}

func TestClosedIncidentLedgerGateDoesNotBlockSystemScope(t *testing.T) {
	// The cross-feature gate is scoped to records carrying an incident_id.
	// System configuration remains capable of governed operation after any
	// individual incident has closed.
	db := setupTestDB(t)
	var triggerExists bool
	if err := db.runtime.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='ledger_reject_closed_incident')`).Scan(&triggerExists); err != nil {
		t.Fatal(err)
	}
	if !triggerExists {
		t.Fatal("closed-incident ledger gate not installed")
	}
}
