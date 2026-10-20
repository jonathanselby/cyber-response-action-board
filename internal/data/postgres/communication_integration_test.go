package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

type postgresTestNotifier struct {
	mu       sync.Mutex
	calls    []string
	attempts map[string]app.NotificationAttempt
}

func (n *postgresTestNotifier) Deliver(_ context.Context, channel app.NotificationChannel, _ app.NotificationMessage) app.NotificationAttempt {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls = append(n.calls, channel.Name)
	if value, ok := n.attempts[channel.Name]; ok {
		return value
	}
	return app.NotificationAttempt{Status: domain.NotificationDelivered}
}

func (n *postgresTestNotifier) callNames() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.calls...)
}

func communicationTestPrincipal(human string, role authz.Role, incidentID string, notify bool) authz.Principal {
	outputs := map[authz.Action]bool{}
	if notify {
		outputs[authz.ActionNotify] = true
	}
	return authz.Principal{
		Authenticated: true,
		HumanID:       human,
		Role:          role,
		Incidents:     map[string]bool{incidentID: true},
		Compartments:  map[string]map[string]bool{incidentID: {"GENERAL": true}},
		Outputs:       outputs,
	}
}

func newCommunicationRuntime(t *testing.T, db *testDB) (*CommunicationRuntime, *Kernel, Signer) {
	t.Helper()
	kernel, signer := slice3Kernel(t, db)
	store, err := NewCommunicationStore(db.runtime)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewCommunicationRuntime(kernel, signer, store)
	if err != nil {
		t.Fatal(err)
	}
	return runtime, kernel, signer
}

func establishCommunicationIncident(t *testing.T, db *testDB, kernel *Kernel, signer Signer, incidentID string, now time.Time) app.IncidentService {
	t.Helper()
	store, err := NewIncidentStore(db.runtime)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewIncidentRuntime(kernel, signer, store)
	if err != nil {
		t.Fatal(err)
	}
	service := app.IncidentService{Backend: runtime}
	scope := domain.Scope{IncidentID: incidentID, Mode: "live", Compartment: "GENERAL"}
	maker := authz.Principal{Authenticated: true, HumanID: "declarer", Role: authz.RoleMajor}
	confirmer := authz.Principal{Authenticated: true, HumanID: "declaration-confirmer", Role: authz.RoleMajor}
	proposal, err := service.ProposeIncidentDeclaration(context.Background(), maker, scope, "DECLARE-COMM", domain.IncidentDeclaration{Title: "Bafflewick", Summary: "Identity service unavailable", Severity: "High"}, now)
	if err != nil {
		t.Fatal(err)
	}
	version, err := domain.ProposalVersion(proposal)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Decide(context.Background(), confirmer, incidentID, domain.TypeIncidentDeclaration, proposal.RecordID, version, "confirmed", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	return service
}

func TestCommunicationIssueCadenceNotificationAndRebuild(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	base := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	incidentID := "INC-COMM"
	commRuntime, kernel, signer := newCommunicationRuntime(t, db)
	incidentService := establishCommunicationIncident(t, db, kernel, signer, incidentID, base)

	sender := &postgresTestNotifier{attempts: map[string]app.NotificationAttempt{
		"first":  {Status: domain.NotificationFailed, Detail: "endpoint unavailable"},
		"second": {Status: domain.NotificationDelivered, Acknowledgement: "HTTP 202"},
	}}
	service := app.CommunicationService{
		Backend:  commRuntime,
		Incident: incidentService,
		Notifier: sender,
		Channels: []app.NotificationChannel{
			{Name: "first", Type: "webhook", Target: "https://example.invalid/first", DestinationID: "first-destination", External: true},
			{Name: "second", Type: "webhook", Target: "https://example.invalid/second", DestinationID: "second-destination", External: true},
		},
	}
	minor := communicationTestPrincipal("minor", authz.RoleMinor, incidentID, false)
	major := communicationTestPrincipal("major", authz.RoleMajor, incidentID, true)
	confirmer := communicationTestPrincipal("confirmer", authz.RoleMajor, incidentID, false)
	scope := domain.Scope{IncidentID: incidentID, Mode: "live", Compartment: "GENERAL"}

	cadenceProposal, err := service.ProposeCadence(ctx, major, scope, "CAD-1", domain.CommunicationCadence{Name: "Hourly SITREP", IntervalMinutes: 60, FirstDueAt: base.Add(10 * time.Minute), OwnerHumanID: "minor", Audience: []string{"incident-team"}, Channels: []string{"first", "second"}}, base.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	cadenceVersion, _ := domain.ProposalVersion(cadenceProposal)
	if _, err := service.Decide(ctx, confirmer, incidentID, domain.TypeCommunicationCadence, "CAD-1", cadenceVersion, "confirmed", base.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	cadence, found, err := commRuntime.Store.CommunicationCadence(ctx, incidentID, "CAD-1")
	if err != nil || !found || !cadence.NextDueAt.Equal(base.Add(10*time.Minute)) {
		t.Fatalf("cadence=%+v found=%v err=%v", cadence, found, err)
	}
	var wakeKind, wakeClockID string
	if err := db.runtime.QueryRow(ctx, `SELECT wake_kind,clock_id FROM crab.timed_wait_list WHERE incident_id=$1 AND wake_kind='communication_cadence'`, incidentID).Scan(&wakeKind, &wakeClockID); err != nil || wakeKind != "communication_cadence" || wakeClockID != "communication:CAD-1" {
		t.Fatalf("cadence wake kind=%q id=%q err=%v", wakeKind, wakeClockID, err)
	}

	draftRecord, err := service.GenerateDueSituationReportDraft(ctx, minor, incidentID, "CAD-1", "DRAFT-1", base.Add(11*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if draftRecord.RecordType != domain.TypeCommunicationDraft {
		t.Fatalf("draft record type=%s", draftRecord.RecordType)
	}
	draft, found, err := commRuntime.Store.CommunicationDraft(ctx, incidentID, "DRAFT-1")
	if err != nil || !found || draft.Draft.Kind != domain.CommunicationSituationReport || draft.SHA256 == "" {
		t.Fatalf("draft=%+v found=%v err=%v", draft, found, err)
	}
	cadence, _, _ = commRuntime.Store.CommunicationCadence(ctx, incidentID, "CAD-1")
	if cadence.LastGeneratedAt == nil || !cadence.LastGeneratedAt.Equal(base.Add(11*time.Minute)) || !cadence.NextDueAt.Equal(base.Add(70*time.Minute)) {
		t.Fatalf("cadence not advanced deterministically: %+v", cadence)
	}

	issueProposal, err := service.ProposeIssue(ctx, major, incidentID, "DRAFT-1", "COMM-1", base.Add(12*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.NotifyIssued(ctx, major, incidentID, "COMM-1", nil, base.Add(13*time.Minute)); !errors.Is(err, app.ErrCommunicationUnavailable) {
		t.Fatalf("unconfirmed communication could notify: %v", err)
	}
	issueVersion, _ := domain.ProposalVersion(issueProposal)
	if _, err := service.Decide(ctx, confirmer, incidentID, domain.TypeCommunicationIssue, "COMM-1", issueVersion, "confirmed", base.Add(14*time.Minute)); err != nil {
		t.Fatal(err)
	}
	issued, found, err := commRuntime.Store.IssuedCommunication(ctx, incidentID, "COMM-1")
	if err != nil || !found || issued.Issue.Body != draft.Draft.Body || issued.Issue.DraftSHA256 != draft.SHA256 {
		t.Fatalf("issued=%+v found=%v err=%v", issued, found, err)
	}

	outcomes, err := service.NotifyIssued(ctx, major, incidentID, "COMM-1", nil, base.Add(15*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 2 || outcomes[0].Status != domain.NotificationFailed || outcomes[1].Status != domain.NotificationDelivered {
		t.Fatalf("notification outcomes=%+v", outcomes)
	}
	if calls := sender.callNames(); len(calls) != 2 || calls[0] != "first" || calls[1] != "second" {
		t.Fatalf("failed channel suppressed later attempt: %v", calls)
	}
	storedOutcomes, err := commRuntime.Store.NotificationOutcomes(ctx, incidentID, "COMM-1")
	if err != nil || len(storedOutcomes) != 2 {
		t.Fatalf("stored outcomes=%+v err=%v", storedOutcomes, err)
	}

	if _, err := kernel.Rebuild(ctx, IncidentProjection{}, DeclarationAccessProjection{}, CommunicationProjection{}); err != nil {
		t.Fatal(err)
	}
	rebuiltDraft, found, err := commRuntime.Store.CommunicationDraft(ctx, incidentID, "DRAFT-1")
	if err != nil || !found || rebuiltDraft.SHA256 != draft.SHA256 || rebuiltDraft.Draft.Body != draft.Draft.Body {
		t.Fatalf("rebuilt draft=%+v found=%v err=%v", rebuiltDraft, found, err)
	}
	rebuiltIssued, found, err := commRuntime.Store.IssuedCommunication(ctx, incidentID, "COMM-1")
	if err != nil || !found || rebuiltIssued.Issue.DraftSHA256 != draft.SHA256 {
		t.Fatalf("rebuilt issued=%+v found=%v err=%v", rebuiltIssued, found, err)
	}
	rebuiltOutcomes, err := commRuntime.Store.NotificationOutcomes(ctx, incidentID, "COMM-1")
	if err != nil || len(rebuiltOutcomes) != 2 {
		t.Fatalf("rebuilt outcomes=%+v err=%v", rebuiltOutcomes, err)
	}
	rebuiltCadence, found, err := commRuntime.Store.CommunicationCadence(ctx, incidentID, "CAD-1")
	if err != nil || !found || rebuiltCadence.LastGeneratedAt == nil || !rebuiltCadence.NextDueAt.Equal(base.Add(70*time.Minute)) {
		t.Fatalf("rebuilt cadence=%+v found=%v err=%v", rebuiltCadence, found, err)
	}
}

func TestExerciseExternalNotificationSuppressedBeforeAdapterIOInPostgresPath(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	base := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	incidentID := "INC-EX-COMM"
	commRuntime, kernel, signer := newCommunicationRuntime(t, db)
	store, err := NewIncidentStore(db.runtime)
	if err != nil {
		t.Fatal(err)
	}
	incidentRuntime, err := NewIncidentRuntime(kernel, signer, store)
	if err != nil {
		t.Fatal(err)
	}
	incidentService := app.IncidentService{Backend: incidentRuntime}
	scope := domain.Scope{IncidentID: incidentID, Mode: "exercise", Compartment: "GENERAL"}
	declarer := authz.Principal{Authenticated: true, HumanID: "declarer-ex", Role: authz.RoleMajor}
	declarationConfirmer := authz.Principal{Authenticated: true, HumanID: "declaration-confirmer-ex", Role: authz.RoleMajor}
	proposal, err := incidentService.ProposeIncidentDeclaration(ctx, declarer, scope, "DECLARE-EX", domain.IncidentDeclaration{Title: "Exercise", Summary: "Simulated incident", Severity: "Medium"}, base)
	if err != nil {
		t.Fatal(err)
	}
	version, _ := domain.ProposalVersion(proposal)
	if _, err := incidentService.Decide(ctx, declarationConfirmer, incidentID, domain.TypeIncidentDeclaration, proposal.RecordID, version, "confirmed", base.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	sender := &postgresTestNotifier{}
	service := app.CommunicationService{Backend: commRuntime, Incident: incidentService, Notifier: sender, Channels: []app.NotificationChannel{{Name: "external", DestinationID: "external-regulator", External: true}}}
	major := communicationTestPrincipal("major-ex", authz.RoleMajor, incidentID, true)
	confirmer := communicationTestPrincipal("confirmer-ex", authz.RoleMajor, incidentID, false)
	draft := domain.CommunicationDraft{Kind: domain.CommunicationRegulatorNotice, Subject: "Exercise regulator notice", Body: "EXERCISE only", Audience: []string{"regulator"}, Channels: []string{"external"}}
	if _, err := service.RecordDraft(ctx, major, scope, "DRAFT-EX", draft, base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	issueProposal, err := service.ProposeIssue(ctx, major, incidentID, "DRAFT-EX", "COMM-EX", base.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	issueVersion, _ := domain.ProposalVersion(issueProposal)
	if _, err := service.Decide(ctx, confirmer, incidentID, domain.TypeCommunicationIssue, "COMM-EX", issueVersion, "confirmed", base.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	outcomes, err := service.NotifyIssued(ctx, major, incidentID, "COMM-EX", nil, base.Add(4*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(sender.callNames()) != 0 || len(outcomes) != 1 || outcomes[0].Status != domain.NotificationSuppressed {
		t.Fatalf("exercise adapter calls=%v outcomes=%+v", sender.callNames(), outcomes)
	}
	stored, err := commRuntime.Store.NotificationOutcomes(ctx, incidentID, "COMM-EX")
	if err != nil || len(stored) != 1 || stored[0].Outcome.Status != domain.NotificationSuppressed {
		t.Fatalf("stored exercise outcomes=%+v err=%v", stored, err)
	}
}
