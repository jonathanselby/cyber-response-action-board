package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

const (
	TypeFirstReport            = "intake.first_report"
	TypeFirstReportDisposition = "intake.first_report_disposition"
	TypeScopeAssessment        = "incident.scope_assessment"
	TypeSeverityChange         = "incident.severity_change"
	TypeHandover               = "incident.handover"
	TypeResourceRequest        = "incident.resource_request"
)

type FirstReport struct {
	Reporter        string        `json:"reporter,omitempty"`
	ContactRoute    string        `json:"contact_route,omitempty"`
	ReceivedAt      time.Time     `json:"received_at"`
	TimeConfidence  string        `json:"time_confidence"`
	Description     string        `json:"description"`
	Affected        string        `json:"affected,omitempty"`
	ImmediateImpact string        `json:"immediate_impact,omitempty"`
	SourceChannel   string        `json:"source_channel"`
	TriageStatus    string        `json:"triage_status"`
	Attachments     []BlobReceipt `json:"attachments,omitempty"`
}

type FirstReportDisposition struct {
	ReportID   string `json:"report_id"`
	Decision   string `json:"decision"`
	IncidentID string `json:"incident_id,omitempty"`
	Reason     string `json:"reason"`
}

type ScopeAssessment struct {
	AffectedAssets        []string `json:"affected_assets,omitempty"`
	AffectedServices      []string `json:"affected_services,omitempty"`
	PeopleDataPopulations []string `json:"people_data_populations,omitempty"`
	Locations             []string `json:"locations,omitempty"`
	Suppliers             []string `json:"suppliers,omitempty"`
	Impact                string   `json:"impact"`
	Confidence            string   `json:"confidence"`
	ReasonEvidence        string   `json:"reason_evidence"`
}

type SeverityChange struct {
	Severity       string `json:"severity"`
	ReasonEvidence string `json:"reason_evidence"`
}

type Handover struct {
	Situation                string   `json:"situation"`
	OperationalRoles         []string `json:"operational_roles,omitempty"`
	CriticalActions          []string `json:"critical_actions,omitempty"`
	PendingDecisions         []string `json:"pending_decisions,omitempty"`
	Obligations              []string `json:"obligations,omitempty"`
	Unknowns                 []string `json:"unknowns,omitempty"`
	NextObjectives           []string `json:"next_objectives,omitempty"`
	CommunicationCommitments []string `json:"communication_commitments,omitempty"`
}

type ResourceRequest struct {
	Category        string `json:"category"`
	Request         string `json:"request"`
	OwnerHumanID    string `json:"owner_human_id"`
	Priority        string `json:"priority"`
	Status          string `json:"status"`
	AuthoritySource string `json:"authority_source"`
	Notes           string `json:"notes,omitempty"`
}

type FirstReportView struct {
	ReportID      string
	Report        FirstReport
	Maker         string
	RecordedAt    time.Time
	Disposition   *FirstReportDisposition
	DispositionAt *time.Time
}

type ScopeAssessmentView struct {
	RecordID    string
	Compartment string
	Assessment  ScopeAssessment
	Maker       string
	RecordedAt  time.Time
}

type SeverityChangeView struct {
	RecordID    string
	Compartment string
	Change      SeverityChange
	Maker       string
	Confirmer   string
	ConfirmedAt time.Time
}

type HandoverView struct {
	RecordID    string
	Compartment string
	Handover    Handover
	Maker       string
	RecordedAt  time.Time
}

type ResourceRequestView struct {
	RecordID    string
	Compartment string
	Request     ResourceRequest
	Maker       string
	RecordedAt  time.Time
}

func RecordFirstReport(recordID, maker string, now time.Time, report FirstReport) (ledger.Record, error) {
	if strings.TrimSpace(recordID) == "" || strings.TrimSpace(maker) == "" || now.IsZero() {
		return ledger.Record{}, errors.New("report id, maker and time are required")
	}
	if report.ReceivedAt.IsZero() || strings.TrimSpace(report.Description) == "" || strings.TrimSpace(report.SourceChannel) == "" {
		return ledger.Record{}, errors.New("received time, description and source channel are required")
	}
	if report.TimeConfidence != ledger.TimeTrusted && report.TimeConfidence != ledger.TimeDrifting && report.TimeConfidence != ledger.TimeUntrusted {
		return ledger.Record{}, errors.New("first-report time confidence must be trusted, drifting or untrusted")
	}
	if strings.TrimSpace(report.TriageStatus) == "" {
		report.TriageStatus = "new"
	}
	report.ReceivedAt = report.ReceivedAt.UTC()
	raw, err := json.Marshal(report)
	if err != nil {
		return ledger.Record{}, err
	}
	record := ledger.Record{SchemaVersion: 1, Timestamp: now.UTC(), TimeStatus: ledger.TimeTrusted, IncidentMode: "system", WriteSemantic: "record", RecordType: TypeFirstReport, RecordID: recordID, Status: "recorded", Maker: maker, Compartment: "SYSTEM", Data: raw, MakerSignature: ledger.Signature{MissingReason: "maker authenticated by CRAB session; no per-record signing key"}}
	if err := ledger.ValidateRecord(record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func RecordFirstReportDisposition(recordID, maker string, now time.Time, value FirstReportDisposition) (ledger.Record, error) {
	if strings.TrimSpace(recordID) == "" || strings.TrimSpace(maker) == "" || now.IsZero() || strings.TrimSpace(value.ReportID) == "" || strings.TrimSpace(value.Reason) == "" {
		return ledger.Record{}, errors.New("disposition id, report id, maker, time and reason are required")
	}
	switch value.Decision {
	case "associate", "reject", "retain", "propose_declaration":
	default:
		return ledger.Record{}, errors.New("first-report decision must be associate, reject, retain or propose_declaration")
	}
	if (value.Decision == "associate" || value.Decision == "propose_declaration") && strings.TrimSpace(value.IncidentID) == "" {
		return ledger.Record{}, errors.New("associated/proposed declaration decision requires incident id")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return ledger.Record{}, err
	}
	record := ledger.Record{SchemaVersion: 1, Timestamp: now.UTC(), TimeStatus: ledger.TimeTrusted, IncidentMode: "system", WriteSemantic: "record", RecordType: TypeFirstReportDisposition, RecordID: recordID, Status: "recorded", Maker: maker, Compartment: "SYSTEM", Data: raw, MakerSignature: ledger.Signature{MissingReason: "maker authenticated by CRAB session; no per-record signing key"}}
	if err := ledger.ValidateRecord(record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func RecordScopeAssessment(scope Scope, recordID, maker string, now time.Time, value ScopeAssessment) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if strings.TrimSpace(value.Impact) == "" || strings.TrimSpace(value.Confidence) == "" || strings.TrimSpace(value.ReasonEvidence) == "" {
		return ledger.Record{}, errors.New("scope assessment impact, confidence and reason/evidence are required")
	}
	return immediateRecord(scope, TypeScopeAssessment, recordID, maker, now, value)
}

func ProposeSeverityChange(scope Scope, recordID, maker string, now time.Time, value SeverityChange) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if strings.TrimSpace(value.Severity) == "" || strings.TrimSpace(value.ReasonEvidence) == "" {
		return ledger.Record{}, errors.New("severity and reason/evidence are required")
	}
	return proposedRecord(scope, TypeSeverityChange, recordID, maker, now, value)
}

func RecordHandover(scope Scope, recordID, maker string, now time.Time, value Handover) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if strings.TrimSpace(value.Situation) == "" {
		return ledger.Record{}, errors.New("handover situation is required")
	}
	return immediateRecord(scope, TypeHandover, recordID, maker, now, value)
}

func RecordResourceRequest(scope Scope, recordID, maker string, now time.Time, value ResourceRequest) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if strings.TrimSpace(value.Category) == "" || strings.TrimSpace(value.Request) == "" || strings.TrimSpace(value.OwnerHumanID) == "" || strings.TrimSpace(value.Priority) == "" || strings.TrimSpace(value.Status) == "" || strings.TrimSpace(value.AuthoritySource) == "" {
		return ledger.Record{}, errors.New("resource request category, request, owner, priority, status and authority source are required")
	}
	return immediateRecord(scope, TypeResourceRequest, recordID, maker, now, value)
}
