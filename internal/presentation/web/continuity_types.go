package web

import (
	"context"
	"io"
	"time"
)

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

type ContinuityRuntime interface {
	Intake(context.Context, string) (IntakeView, bool, error)
	CaptureFirstReport(context.Context, string, FirstReportCapture) error
	TriageFirstReport(context.Context, string, string, string, string, string) error
	Operations(context.Context, string, string) (OperationsView, bool, error)
	RecordScopeAssessment(context.Context, string, string, ScopeAssessmentInput) error
	ProposeSeverityChange(context.Context, string, string, string, string) error
	DecideSeverity(context.Context, string, string, string, string, string) error
	GenerateHandover(context.Context, string, string, []string, []string) error
	RecordResourceRequest(context.Context, string, string, ResourceRequestInput) error
}
