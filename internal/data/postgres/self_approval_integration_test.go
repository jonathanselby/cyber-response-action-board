package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/adapters/clock"
	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

type openWindow struct{ human, kind string }

func (w openWindow) Permits(_ context.Context, human, kind string, _ time.Time) (bool, error) {
	return human == w.human && kind == w.kind, nil
}

// T-210: a self-approval made while the clock is unverified carries both the
// self-approval flag and untrusted time, in the ledger and on the timeline.
// Without a window the maker still cannot decide (T-173 unchanged).
func TestSelfApprovalUnderUnverifiedTime(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	kernel, signer := slice3Kernel(t, db)
	kernel.SetClock(clock.Fixed{Status: clock.Untrusted})
	store, err := NewIncidentStore(db.runtime)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewIncidentRuntime(kernel, signer, store)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 10, 26, 3, 0, 0, 0, time.UTC)
	scope := domain.Scope{IncidentID: "INC-SELF", Mode: "exercise", Compartment: "GENERAL"}
	plain := app.IncidentService{Backend: runtime}
	declaration, err := plain.ProposeIncidentDeclaration(ctx, slice3Principal("alice", authz.RoleMajor, scope.IncidentID, false), scope, "DECLARE-SELF", domain.IncidentDeclaration{Title: "T", Summary: "S", Severity: "Severity level 2"}, base)
	if err != nil {
		t.Fatal(err)
	}
	version, _ := domain.ProposalVersion(declaration)
	if _, err := plain.Decide(ctx, slice3Principal("bob", authz.RoleMajor, scope.IncidentID, false), scope.IncidentID, domain.TypeIncidentDeclaration, declaration.RecordID, version, "confirmed", base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	alice := slice3Principal("alice", authz.RoleMajor, scope.IncidentID, true)
	proposal, err := plain.ProposeDecision(ctx, alice, scope, "DEC-SELF", domain.Decision{Decision: "Isolate payroll"}, base.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	version, _ = domain.ProposalVersion(proposal)
	selfCtx := app.WithSelfApproval(ctx, "Only Major available")
	if _, err := plain.Decide(selfCtx, alice, scope.IncidentID, domain.TypeDecision, proposal.RecordID, version, "confirmed", base.Add(3*time.Minute)); err == nil {
		t.Fatal("self-approval succeeded without a window")
	}
	windowed := app.IncidentService{Backend: runtime, Window: openWindow{"alice", domain.TypeDecision}}
	if _, err := windowed.Decide(ctx, alice, scope.IncidentID, domain.TypeDecision, proposal.RecordID, version, "confirmed", base.Add(3*time.Minute)); err == nil {
		t.Fatal("maker decided without asking for self-approval")
	}
	decided, err := windowed.Decide(selfCtx, alice, scope.IncidentID, domain.TypeDecision, proposal.RecordID, version, "confirmed", base.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if !decided.SelfApproval || decided.SelfApprovalReason == "" || decided.Confirmer != "alice" {
		t.Fatalf("decided record not flagged: %+v", decided)
	}
	var governance, timeStatus string
	if err := db.runtime.QueryRow(ctx, `SELECT governance,time_status FROM crab.incident_timeline WHERE record_id='DEC-SELF' ORDER BY sequence DESC LIMIT 1`).Scan(&governance, &timeStatus); err != nil {
		t.Fatal(err)
	}
	if governance != "self-approved" || timeStatus != ledger.TimeUntrusted {
		t.Fatalf("timeline governance=%s time=%s", governance, timeStatus)
	}
	if res, err := kernel.Verify(ctx); err != nil || !res.Valid {
		t.Fatalf("ledger invalid: %+v %v", res, err)
	}
}
