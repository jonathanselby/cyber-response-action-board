package web

import (
	"context"
	"io"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

// Runtime is the single seam between the browser presentation tier and the
// composition root. Every method resolves the session, authorises in the
// application tier and returns authorised view data. Presentation never
// decides authority.
type Runtime interface {
	// Session and frame.
	Bootstrapped(context.Context) (bool, error)
	Bootstrap(context.Context, BootstrapInput) error
	Authenticate(context.Context, string, string) (string, error)
	Logout(context.Context, string) error
	Frame(ctx context.Context, token, incidentID string) (FrameView, bool, error)
	Home(context.Context, string) (HomeView, bool, error)
	Incident(context.Context, string, string) (IncidentView, bool, error)

	// Incident board.
	Board(ctx context.Context, token, incidentID, compartment string) (BoardView, error)
	DeclareIncident(ctx context.Context, token, incidentID, mode, title, summary, severity string) error
	RecordFact(ctx context.Context, token, incidentID, statement, source string) error
	RecordUncertainty(ctx context.Context, token, incidentID, kind, statement string) error
	ProposeDecision(ctx context.Context, token, incidentID, decision, rationale string) error
	ProposeRoleAssignment(ctx context.Context, token, incidentID, roleName, humanID string, deputy bool) error
	ProposeAction(ctx context.Context, token, incidentID, title, owner, priority, sourceRef string) error
	ProposeActionStatus(ctx context.Context, token, incidentID, actionID, status, skipReason string) error
	Retrieve(ctx context.Context, token, incidentID, question string) (GuidanceView, error)

	// Continuity.
	Intake(context.Context, string) (IntakeView, bool, error)
	CaptureFirstReport(context.Context, string, FirstReportCapture) error
	TriageFirstReport(ctx context.Context, token, reportID, decision, incidentID, reason string) error
	Operations(context.Context, string, string) (OperationsView, bool, error)
	RecordScopeAssessment(context.Context, string, string, ScopeAssessmentInput) error
	ProposeSeverityChange(ctx context.Context, token, incidentID, severity, reason string) error
	GenerateHandover(ctx context.Context, token, incidentID string, objectives, commitments []string) error
	RecordResourceRequest(context.Context, string, string, ResourceRequestInput) error

	// Obligations.
	GlobalObligations(context.Context, string) (GlobalObligationsPage, error)
	IncidentObligations(context.Context, string, string, string, time.Time) (IncidentObligationsPage, error)
	ProposeObligationDefinition(context.Context, string, domain.ObligationDefinition) error
	ProposeObligationTrigger(context.Context, string, string, string, domain.ObligationTrigger) error
	ProposeObligationChange(context.Context, string, string, string, domain.ObligationChange) error

	// Evidence.
	EvidenceRegister(ctx context.Context, token, incidentID, compartment string) ([]domain.EvidenceView, error)
	CaptureEvidence(ctx context.Context, token, incidentID, compartment, filename, mediaType, description, collector, source string, collectedAt, occurredAt *time.Time, sourceLocation, currentHandler, relatedRef string, body io.Reader) error
	EvidenceDetail(ctx context.Context, token, evidenceID string) (domain.EvidenceView, error)
	EvidenceHistory(ctx context.Context, token, evidenceID string) ([]domain.EvidenceHandlingView, error)
	RecordEvidenceHandling(ctx context.Context, token, evidenceID, action, handler, location, note string, occurredAt *time.Time) error
	DownloadEvidence(ctx context.Context, token, evidenceID string, w io.Writer) error

	// Communications.
	Communications(ctx context.Context, token, incidentID, compartment string) (CommunicationsPage, error)
	RecordCommunicationDraft(ctx context.Context, token, incidentID, compartment string, value domain.CommunicationDraft) error
	ProposeCommunicationIssue(ctx context.Context, token, incidentID, draftID string) error
	ProposeCommunicationCadence(ctx context.Context, token, incidentID, compartment string, value domain.CommunicationCadence) error
	RecordIncomingEnquiry(ctx context.Context, token, incidentID, compartment string, value domain.IncomingEnquiry) error
	GenerateSituationReport(ctx context.Context, token, incidentID, cadenceID, compartment string) error
	NotifyCommunication(ctx context.Context, token, incidentID, communicationID string, channels []string) error

	// Access and readiness.
	AccessAdministration(context.Context, string, time.Time) (AccessAdministrationPage, error)
	IncidentAccess(context.Context, string, string, time.Time) (AccessAdministrationPage, error)
	ProposeSponsor(ctx context.Context, token, incidentID, humanID string, active bool) error
	ProposeController(ctx context.Context, token, incidentID, humanID, compartment string, active bool) error
	ProposeIncidentMembership(ctx context.Context, token, incidentID, humanID string, active bool, effectiveAt, expiresAt *time.Time, reason string) error
	ProposeCompartmentMembership(ctx context.Context, token, incidentID, humanID, compartment string, active bool, effectiveAt, expiresAt *time.Time, reason string) error
	Readiness(context.Context, string) (ReadinessPage, error)

	// Close-out and paper.
	CloseOut(ctx context.Context, token, incidentID string) (CloseOutView, error)
	RecordRecoveryUpdate(ctx context.Context, token, incidentID string, input RecoveryUpdateInput) error
	ProposeStandDown(ctx context.Context, token, incidentID, reason string) error
	ProposePIR(ctx context.Context, token, incidentID, summary string, lessons, longTermWork []string) error
	ProposeRetention(ctx context.Context, token, incidentID string, input RetentionInput) error
	ProposeClosure(ctx context.Context, token, incidentID string, input ClosureInput) error
	TranscribePaper(ctx context.Context, token, incidentID string, input PaperInput) error
	IncidentBriefPDF(ctx context.Context, token, incidentID, pageSize, printMode string) ([]byte, error)
	EmergencyPackPDF(ctx context.Context, token, incidentID, pageSize, printMode string) ([]byte, error)
	OrganisationPackPDF(ctx context.Context, token, pageSize, printMode string) ([]byte, error)

	// Approvals: one surface and one decision path for every governed change.
	Approvals(ctx context.Context, token, incidentID string) (ApprovalsView, error)
	DecideProposal(ctx context.Context, token string, input DecisionInput) error

	// Decide is retained for the JSON API contract (incident-scoped decisions).
	Decide(ctx context.Context, token, incidentID, recordType, recordID, version, outcome string) error
}

// RuntimeFactory is installed by the composition root.
var RuntimeFactory func() (Runtime, error)

// ---- Frame -----------------------------------------------------------------

type FrameView struct {
	HumanID   string
	Role      string
	Custodian bool
	Status    StatusFlags
	Incident  *IncidentContext
}

type StatusFlags struct {
	IntegrityFailed  bool
	BreakGlass       bool
	Isolate          bool
	TimeStatus       string // trusted | drifting | untrusted
	PendingApprovals int
	PackStale        bool
	ReadinessFail    bool
	ModelUnavailable bool
}

type IncidentContext struct {
	ID           string
	Title        string
	Mode         string
	Phase        string
	Compartments []string // compartments this human may read, GENERAL first
}

// ---- Sign-in and incidents -------------------------------------------------

type BootstrapInput struct {
	PreparedBy        string
	PreparedPassword  string
	ConfirmedBy       string
	ConfirmedPassword string
}

type IncidentLink struct {
	IncidentID string
	Mode       string
	Title      string
	Severity   string
	Phase      string
	Pending    int
}

type FactView struct{ Statement, Source string }
type ActionView struct{ RecordID, Title, Owner, Status, Priority string }
type PendingView struct{ RecordType, RecordID, Version, Maker string }
type TimelineView struct{ Summary, RecordedAt, Governance string }

// IncidentView is the JSON API incident shape.
type IncidentView struct {
	IncidentID string
	Mode       string
	Title      string
	Summary    string
	Severity   string
	Facts      []FactView
	Actions    []ActionView
	Pending    []PendingView
	Timeline   []TimelineView
}

type HomeView struct {
	HumanID             string
	Role                string
	CanDeclare          bool
	Incidents           []IncidentLink
	PendingDeclarations []PendingDeclaration
	// Command Summary attention items, already scoped to this user (T-198).
	WaitingApprovals int
	CanTriage        bool
	AwaitingTriage   int
}

type PendingDeclaration struct {
	IncidentID string
	RecordID   string
	Version    string
	Maker      string
	Title      string
	Summary    string
	Severity   string
	Mode       string
}

// ---- Board -----------------------------------------------------------------

type BoardView struct {
	Picture          domain.OperatingPicture
	Phase            string
	Compartment      string
	LatestScope      *domain.ScopeAssessmentView
	SeverityHistory  []domain.SeverityChangeView
	LatestHandover   *domain.HandoverView
	Resources        []domain.ResourceRequestView
	Clocks           []domain.ObligationClockView
	NextReportDue    string
	OpenEnquiries    int
	IssuedRecently   []domain.CommunicationView
	Readiness        []ReadinessCheckView
	SeverityValues   []string
	Members          []string
	PendingApprovals int
}

type GuidanceView struct {
	Question  string
	Refused   bool
	Reason    string
	Citations []domain.RetrievalCitation
}

// ---- Approvals -------------------------------------------------------------

type ApprovalsView struct {
	Waiting       []ApprovalView
	AwaitingOther []ApprovalView
	Incidents     []IncidentLink
	Filter        string
}

type ApprovalView struct {
	Kind          string
	Label         string
	IncidentID    string
	IncidentTitle string
	Compartment   string
	Summary       string
	Effective     []Field
	Proposed      []Field
	Maker         string
	ProposedAt    string
	Reason        string
	Consequence   string
	ScopeID       string
	RecordType    string
	RecordID      string
	Version       string
	CanDecide     bool
}

type Field struct{ Name, Value string }

type DecisionInput struct {
	Kind       string
	ScopeID    string
	RecordType string
	RecordID   string
	Version    string
	Outcome    string
}

// ---- Continuity ------------------------------------------------------------

const firstReportRequestBytes = int64(1 << 30)

type FirstReportAttachment struct {
	Filename  string
	MediaType string
	Reader    io.Reader
}

type FirstReportCapture struct {
	Reporter        string
	ContactRoute    string
	ReceivedAt      time.Time
	TimeConfidence  string
	Description     string
	Affected        string
	ImmediateImpact string
	SourceChannel   string
	Attachments     []FirstReportAttachment
}

type IntakeReportRow struct {
	ReportID        string
	Reporter        string
	ContactRoute    string
	ReceivedAt      string
	Description     string
	Affected        string
	ImmediateImpact string
	SourceChannel   string
	TriageStatus    string
	AttachmentCount int
	Maker           string
	RecordedAt      string
	Decision        string
	IncidentID      string
	Reason          string
}

type IntakeView struct {
	HumanID   string
	Role      string
	CanTriage bool
	Reports   []IntakeReportRow
}

type ScopeAssessmentInput struct {
	AffectedAssets        string
	AffectedServices      string
	PeopleDataPopulations string
	Locations             string
	Suppliers             string
	Impact                string
	Confidence            string
	ReasonEvidence        string
}

type ScopeAssessmentRow struct {
	AffectedAssets        string
	AffectedServices      string
	PeopleDataPopulations string
	Locations             string
	Suppliers             string
	Impact                string
	Confidence            string
	ReasonEvidence        string
	Maker                 string
	RecordedAt            string
}

type SeverityChangeRow struct {
	Severity       string
	ReasonEvidence string
	Maker          string
	Confirmer      string
	RecordedAt     string
}

type PendingSeverityRow struct {
	RecordID       string
	Version        string
	Maker          string
	Severity       string
	ReasonEvidence string
	ProposedAt     string
}

type HandoverRow struct {
	Situation                string
	OperationalRoles         string
	CriticalActions          string
	PendingDecisions         string
	Obligations              string
	Unknowns                 string
	NextObjectives           string
	CommunicationCommitments string
	Maker                    string
	RecordedAt               string
}

type ResourceRequestInput struct {
	RecordID        string
	Category        string
	Request         string
	OwnerHumanID    string
	Priority        string
	Status          string
	AuthoritySource string
	Notes           string
}

type ResourceRequestRow struct {
	RecordID        string
	Category        string
	Request         string
	OwnerHumanID    string
	Priority        string
	Status          string
	AuthoritySource string
	Notes           string
	Maker           string
	RecordedAt      string
}

type OperationsView struct {
	Scopes          []ScopeAssessmentRow
	SeverityHistory []SeverityChangeRow
	PendingSeverity []PendingSeverityRow
	Handovers       []HandoverRow
	Resources       []ResourceRequestRow
}

// ---- Access and readiness --------------------------------------------------

type AccessDesignationView struct {
	HumanID     string
	IncidentID  string
	Compartment string
}

type AccessMembershipView struct {
	HumanID     string
	IncidentID  string
	Compartment string
	EffectiveAt string
	ExpiresAt   string
	Reason      string
	Status      string
	Implicit    bool
}

type AccessPendingView struct {
	ScopeID     string
	RecordID    string
	RecordType  string
	Compartment string
	Version     string
	Maker       string
	Mode        string
	ProposedAt  string
}

type AccessAdministrationPage struct {
	Sponsors               []AccessDesignationView
	Controllers            []AccessDesignationView
	IncidentMemberships    []AccessMembershipView
	CompartmentMemberships []AccessMembershipView
	Pending                []AccessPendingView
}

type ReadinessCheckView struct {
	Name    string
	Status  string
	Message string
}

type ReadinessPage struct {
	Ready  bool
	Checks []ReadinessCheckView
}

// ---- Evidence --------------------------------------------------------------

type EvidenceRegisterPage struct {
	IncidentID   string
	Compartment  string
	Compartments []string
	Items        []domain.EvidenceView
}

type EvidenceDetailPage struct {
	Item    domain.EvidenceView
	History []domain.EvidenceHandlingView
	Inline  string // "image" | "audio" | "video" | "" (download only)
}

// ---- Obligations -----------------------------------------------------------

type ObligationDefinitionItem struct {
	DefinitionID string
	Definition   domain.ObligationDefinition
	Maker        string
	Confirmer    string
	ConfirmedAt  string
}

type PendingObligationItem struct {
	ScopeID     string
	RecordID    string
	RecordType  string
	Compartment string
	Version     string
	Maker       string
	Mode        string
	ProposedAt  string
}

type GlobalObligationsPage struct {
	Definitions []ObligationDefinitionItem
	Pending     []PendingObligationItem
}

type IncidentObligationsPage struct {
	IncidentID  string
	Compartment string
	Clocks      []domain.ObligationClockView
	Pending     []PendingObligationItem
}

// ---- Communications --------------------------------------------------------

type PendingCommunicationItem struct {
	IncidentID  string
	RecordID    string
	RecordType  string
	Compartment string
	Version     string
	Maker       string
	Mode        string
	ProposedAt  string
}

type CommunicationsPage struct {
	IncidentID   string
	Compartment  string
	Compartments []string
	Drafts       []domain.CommunicationDraftView
	Issued       []domain.CommunicationView
	Outcomes     map[string][]domain.NotificationOutcomeView
	Cadences     []domain.CommunicationCadenceView
	Enquiries    []domain.IncomingEnquiryView
	Pending      []PendingCommunicationItem
}

// ---- Close-out -------------------------------------------------------------

type CloseOutView struct {
	IncidentID string
	State      domain.RecoveryState
	PIRDraft   *domain.PIRDraft
	Pending    int
}

type RecoveryUpdateInput struct {
	Summary             string
	OwnerHumanID        string
	RestorationPriority string
	Verified            bool
	ResidualRisk        string
	HandoffRef          string
}

type RetentionInput struct {
	RetentionClass           string
	Hold                     bool
	HoldReason               string
	DispositionEligibleAfter *time.Time
	HandoffRef               string
}

type ClosureInput struct {
	Reason                string
	PrivilegedCompartment string
	PrivilegedSummary     string
	ShareableSummary      string
}

type PaperInput struct {
	Statement       string
	PaperOccurredAt time.Time
	SourceReference string
	TimeConfidence  string
}
