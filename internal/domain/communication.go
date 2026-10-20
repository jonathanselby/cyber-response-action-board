package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

const (
	TypeCommunicationDraft   = "incident.communication_draft"
	TypeCommunicationIssue   = "incident.communication"
	TypeCommunicationCadence = "incident.communication_cadence"
	TypeNotificationOutcome  = "incident.notification_outcome"
	TypeIncomingEnquiry      = "incident.incoming_enquiry"

	CommunicationSituationReport      = "situation_report"
	CommunicationRegulatorNotice      = "regulator_notification"
	CommunicationIndividualNotice     = "individual_notice"
	CommunicationExecutiveUpdate      = "executive_update"
	CommunicationStaffCommunication   = "staff_communication"
	CommunicationHoldingStatement     = "holding_statement"
	CommunicationClosurePIR           = "closure_pir"
	NotificationDelivered             = "delivered"
	NotificationFailed                = "failed"
	NotificationSuppressed            = "suppressed"
	MaxCommunicationBodyBytes         = 256 << 10
	MaxCommunicationCadenceMinutes    = 7 * 24 * 60
	MaxCommunicationRecipients        = 128
	MaxCommunicationChannels          = 16
	MaxCommunicationRelatedReferences = 128
)

type CommunicationDraft struct {
	Kind             string   `json:"kind"`
	Subject          string   `json:"subject"`
	Body             string   `json:"body"`
	Audience         []string `json:"audience"`
	Channels         []string `json:"channels,omitempty"`
	SourceRefs       []string `json:"source_refs,omitempty"`
	RelatedRecordIDs []string `json:"related_record_ids,omitempty"`
	CadenceID        string   `json:"cadence_id,omitempty"`
}

type CommunicationIssue struct {
	DraftRecordID     string   `json:"draft_record_id"`
	DraftSHA256       string   `json:"draft_sha256"`
	Kind              string   `json:"kind"`
	Subject           string   `json:"subject"`
	Body              string   `json:"body"`
	Audience          []string `json:"audience"`
	Channels          []string `json:"channels,omitempty"`
	SourceRefs        []string `json:"source_refs,omitempty"`
	RelatedRecordIDs  []string `json:"related_record_ids,omitempty"`
	CadenceID         string   `json:"cadence_id,omitempty"`
}

type CommunicationCadence struct {
	Name            string    `json:"name"`
	IntervalMinutes int       `json:"interval_minutes"`
	FirstDueAt      time.Time `json:"first_due_at"`
	OwnerHumanID    string    `json:"owner_human_id"`
	Audience        []string  `json:"audience"`
	Channels        []string  `json:"channels"`
}

type NotificationOutcome struct {
	CommunicationID      string    `json:"communication_id"`
	Channel              string    `json:"channel"`
	DestinationID        string    `json:"destination_id,omitempty"`
	AttemptedAt          time.Time `json:"attempted_at"`
	Status               string    `json:"status"`
	Detail               string    `json:"detail,omitempty"`
	Acknowledgement      string    `json:"acknowledgement,omitempty"`
	DurationMS           int64     `json:"duration_ms"`
}

type IncomingEnquiry struct {
	Source            string    `json:"source"`
	Contact           string    `json:"contact,omitempty"`
	Subject           string    `json:"subject"`
	Message           string    `json:"message"`
	ReceivedAt        time.Time `json:"received_at"`
	AssignedToHumanID string    `json:"assigned_to_human_id"`
	RelatedRecordIDs  []string  `json:"related_record_ids,omitempty"`
}

type CommunicationDraftView struct {
	RecordID     string
	IncidentID   string
	Mode         string
	Compartment  string
	Draft        CommunicationDraft
	SHA256       string
	Maker        string
	RecordedAt   time.Time
}

type CommunicationView struct {
	RecordID     string
	IncidentID   string
	Mode         string
	Compartment  string
	Issue        CommunicationIssue
	Maker        string
	Confirmer    string
	IssuedAt     time.Time
}

type CommunicationCadenceView struct {
	CadenceID       string
	IncidentID      string
	Mode            string
	Compartment     string
	Cadence         CommunicationCadence
	Maker           string
	Confirmer       string
	ConfirmedAt     time.Time
	LastGeneratedAt *time.Time
	NextDueAt       time.Time
	Simulated       bool
}

type NotificationOutcomeView struct {
	RecordID    string
	IncidentID  string
	Compartment string
	Outcome     NotificationOutcome
	Maker       string
	RecordedAt  time.Time
}

type IncomingEnquiryView struct {
	RecordID    string
	IncidentID  string
	Compartment string
	Enquiry     IncomingEnquiry
	Maker       string
	RecordedAt  time.Time
}

func ValidateCommunicationDraft(value CommunicationDraft) error {
	if !validCommunicationKind(value.Kind) {
		return errors.New("unsupported communication kind")
	}
	if strings.TrimSpace(value.Subject) == "" || strings.TrimSpace(value.Body) == "" {
		return errors.New("communication subject and body are required")
	}
	if len(value.Body) > MaxCommunicationBodyBytes {
		return errors.New("communication body exceeds bounded size")
	}
	if err := validateStringSet("audience", value.Audience, 1, MaxCommunicationRecipients); err != nil {
		return err
	}
	if err := validateStringSet("channels", value.Channels, 0, MaxCommunicationChannels); err != nil {
		return err
	}
	if err := validateStringSet("source refs", value.SourceRefs, 0, MaxCommunicationRelatedReferences); err != nil {
		return err
	}
	return validateStringSet("related record ids", value.RelatedRecordIDs, 0, MaxCommunicationRelatedReferences)
}

func CommunicationDraftDigest(value CommunicationDraft) (string, error) {
	if err := ValidateCommunicationDraft(value); err != nil {
		return "", err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func CommunicationIssueFromDraft(recordID, digest string, draft CommunicationDraft) (CommunicationIssue, error) {
	if strings.TrimSpace(recordID) == "" || len(digest) != 64 {
		return CommunicationIssue{}, errors.New("communication issue requires draft id and sha256")
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return CommunicationIssue{}, errors.New("communication draft sha256 is invalid")
	}
	if err := ValidateCommunicationDraft(draft); err != nil {
		return CommunicationIssue{}, err
	}
	return CommunicationIssue{
		DraftRecordID:    recordID,
		DraftSHA256:      digest,
		Kind:             draft.Kind,
		Subject:          draft.Subject,
		Body:             draft.Body,
		Audience:         append([]string(nil), draft.Audience...),
		Channels:         append([]string(nil), draft.Channels...),
		SourceRefs:       append([]string(nil), draft.SourceRefs...),
		RelatedRecordIDs: append([]string(nil), draft.RelatedRecordIDs...),
		CadenceID:        draft.CadenceID,
	}, nil
}

func ValidateCommunicationIssue(value CommunicationIssue) error {
	draft := CommunicationDraft{Kind: value.Kind, Subject: value.Subject, Body: value.Body, Audience: value.Audience, Channels: value.Channels, SourceRefs: value.SourceRefs, RelatedRecordIDs: value.RelatedRecordIDs, CadenceID: value.CadenceID}
	if err := ValidateCommunicationDraft(draft); err != nil {
		return err
	}
	if strings.TrimSpace(value.DraftRecordID) == "" || len(value.DraftSHA256) != 64 {
		return errors.New("communication issue requires bound draft id and sha256")
	}
	if _, err := hex.DecodeString(value.DraftSHA256); err != nil {
		return errors.New("communication issue draft sha256 is invalid")
	}
	return nil
}

func ValidateCommunicationCadence(value CommunicationCadence) error {
	if strings.TrimSpace(value.Name) == "" || strings.TrimSpace(value.OwnerHumanID) == "" || value.FirstDueAt.IsZero() {
		return errors.New("communication cadence requires name, owner and first due time")
	}
	if value.IntervalMinutes <= 0 || value.IntervalMinutes > MaxCommunicationCadenceMinutes {
		return errors.New("communication cadence interval is outside supported bounds")
	}
	if err := validateStringSet("audience", value.Audience, 1, MaxCommunicationRecipients); err != nil {
		return err
	}
	return validateStringSet("channels", value.Channels, 1, MaxCommunicationChannels)
}

func ValidateNotificationOutcome(value NotificationOutcome) error {
	if strings.TrimSpace(value.CommunicationID) == "" || strings.TrimSpace(value.Channel) == "" || value.AttemptedAt.IsZero() {
		return errors.New("notification outcome requires communication, channel and attempt time")
	}
	switch value.Status {
	case NotificationDelivered, NotificationFailed, NotificationSuppressed:
	default:
		return errors.New("notification outcome must be delivered, failed or suppressed")
	}
	if value.DurationMS < 0 {
		return errors.New("notification duration cannot be negative")
	}
	return nil
}

func ValidateIncomingEnquiry(value IncomingEnquiry) error {
	if strings.TrimSpace(value.Source) == "" || strings.TrimSpace(value.Subject) == "" || strings.TrimSpace(value.Message) == "" || strings.TrimSpace(value.AssignedToHumanID) == "" || value.ReceivedAt.IsZero() {
		return errors.New("incoming enquiry requires source, subject, message, received time and assignee")
	}
	if len(value.Message) > MaxCommunicationBodyBytes {
		return errors.New("incoming enquiry message exceeds bounded size")
	}
	return validateStringSet("related record ids", value.RelatedRecordIDs, 0, MaxCommunicationRelatedReferences)
}

func RecordCommunicationDraft(scope Scope, recordID, maker string, now time.Time, value CommunicationDraft) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if err := ValidateCommunicationDraft(value); err != nil {
		return ledger.Record{}, err
	}
	return immediateRecord(scope, TypeCommunicationDraft, recordID, maker, now, value)
}

func ProposeCommunicationIssue(scope Scope, recordID, maker string, now time.Time, value CommunicationIssue) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if err := ValidateCommunicationIssue(value); err != nil {
		return ledger.Record{}, err
	}
	return proposedRecord(scope, TypeCommunicationIssue, recordID, maker, now, value)
}

func ProposeCommunicationCadence(scope Scope, recordID, maker string, now time.Time, value CommunicationCadence) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if err := ValidateCommunicationCadence(value); err != nil {
		return ledger.Record{}, err
	}
	value.FirstDueAt = value.FirstDueAt.UTC()
	return proposedRecord(scope, TypeCommunicationCadence, recordID, maker, now, value)
}

func RecordNotificationOutcome(scope Scope, recordID, maker string, now time.Time, value NotificationOutcome) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if err := ValidateNotificationOutcome(value); err != nil {
		return ledger.Record{}, err
	}
	value.AttemptedAt = value.AttemptedAt.UTC()
	return immediateRecord(scope, TypeNotificationOutcome, recordID, maker, now, value)
}

func RecordIncomingEnquiry(scope Scope, recordID, maker string, now time.Time, value IncomingEnquiry) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if err := ValidateIncomingEnquiry(value); err != nil {
		return ledger.Record{}, err
	}
	value.ReceivedAt = value.ReceivedAt.UTC()
	return immediateRecord(scope, TypeIncomingEnquiry, recordID, maker, now, value)
}

func BuildSituationReportDraft(picture OperatingPicture, cadenceID string, audience, channels []string, generatedAt time.Time) (CommunicationDraft, error) {
	if picture.Incident.IncidentID == "" || generatedAt.IsZero() {
		return CommunicationDraft{}, errors.New("situation report requires current incident state and generation time")
	}
	if err := validateStringSet("audience", audience, 1, MaxCommunicationRecipients); err != nil {
		return CommunicationDraft{}, err
	}
	if err := validateStringSet("channels", channels, 1, MaxCommunicationChannels); err != nil {
		return CommunicationDraft{}, err
	}
	var b strings.Builder
	if picture.Incident.Mode == "exercise" {
		b.WriteString("EXERCISE — NOT A LIVE INCIDENT\n\n")
	}
	fmt.Fprintf(&b, "Incident: %s\nSeverity: %s\nGenerated: %s\n\nSituation\n%s\n", picture.Incident.Title, picture.Incident.Severity, generatedAt.UTC().Format(time.RFC3339), picture.Incident.Summary)
	refs := make([]string, 0)
	if len(picture.Facts) > 0 {
		b.WriteString("\nConfirmed/recorded facts\n")
		for _, fact := range picture.Facts {
			fmt.Fprintf(&b, "- %s\n", fact.Fact.Statement)
			refs = append(refs, fact.RecordID)
		}
	}
	if len(picture.Unknowns) > 0 {
		b.WriteString("\nUnknowns\n")
		for _, item := range picture.Unknowns {
			fmt.Fprintf(&b, "- %s\n", item.Uncertainty.Statement)
			refs = append(refs, item.RecordID)
		}
	}
	if len(picture.Decisions) > 0 {
		b.WriteString("\nRecent decisions\n")
		for _, item := range picture.Decisions {
			fmt.Fprintf(&b, "- %s\n", item.Decision.Decision)
			refs = append(refs, item.RecordID)
		}
	}
	if len(picture.Actions) > 0 {
		b.WriteString("\nActions\n")
		for _, item := range picture.Actions {
			if item.Action.Status == "complete" || item.Action.Status == "skipped" {
				continue
			}
			fmt.Fprintf(&b, "- %s — %s — %s\n", item.Action.Title, item.Action.OwnerHumanID, item.Action.Status)
			refs = append(refs, item.RecordID)
		}
	}
	if len(picture.Obligations) > 0 {
		b.WriteString("\nActive obligations\n")
		for _, item := range picture.Obligations {
			if item.Status != ObligationStatusActive {
				continue
			}
			fmt.Fprintf(&b, "- %s — due %s — owner %s\n", item.Name, item.Deadline.UTC().Format(time.RFC3339), item.Owner)
			refs = append(refs, item.ClockID)
		}
	}
	if len(picture.OperationalRoles) > 0 {
		b.WriteString("\nOperational roles\n")
		for _, item := range picture.OperationalRoles {
			fmt.Fprintf(&b, "- %s — %s\n", item.Assignment.RoleName, item.Assignment.HumanID)
			refs = append(refs, item.RecordID)
		}
	}
	refs = uniqueSorted(refs)
	value := CommunicationDraft{
		Kind:       CommunicationSituationReport,
		Subject:    "Situation Report — " + picture.Incident.Title,
		Body:       strings.TrimSpace(b.String()),
		Audience:   append([]string(nil), audience...),
		Channels:   append([]string(nil), channels...),
		SourceRefs: refs,
		CadenceID:  cadenceID,
	}
	return value, ValidateCommunicationDraft(value)
}

func validCommunicationKind(kind string) bool {
	switch kind {
	case CommunicationSituationReport, CommunicationRegulatorNotice, CommunicationIndividualNotice, CommunicationExecutiveUpdate, CommunicationStaffCommunication, CommunicationHoldingStatement, CommunicationClosurePIR:
		return true
	default:
		return false
	}
}

func validateStringSet(label string, values []string, minimum, maximum int) error {
	if len(values) < minimum || len(values) > maximum {
		return fmt.Errorf("%s count is outside supported bounds", label)
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return fmt.Errorf("%s contains blank value", label)
		}
		if _, ok := seen[trimmed]; ok {
			return fmt.Errorf("%s contains duplicate value", label)
		}
		seen[trimmed] = struct{}{}
	}
	return nil
}

func uniqueSorted(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			set[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
