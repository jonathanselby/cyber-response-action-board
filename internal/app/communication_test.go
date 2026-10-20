package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

type fakeCommunicationBackend struct {
	drafts    map[string]domain.CommunicationDraftView
	issued    map[string]domain.CommunicationView
	cadences  map[string]domain.CommunicationCadenceView
	pending   map[string]ledger.Record
	appended  []ledger.Record
	outcomes  []domain.NotificationOutcome
}

func (b *fakeCommunicationBackend) AppendCommunication(_ context.Context, record ledger.Record) error {
	b.appended = append(b.appended, record)
	if record.RecordType == domain.TypeNotificationOutcome {
		value, err := domain.DecodePayload[domain.NotificationOutcome](record)
		if err != nil {
			return err
		}
		b.outcomes = append(b.outcomes, value)
	}
	if record.RecordType == domain.TypeCommunicationDraft {
		value, err := domain.DecodePayload[domain.CommunicationDraft](record)
		if err != nil {
			return err
		}
		digest, err := domain.CommunicationDraftDigest(value)
		if err != nil {
			return err
		}
		if b.drafts == nil {
			b.drafts = map[string]domain.CommunicationDraftView{}
		}
		b.drafts[record.RecordID] = domain.CommunicationDraftView{RecordID: record.RecordID, IncidentID: record.IncidentID, Mode: record.IncidentMode, Compartment: record.Compartment, Draft: value, SHA256: digest, Maker: record.Maker, RecordedAt: record.Timestamp}
	}
	return nil
}
func (b *fakeCommunicationBackend) PendingCommunication(_ context.Context, _, recordType, recordID string) (ledger.Record, bool, error) {
	value, ok := b.pending[recordType+"|"+recordID]
	return value, ok, nil
}
func (b *fakeCommunicationBackend) CommunicationDraft(_ context.Context, _, recordID string) (domain.CommunicationDraftView, bool, error) {
	value, ok := b.drafts[recordID]
	return value, ok, nil
}
func (b *fakeCommunicationBackend) IssuedCommunication(_ context.Context, _, recordID string) (domain.CommunicationView, bool, error) {
	value, ok := b.issued[recordID]
	return value, ok, nil
}
func (b *fakeCommunicationBackend) CommunicationCadence(_ context.Context, _, recordID string) (domain.CommunicationCadenceView, bool, error) {
	value, ok := b.cadences[recordID]
	return value, ok, nil
}

type fakeIncidentPictureReader struct{ picture domain.OperatingPicture }

func (f fakeIncidentPictureReader) CommonOperatingPicture(context.Context, authz.Principal, string, string) (domain.OperatingPicture, bool, error) {
	return f.picture, true, nil
}

type fakeNotificationSender struct {
	mu       sync.Mutex
	attempts map[string]NotificationAttempt
	calls    []string
}

func (f *fakeNotificationSender) Deliver(_ context.Context, channel NotificationChannel, _ NotificationMessage) NotificationAttempt {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, channel.Name)
	if attempt, ok := f.attempts[channel.Name]; ok {
		return attempt
	}
	return NotificationAttempt{Status: domain.NotificationDelivered}
}
func (f *fakeNotificationSender) callNames() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func communicationPrincipal(role authz.Role, notify bool) authz.Principal {
	outputs := map[authz.Action]bool{}
	if notify {
		outputs[authz.ActionNotify] = true
	}
	return authz.Principal{Authenticated: true, HumanID: "alice", Role: role, Incidents: map[string]bool{"INC-COMM": true}, Compartments: map[string]map[string]bool{"INC-COMM": {"GENERAL": true}}, Outputs: outputs}
}

func issuedCommunication(mode string, channels ...string) domain.CommunicationView {
	return domain.CommunicationView{RecordID: "COMM-1", IncidentID: "INC-COMM", Mode: mode, Compartment: "GENERAL", Issue: domain.CommunicationIssue{DraftRecordID: "DRAFT-1", DraftSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Kind: domain.CommunicationRegulatorNotice, Subject: "Notice", Body: "Approved content", Audience: []string{"regulator"}, Channels: channels}}
}

func TestUnconfirmedDraftCannotNotify(t *testing.T) {
	backend := &fakeCommunicationBackend{drafts: map[string]domain.CommunicationDraftView{"DRAFT-1": {RecordID: "DRAFT-1", IncidentID: "INC-COMM", Mode: "live", Compartment: "GENERAL"}}, issued: map[string]domain.CommunicationView{}}
	sender := &fakeNotificationSender{}
	service := CommunicationService{Backend: backend, Notifier: sender, Channels: []NotificationChannel{{Name: "webhook", External: true}}}
	_, err := service.NotifyIssued(context.Background(), communicationPrincipal(authz.RoleMajor, true), "INC-COMM", "DRAFT-1", nil, time.Now())
	if !errors.Is(err, ErrCommunicationUnavailable) || len(sender.callNames()) != 0 || len(backend.outcomes) != 0 {
		t.Fatalf("unconfirmed draft notification err=%v calls=%v outcomes=%v", err, sender.callNames(), backend.outcomes)
	}
}

func TestNotifierContinuesAfterFailureAndRecordsEachOutcome(t *testing.T) {
	backend := &fakeCommunicationBackend{issued: map[string]domain.CommunicationView{"COMM-1": issuedCommunication("live", "first", "second")}}
	sender := &fakeNotificationSender{attempts: map[string]NotificationAttempt{"first": {Status: domain.NotificationFailed, Detail: "endpoint unavailable"}, "second": {Status: domain.NotificationDelivered, Acknowledgement: "HTTP 202"}}}
	service := CommunicationService{Backend: backend, Notifier: sender, Channels: []NotificationChannel{{Name: "first", DestinationID: "a", External: true}, {Name: "second", DestinationID: "b", External: true}}}
	outcomes, err := service.NotifyIssued(context.Background(), communicationPrincipal(authz.RoleMajor, true), "INC-COMM", "COMM-1", nil, time.Date(2026, 10, 20, 7, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 2 || outcomes[0].Status != domain.NotificationFailed || outcomes[1].Status != domain.NotificationDelivered || len(backend.outcomes) != 2 {
		t.Fatalf("outcomes=%+v recorded=%+v", outcomes, backend.outcomes)
	}
	if calls := sender.callNames(); len(calls) != 2 || calls[0] != "first" || calls[1] != "second" {
		t.Fatalf("channel failure suppressed later attempt: %v", calls)
	}
}

func TestExerciseSuppressesExternalBeforeIOButRecordsLocalOutcome(t *testing.T) {
	backend := &fakeCommunicationBackend{issued: map[string]domain.CommunicationView{"COMM-1": issuedCommunication("exercise", "local", "external")}}
	sender := &fakeNotificationSender{attempts: map[string]NotificationAttempt{"local": {Status: domain.NotificationDelivered}}}
	service := CommunicationService{Backend: backend, Notifier: sender, Channels: []NotificationChannel{{Name: "local", DestinationID: "local-ops", External: false}, {Name: "external", DestinationID: "external-regulator", External: true}}}
	outcomes, err := service.NotifyIssued(context.Background(), communicationPrincipal(authz.RoleMajor, true), "INC-COMM", "COMM-1", nil, time.Date(2026, 10, 20, 7, 5, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if calls := sender.callNames(); len(calls) != 1 || calls[0] != "local" {
		t.Fatalf("exercise external I/O occurred: %v", calls)
	}
	if len(outcomes) != 2 || outcomes[0].Status != domain.NotificationDelivered || outcomes[1].Status != domain.NotificationSuppressed || len(backend.outcomes) != 2 {
		t.Fatalf("exercise outcomes=%+v recorded=%+v", outcomes, backend.outcomes)
	}
}

func TestDueSituationReportUsesCurrentCRABState(t *testing.T) {
	now := time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)
	backend := &fakeCommunicationBackend{cadences: map[string]domain.CommunicationCadenceView{"CAD-1": {CadenceID: "CAD-1", IncidentID: "INC-COMM", Mode: "live", Compartment: "GENERAL", Cadence: domain.CommunicationCadence{Name: "Hourly SITREP", IntervalMinutes: 60, FirstDueAt: now.Add(-time.Hour), OwnerHumanID: "alice", Audience: []string{"incident-team"}, Channels: []string{"local"}}, NextDueAt: now.Add(-time.Minute)}}}
	picture := domain.OperatingPicture{Incident: domain.IncidentState{IncidentID: "INC-COMM", Mode: "live", Title: "Bafflewick", Summary: "Service unavailable", Severity: "High"}, Facts: []domain.FactView{{RecordID: "FACT-1", Fact: domain.Fact{Statement: "Service remains unavailable"}}}}
	service := CommunicationService{Backend: backend, Incident: fakeIncidentPictureReader{picture: picture}}
	record, err := service.GenerateDueSituationReportDraft(context.Background(), communicationPrincipal(authz.RoleMinor, false), "INC-COMM", "CAD-1", "DRAFT-SITREP", now)
	if err != nil {
		t.Fatal(err)
	}
	if record.RecordType != domain.TypeCommunicationDraft {
		t.Fatalf("record type=%s", record.RecordType)
	}
	draft := backend.drafts["DRAFT-SITREP"].Draft
	if draft.Kind != domain.CommunicationSituationReport || draft.CadenceID != "CAD-1" || !stringsContains(draft.Body, "Service remains unavailable") {
		t.Fatalf("draft=%+v", draft)
	}
}

func TestProposeIssueUsesStoredDraftNotCallerContent(t *testing.T) {
	draft := domain.CommunicationDraft{Kind: domain.CommunicationHoldingStatement, Subject: "Holding statement", Body: "Stored exact wording", Audience: []string{"media"}, Channels: []string{"webhook"}}
	digest, _ := domain.CommunicationDraftDigest(draft)
	backend := &fakeCommunicationBackend{drafts: map[string]domain.CommunicationDraftView{"DRAFT-1": {RecordID: "DRAFT-1", IncidentID: "INC-COMM", Mode: "live", Compartment: "GENERAL", Draft: draft, SHA256: digest}}}
	service := CommunicationService{Backend: backend}
	record, err := service.ProposeIssue(context.Background(), communicationPrincipal(authz.RoleMajor, false), "INC-COMM", "DRAFT-1", "COMM-1", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	issue, err := domain.DecodePayload[domain.CommunicationIssue](record)
	if err != nil {
		t.Fatal(err)
	}
	if issue.Body != "Stored exact wording" || issue.DraftSHA256 != digest {
		t.Fatalf("issue=%+v", issue)
	}
}

func stringsContains(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return true
		}
	}
	return false
}
