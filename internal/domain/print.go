package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

const (
	PageSizeA4     = "A4"
	PageSizeLetter = "LETTER"

	PrintColour     = "colour"
	PrintMonochrome = "monochrome"

	TypePaperTranscription = "incident.paper_transcription"
)

type PrintSourceMark struct {
	LedgerSequence       uint64
	LedgerHash           string
	ConfigurationVersion string
	GeneratedAt          time.Time
	TimeStatus           string
}

type PrintContact struct {
	Role      string
	Name      string
	Primary   string
	Alternate string
}

type IncidentBrief struct {
	Source                    PrintSourceMark
	Incident                  IncidentState
	Compartment               string
	Facts                     []FactView
	Unknowns                  []UncertaintyView
	Hypotheses                []UncertaintyView
	Decisions                 []DecisionView
	Actions                   []ActionView
	Obligations               []ObligationClockView
	OperationalRoles          []OperationalRoleView
	NextCommunicationDeadline *time.Time
	Contacts                  []PrintContact
}

type PackSection struct {
	Title string
	Body  string
	// Optional table, printed before the body text.
	Columns []string
	Widths  []float64
	Rows    [][]string
}

type BlankForm struct {
	Title  string
	Fields []string
}

type EmergencyResponsePack struct {
	Source       PrintSourceMark
	Organisation string
	Mode         string
	Sections     []PackSection
	Forms        []BlankForm
}

type EvidenceRegisterDocument struct {
	Source      PrintSourceMark
	Incident    IncidentState
	Compartment string
	Evidence    []EvidenceView
}

type TimelineDocument struct {
	Source      PrintSourceMark
	Incident    IncidentState
	Compartment string
	Entries     []TimelineEntry
}

type ReviewClosureDocument struct {
	Source   PrintSourceMark
	Incident IncidentState
	Title    string
	Status   string
	Sections []PackSection
}

type PaperTranscription struct {
	Statement       string    `json:"statement"`
	PaperOccurredAt time.Time `json:"paper_occurred_at"`
	SourceReference string    `json:"source_reference"`
	TimeConfidence  string    `json:"time_confidence"`
}

func ValidatePrintSourceMark(value PrintSourceMark) error {
	if value.LedgerSequence == 0 || strings.TrimSpace(value.LedgerHash) == "" || strings.TrimSpace(value.ConfigurationVersion) == "" || value.GeneratedAt.IsZero() {
		return errors.New("print source mark requires ledger sequence/hash, configuration version and generation time")
	}
	if value.TimeStatus != ledger.TimeTrusted && value.TimeStatus != ledger.TimeDrifting && value.TimeStatus != ledger.TimeUntrusted {
		return errors.New("print source mark has invalid time status")
	}
	return nil
}

func ValidatePageSize(value string) error {
	if value != PageSizeA4 && value != PageSizeLetter {
		return errors.New("page size must be A4 or LETTER")
	}
	return nil
}

func ValidatePrintMode(value string) error {
	if value != PrintColour && value != PrintMonochrome {
		return errors.New("print mode must be colour or monochrome")
	}
	return nil
}

func ValidateEmergencyResponsePack(value EmergencyResponsePack) error {
	if err := ValidatePrintSourceMark(value.Source); err != nil {
		return err
	}
	if strings.TrimSpace(value.Organisation) == "" {
		return errors.New("emergency response pack requires organisation")
	}
	if value.Mode != "live" && value.Mode != "exercise" {
		return errors.New("emergency response pack mode must be live or exercise")
	}
	if len(value.Sections) == 0 || len(value.Forms) == 0 {
		return errors.New("emergency response pack requires approved sections and blank forms")
	}
	for _, section := range value.Sections {
		if strings.TrimSpace(section.Title) == "" || (strings.TrimSpace(section.Body) == "" && len(section.Rows) == 0) {
			return errors.New("emergency response pack section requires title and body or rows")
		}
	}
	for _, form := range value.Forms {
		if strings.TrimSpace(form.Title) == "" || len(form.Fields) == 0 {
			return errors.New("blank contingency form requires title and fields")
		}
		for _, field := range form.Fields {
			if strings.TrimSpace(field) == "" {
				return errors.New("blank contingency form contains blank field")
			}
		}
	}
	return nil
}

func DefaultContingencyForms() []BlankForm {
	return []BlankForm{
		{Title: "Incident Record", Fields: []string{"Incident ID", "Date/time", "Reporter", "Situation / impact", "Known facts", "Unknowns / hypotheses", "Severity", "Source reference"}},
		{Title: "Decision Record", Fields: []string{"Incident ID", "Date/time", "Decision", "Rationale", "Maker", "Confirmer", "Source reference"}},
		{Title: "Action Record", Fields: []string{"Incident ID", "Date/time", "Action", "Owner", "Priority", "Due", "Status", "Source reference"}},
		{Title: "Communication Record", Fields: []string{"Incident ID", "Date/time", "Audience / recipient", "Approved wording", "Authority / approver", "Channel", "Delivery outcome", "Acknowledgement / response"}},
		{Title: "Evidence Record", Fields: []string{"Incident ID", "Date/time", "Evidence ID", "Description", "Collector", "Source", "Location", "Current handler", "Hash / reference"}},
	}
}

func ValidatePaperTranscription(value PaperTranscription) error {
	if strings.TrimSpace(value.Statement) == "" || value.PaperOccurredAt.IsZero() || strings.TrimSpace(value.SourceReference) == "" {
		return errors.New("paper transcription requires statement, paper occurrence time and source reference")
	}
	if value.TimeConfidence != ledger.TimeTrusted && value.TimeConfidence != ledger.TimeDrifting && value.TimeConfidence != ledger.TimeUntrusted {
		return errors.New("paper transcription time confidence must be trusted, drifting or untrusted")
	}
	return nil
}

func RecordPaperTranscription(scope Scope, recordID, transcriber string, transcribedAt time.Time, value PaperTranscription) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if err := ValidatePaperTranscription(value); err != nil {
		return ledger.Record{}, err
	}
	if transcribedAt.IsZero() {
		return ledger.Record{}, errors.New("paper transcription time is required")
	}
	record, err := immediateRecord(scope, TypePaperTranscription, recordID, transcriber, transcribedAt, value)
	if err != nil {
		return ledger.Record{}, err
	}
	record.TimeStatus = value.TimeConfidence
	if err := ledger.ValidateRecord(record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}
