package web

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

// fakeRuntime implements the single Runtime seam for presentation tests. It
// knows exactly one authorised incident, INC-1. Any other incident ID is
// treated as not authorised, mirroring the application tier.
type fakeRuntime struct {
	bootstrapped bool
	mode         string // incident mode; "exercise" by default
	status       StatusFlags
	compartments []string

	refuse       error // returned by every mutation when set
	calls        []string
	decision     DecisionInput
	decided      bool
	severity     string
	capture      FirstReportCapture
	evidenceComp string
	pdfErr       error
	reads        map[string]int // read calls, for frame/summary cost tests
}

func (f *fakeRuntime) read(name string) {
	if f.reads == nil {
		f.reads = map[string]int{}
	}
	f.reads[name]++
}

const testIncident = "INC-1"

func (f *fakeRuntime) record(call string) error {
	f.calls = append(f.calls, call)
	return f.refuse
}

func (f *fakeRuntime) called(name string) bool {
	for _, c := range f.calls {
		if c == name {
			return true
		}
	}
	return false
}

func (f *fakeRuntime) incidentMode() string {
	if f.mode == "" {
		return "exercise"
	}
	return f.mode
}

func (f *fakeRuntime) Bootstrapped(context.Context) (bool, error) { return f.bootstrapped, nil }
func (f *fakeRuntime) Bootstrap(context.Context, BootstrapInput) error {
	f.bootstrapped = true
	return nil
}
func (f *fakeRuntime) Authenticate(_ context.Context, id, _ string) (string, error) {
	if id == "wrong" {
		return "", errors.New("authentication failed")
	}
	return "token", nil
}
func (f *fakeRuntime) Logout(context.Context, string) error { return nil }

func (f *fakeRuntime) Frame(_ context.Context, token, incidentID string) (FrameView, bool, error) {
	f.read("Frame")
	if token == "" || token == "expired" {
		return FrameView{}, false, nil
	}
	frame := FrameView{HumanID: "alice", Role: "Major", Custodian: true, Status: f.status}
	if incidentID == testIncident {
		comps := f.compartments
		if len(comps) == 0 {
			comps = []string{"GENERAL"}
		}
		frame.Incident = &IncidentContext{ID: testIncident, Title: "Bafflewick payroll", Mode: f.incidentMode(), Phase: "Active response", Compartments: comps}
	}
	return frame, true, nil
}

func (f *fakeRuntime) Home(context.Context, string) (HomeView, bool, error) {
	f.read("Home")
	return HomeView{HumanID: "alice", Role: "Major", CanDeclare: true, Incidents: []IncidentLink{{IncidentID: testIncident, Mode: f.incidentMode(), Title: "Bafflewick payroll", Severity: "High", Phase: "Active response", Pending: 2}}, WaitingApprovals: 2, CanTriage: true, AwaitingTriage: 1}, true, nil
}

func (f *fakeRuntime) Incident(_ context.Context, _ string, id string) (IncidentView, bool, error) {
	if id != testIncident {
		return IncidentView{}, false, nil
	}
	return IncidentView{IncidentID: testIncident, Mode: f.incidentMode(), Title: "Bafflewick payroll", Severity: "High"}, true, nil
}

func (f *fakeRuntime) Board(_ context.Context, _ string, id, compartment string) (BoardView, error) {
	if id != testIncident {
		return BoardView{}, errors.New("authorisation denied")
	}
	occurred := time.Date(2026, 10, 20, 1, 0, 0, 0, time.UTC)
	flagged := occurred.Add(10 * time.Minute)
	due := time.Date(2026, 10, 20, 4, 0, 0, 0, time.UTC)
	picture := domain.OperatingPicture{
		Incident:   domain.IncidentState{IncidentID: testIncident, Mode: f.incidentMode(), Title: "Bafflewick payroll", Summary: "Payroll files encrypted on the finance share.", Severity: "High", DeclaredAt: occurred},
		Facts:      []domain.FactView{{RecordID: "FACT-1", Compartment: "GENERAL", Fact: domain.Fact{Statement: "Payroll files encrypted", Source: "SOC analyst", OccurredAt: &occurred, FlaggedAt: &flagged}, Maker: "bob", RecordedAt: flagged.Add(5 * time.Minute)}},
		Unknowns:   []domain.UncertaintyView{{RecordID: "UNC-1", Uncertainty: domain.Uncertainty{Kind: "unknown", Statement: "Was data exfiltrated?"}, Maker: "bob"}},
		Hypotheses: []domain.UncertaintyView{{RecordID: "UNC-2", Uncertainty: domain.Uncertainty{Kind: "hypothesis", Statement: "Entry via VPN appliance"}, Maker: "carol"}},
		Actions: []domain.ActionView{{RecordID: "ACT-1", Compartment: "GENERAL", Action: domain.Action{
			Title: "Isolate affected endpoints", OwnerHumanID: "carol", Status: domain.ActionStatusInProgress, Priority: "P1", DueAt: &due,
			SourceRef: "RUNBOOK-RANSOMWARE-4.2", ChecklistTemplateID: "TPL-RANSOMWARE", ChecklistStepID: "isolate"}, Overdue: true}},
		OperationalRoles: []domain.OperationalRoleView{{RecordID: "ROLE-1", Assignment: domain.OperationalRoleAssignment{RoleName: "Incident commander", HumanID: "alice"}}},
		Pending:          []domain.PendingProposal{{RecordID: "DEC-9", RecordType: domain.TypeDecision, Version: "v1", Maker: "bob", ProposedAt: flagged}},
		Timeline:         []domain.TimelineEntry{{RecordType: domain.TypeFact, Governance: "recorded", Summary: "Payroll files encrypted", OccurredAt: &occurred, FlaggedAt: &flagged, RecordedAt: flagged.Add(5 * time.Minute), TimeStatus: "trusted"}},
	}
	clock := domain.ObligationClockView{ClockID: "CLOCK-1", Name: "Regulator notification", Lifecycle: domain.ObligationLifecycleProvisional, Status: "active", Deadline: due.Add(72 * time.Hour), OriginalDeadline: due.Add(72 * time.Hour), Recipient: "Regulator", Owner: "legal", TimeStatus: "untrusted"}
	scope := domain.ScopeAssessmentView{Assessment: domain.ScopeAssessment{Impact: "Payroll unavailable", Confidence: "high", AffectedServices: []string{"Payroll"}}}
	return BoardView{
		Picture: picture, Phase: "Active response", Compartment: compartment, LatestScope: &scope,
		Clocks: []domain.ObligationClockView{clock}, SeverityValues: []string{"High", "Critical"}, Members: []string{"alice", "bob", "carol"},
		Resources: []domain.ResourceRequestView{{Request: domain.ResourceRequest{Category: "people", Request: "IR surge", OwnerHumanID: "alice", Priority: "P1", Status: "requested", AuthoritySource: "incident lead"}}},
	}, nil
}

func (f *fakeRuntime) DeclareIncident(context.Context, string, string, string, string, string, string) error {
	return f.record("DeclareIncident")
}
func (f *fakeRuntime) RecordFact(context.Context, string, string, string, string) error {
	return f.record("RecordFact")
}
func (f *fakeRuntime) RecordUncertainty(context.Context, string, string, string, string) error {
	return f.record("RecordUncertainty")
}
func (f *fakeRuntime) ProposeDecision(context.Context, string, string, string, string) error {
	return f.record("ProposeDecision")
}
func (f *fakeRuntime) ProposeRoleAssignment(context.Context, string, string, string, string, bool) error {
	return f.record("ProposeRoleAssignment")
}
func (f *fakeRuntime) ProposeAction(context.Context, string, string, string, string, string, string) error {
	return f.record("ProposeAction")
}
func (f *fakeRuntime) ProposeActionStatus(context.Context, string, string, string, string, string) error {
	return f.record("ProposeActionStatus")
}
func (f *fakeRuntime) Retrieve(_ context.Context, _ string, _ string, q string) (GuidanceView, error) {
	return GuidanceView{Question: q, Citations: []domain.RetrievalCitation{{Title: "Ransomware runbook", Locator: "lines 10-14", Passage: "Isolate before wiping."}}}, nil
}

func (f *fakeRuntime) Intake(context.Context, string) (IntakeView, bool, error) {
	f.read("Intake")
	return IntakeView{HumanID: "alice", Role: "Major", CanTriage: true, Reports: []IntakeReportRow{{ReportID: "REPORT-1", Description: "Phone report", SourceChannel: "phone", ReceivedAt: "2026-10-21T01:00:00Z"}}}, true, nil
}
func (f *fakeRuntime) CaptureFirstReport(_ context.Context, _ string, input FirstReportCapture) error {
	f.capture = input
	return f.record("CaptureFirstReport")
}
func (f *fakeRuntime) TriageFirstReport(context.Context, string, string, string, string, string) error {
	return f.record("TriageFirstReport")
}
func (f *fakeRuntime) Operations(context.Context, string, string) (OperationsView, bool, error) {
	return OperationsView{}, true, nil
}
func (f *fakeRuntime) RecordScopeAssessment(context.Context, string, string, ScopeAssessmentInput) error {
	return f.record("RecordScopeAssessment")
}
func (f *fakeRuntime) ProposeSeverityChange(_ context.Context, _ string, _ string, severity, _ string) error {
	f.severity = severity
	return f.record("ProposeSeverityChange")
}
func (f *fakeRuntime) GenerateHandover(context.Context, string, string, []string, []string) error {
	return f.record("GenerateHandover")
}
func (f *fakeRuntime) RecordResourceRequest(context.Context, string, string, ResourceRequestInput) error {
	return f.record("RecordResourceRequest")
}

func (f *fakeRuntime) GlobalObligations(context.Context, string) (GlobalObligationsPage, error) {
	return GlobalObligationsPage{}, nil
}
func (f *fakeRuntime) IncidentObligations(context.Context, string, string, string, time.Time) (IncidentObligationsPage, error) {
	return IncidentObligationsPage{}, nil
}
func (f *fakeRuntime) ProposeObligationDefinition(context.Context, string, domain.ObligationDefinition) error {
	return f.record("ProposeObligationDefinition")
}
func (f *fakeRuntime) ProposeObligationTrigger(context.Context, string, string, string, domain.ObligationTrigger) error {
	return f.record("ProposeObligationTrigger")
}
func (f *fakeRuntime) ProposeObligationChange(context.Context, string, string, string, domain.ObligationChange) error {
	return f.record("ProposeObligationChange")
}

func (f *fakeRuntime) EvidenceRegister(_ context.Context, _ string, _ string, compartment string) ([]domain.EvidenceView, error) {
	f.evidenceComp = compartment
	return []domain.EvidenceView{{EvidenceID: "EV-1", IncidentID: testIncident, Compartment: compartment, Evidence: domain.Evidence{BlobReceipt: domain.BlobReceipt{OriginalFilename: "screenshot.png", SizeBytes: 2048, DetectedMediaType: "image/png"}, CurrentHandler: "carol"}}}, nil
}
func (f *fakeRuntime) CaptureEvidence(context.Context, string, string, string, string, string, string, string, string, *time.Time, *time.Time, string, string, string, io.Reader) error {
	return f.record("CaptureEvidence")
}
func (f *fakeRuntime) EvidenceDetail(_ context.Context, _ string, id string) (domain.EvidenceView, error) {
	if id != "EV-1" {
		return domain.EvidenceView{}, errors.New("authorisation denied")
	}
	return domain.EvidenceView{EvidenceID: "EV-1", IncidentID: testIncident, Compartment: "GENERAL", TimeStatus: "trusted", Evidence: domain.Evidence{BlobReceipt: domain.BlobReceipt{OriginalFilename: "screenshot.png", SizeBytes: 2048, DeclaredMediaType: "image/png", DetectedMediaType: "image/png", SHA256: "abc123"}, Collector: "carol", Source: "laptop", CurrentHandler: "carol"}}, nil
}
func (f *fakeRuntime) EvidenceHistory(context.Context, string, string) ([]domain.EvidenceHandlingView, error) {
	return nil, nil
}
func (f *fakeRuntime) RecordEvidenceHandling(context.Context, string, string, string, string, string, string, *time.Time) error {
	return f.record("RecordEvidenceHandling")
}
func (f *fakeRuntime) DownloadEvidence(_ context.Context, _ string, _ string, w io.Writer) error {
	_, err := w.Write([]byte("bytes"))
	return err
}

func (f *fakeRuntime) Communications(_ context.Context, _ string, id, compartment string) (CommunicationsPage, error) {
	return CommunicationsPage{IncidentID: id, Compartment: compartment}, nil
}
func (f *fakeRuntime) RecordCommunicationDraft(context.Context, string, string, string, domain.CommunicationDraft) error {
	return f.record("RecordCommunicationDraft")
}
func (f *fakeRuntime) ProposeCommunicationIssue(context.Context, string, string, string) error {
	return f.record("ProposeCommunicationIssue")
}
func (f *fakeRuntime) ProposeCommunicationCadence(context.Context, string, string, string, domain.CommunicationCadence) error {
	return f.record("ProposeCommunicationCadence")
}
func (f *fakeRuntime) RecordIncomingEnquiry(context.Context, string, string, string, domain.IncomingEnquiry) error {
	return f.record("RecordIncomingEnquiry")
}
func (f *fakeRuntime) GenerateSituationReport(context.Context, string, string, string, string) error {
	return f.record("GenerateSituationReport")
}
func (f *fakeRuntime) NotifyCommunication(context.Context, string, string, string, []string) error {
	return f.record("NotifyCommunication")
}

func (f *fakeRuntime) AccessAdministration(context.Context, string, time.Time) (AccessAdministrationPage, error) {
	return AccessAdministrationPage{}, nil
}
func (f *fakeRuntime) IncidentAccess(context.Context, string, string, time.Time) (AccessAdministrationPage, error) {
	return AccessAdministrationPage{IncidentMemberships: []AccessMembershipView{{HumanID: "dave", IncidentID: testIncident, Status: "active", ExpiresAt: "2026-11-02T00:00:00Z"}}}, nil
}
func (f *fakeRuntime) ProposeSponsor(context.Context, string, string, string, bool) error {
	return f.record("ProposeSponsor")
}
func (f *fakeRuntime) ProposeController(context.Context, string, string, string, string, bool) error {
	return f.record("ProposeController")
}
func (f *fakeRuntime) ProposeIncidentMembership(context.Context, string, string, string, bool, *time.Time, *time.Time, string) error {
	return f.record("ProposeIncidentMembership")
}
func (f *fakeRuntime) ProposeCompartmentMembership(context.Context, string, string, string, string, bool, *time.Time, *time.Time, string) error {
	return f.record("ProposeCompartmentMembership")
}
func (f *fakeRuntime) ProposeOutputPermission(_ context.Context, _, humanID, action string, active bool) error {
	return f.record("output:" + humanID + ":" + action)
}
func (f *fakeRuntime) Readiness(context.Context, string) (ReadinessPage, error) {
	f.read("Readiness")
	return ReadinessPage{Ready: false, Checks: []ReadinessCheckView{{Name: "emergency_pack", Status: "warn", Message: "no generated PDF found"}}}, nil
}

func (f *fakeRuntime) CloseOut(_ context.Context, _ string, id string) (CloseOutView, error) {
	return CloseOutView{IncidentID: id, State: domain.RecoveryState{IncidentID: id, Phase: domain.PhaseActiveResponse}}, nil
}
func (f *fakeRuntime) RecordRecoveryUpdate(context.Context, string, string, RecoveryUpdateInput) error {
	return f.record("RecordRecoveryUpdate")
}
func (f *fakeRuntime) ProposeStandDown(context.Context, string, string, string) error {
	return f.record("ProposeStandDown")
}
func (f *fakeRuntime) ProposePIR(context.Context, string, string, string, []string, []string) error {
	return f.record("ProposePIR")
}
func (f *fakeRuntime) ProposeRetention(context.Context, string, string, RetentionInput) error {
	return f.record("ProposeRetention")
}
func (f *fakeRuntime) ProposeClosure(context.Context, string, string, ClosureInput) error {
	return f.record("ProposeClosure")
}
func (f *fakeRuntime) TranscribePaper(context.Context, string, string, PaperInput) error {
	return f.record("TranscribePaper")
}
func (f *fakeRuntime) IncidentBriefPDF(context.Context, string, string, string, string) ([]byte, error) {
	if f.pdfErr != nil {
		return nil, f.pdfErr
	}
	return []byte("%PDF-1.7 brief"), nil
}
func (f *fakeRuntime) EmergencyPackPDF(context.Context, string, string, string, string) ([]byte, error) {
	return []byte("%PDF-1.7 pack"), nil
}
func (f *fakeRuntime) OrganisationPackPDF(context.Context, string, string, string) ([]byte, error) {
	return []byte("%PDF-1.7 pack"), nil
}

func (f *fakeRuntime) Approvals(context.Context, string, string) (ApprovalsView, error) {
	f.read("Approvals")
	return ApprovalsView{
		Incidents: []IncidentLink{{IncidentID: testIncident, Title: "Bafflewick payroll"}},
		Waiting: []ApprovalView{{
			Kind: "severity", Label: "Severity change", IncidentID: testIncident, IncidentTitle: "Bafflewick payroll", Compartment: "GENERAL",
			Summary: "Change severity to Critical", Effective: []Field{{Name: "Severity", Value: "High"}}, Proposed: []Field{{Name: "Severity", Value: "Critical"}},
			Maker: "bob", ProposedAt: "20 Oct 2026 01:15 UTC", Reason: "Spread to second share confirmed", Consequence: "Severity becomes Critical on the Board, brief and handover.",
			ScopeID: testIncident, RecordType: domain.TypeSeverityChange, RecordID: "SEV-1", Version: "v1", CanDecide: true,
		}},
	}, nil
}
func (f *fakeRuntime) DecideProposal(_ context.Context, _ string, input DecisionInput) error {
	f.decision = input
	f.decided = true
	return f.record("DecideProposal")
}
func (f *fakeRuntime) Decide(context.Context, string, string, string, string, string, string) error {
	return f.record("Decide")
}

// ---- helpers ------------------------------------------------------------------

func withRuntime(t *testing.T, runtime Runtime) *Server {
	t.Helper()
	old := RuntimeFactory
	t.Cleanup(func() { RuntimeFactory = old })
	RuntimeFactory = func() (Runtime, error) { return runtime, nil }
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func get(t *testing.T, s *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "token"})
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}

func postForm(t *testing.T, s *Server, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "http://example.com"+path, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "http://example.com")
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "token"})
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}

func (f *fakeRuntime) Organisation(context.Context, string) (OrganisationPage, error) {
	f.read("Organisation")
	return OrganisationPage{CanPropose: true, CanReadContacts: true, Severity: domain.DefaultSeverityScale().Levels, SeverityMeta: &ConfirmedBy{Maker: "alice", Confirmer: "bob", At: "26 Oct 2026 03:00 UTC"},
		Emergency: &domain.EmergencyConfiguration{Organisation: "Bafflewick Ltd", Contacts: []domain.EmergencyContact{{Name: "Alice Smith", Role: "Incident commander", Primary: "+44 7700 900001", Alternative: "+44 20 7946 0001"}}, Parties: []domain.ExternalParty{{Category: "insurer", Name: "Example Mutual", Primary: "+44 20 7946 0000"}}, Escalation: []string{"Duty manager", "CIO"}, RoleCards: []domain.RoleCard{{Title: "Incident commander", Responsibilities: "Runs the response", Deputy: "bob"}}, Procedures: []domain.Procedure{{Title: "Ransomware first hour", Body: "Isolate. Preserve. Call insurer."}}}}, nil
}
func (f *fakeRuntime) ProposeSeverityScale(_ context.Context, _ string, labels []string) error {
	return f.record("scale:" + strings.Join(labels, "|"))
}
func (f *fakeRuntime) ProposeEmergencyConfiguration(_ context.Context, _ string, v domain.EmergencyConfiguration) error {
	return f.record("emergency:" + v.Organisation)
}
