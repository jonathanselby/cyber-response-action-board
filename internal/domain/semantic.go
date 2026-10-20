package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

const (
	TypeIncidentDeclaration        = "incident.declaration"
	TypeFact                       = "incident.fact"
	TypeUncertainty                = "incident.uncertainty"
	TypeDecision                   = "incident.decision"
	TypeAction                     = "incident.action"
	TypeOperationalRoleAssignment = "incident.operational_role_assignment"
)

type Scope struct {
	IncidentID  string
	Mode        string
	Compartment string
}

type IncidentDeclaration struct {
	Title    string `json:"title"`
	Summary  string `json:"summary"`
	Severity string `json:"severity"`
}

type Fact struct {
	Statement  string          `json:"statement"`
	Source     string          `json:"source,omitempty"`
	OccurredAt *time.Time      `json:"occurred_at,omitempty"`
	FlaggedAt  *time.Time      `json:"flagged_at,omitempty"`
	Awareness  []AwarenessTime `json:"awareness,omitempty"`
}

type Uncertainty struct {
	Kind      string `json:"kind"`
	Statement string `json:"statement"`
}

type Decision struct {
	Decision  string `json:"decision"`
	Rationale string `json:"rationale,omitempty"`
}

type Action struct {
	Title               string     `json:"title"`
	OwnerHumanID        string     `json:"owner_human_id"`
	Status              string     `json:"status"`
	Priority            string     `json:"priority"`
	DueAt               *time.Time `json:"due_at,omitempty"`
	SourceRef           string     `json:"source_ref"`
	Notes               string     `json:"notes,omitempty"`
	EvidenceRefs        []string   `json:"evidence_refs,omitempty"`
	SkipReason          string     `json:"skip_reason,omitempty"`
	ChecklistTemplateID string     `json:"checklist_template_id,omitempty"`
	ChecklistStepID     string     `json:"checklist_step_id,omitempty"`
}

type OperationalRoleAssignment struct {
	RoleName string `json:"role_name"`
	HumanID  string `json:"human_id"`
	Deputy   bool   `json:"deputy,omitempty"`
}

type GovernedData struct {
	Version string          `json:"version"`
	Payload json.RawMessage `json:"payload"`
}

func RecordFact(scope Scope, recordID, maker string, now time.Time, fact Fact) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if strings.TrimSpace(fact.Statement) == "" {
		return ledger.Record{}, errors.New("fact statement is required")
	}
	if err := ValidateFactTiming(fact); err != nil {
		return ledger.Record{}, err
	}
	return immediateRecord(scope, TypeFact, recordID, maker, now, fact)
}

func RecordUncertaintyItem(scope Scope, recordID, maker string, now time.Time, item Uncertainty) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if item.Kind != "unknown" && item.Kind != "hypothesis" {
		return ledger.Record{}, errors.New("uncertainty kind must be unknown or hypothesis")
	}
	if strings.TrimSpace(item.Statement) == "" {
		return ledger.Record{}, errors.New("uncertainty statement is required")
	}
	return immediateRecord(scope, TypeUncertainty, recordID, maker, now, item)
}

func ProposeIncidentDeclaration(scope Scope, recordID, maker string, now time.Time, declaration IncidentDeclaration) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if strings.TrimSpace(declaration.Title) == "" || strings.TrimSpace(declaration.Summary) == "" || strings.TrimSpace(declaration.Severity) == "" {
		return ledger.Record{}, errors.New("incident title, summary and severity are required")
	}
	return proposedRecord(scope, TypeIncidentDeclaration, recordID, maker, now, declaration)
}

func ProposeDecision(scope Scope, recordID, maker string, now time.Time, decision Decision) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if strings.TrimSpace(decision.Decision) == "" {
		return ledger.Record{}, errors.New("decision text is required")
	}
	return proposedRecord(scope, TypeDecision, recordID, maker, now, decision)
}

func ProposeAction(scope Scope, recordID, maker string, now time.Time, action Action) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if err := ValidateAction(action); err != nil {
		return ledger.Record{}, err
	}
	if action.DueAt != nil {
		due := action.DueAt.UTC()
		action.DueAt = &due
	}
	return proposedRecord(scope, TypeAction, recordID, maker, now, action)
}

func ProposeOperationalRoleAssignment(scope Scope, recordID, maker string, now time.Time, assignment OperationalRoleAssignment) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if strings.TrimSpace(assignment.RoleName) == "" || strings.TrimSpace(assignment.HumanID) == "" {
		return ledger.Record{}, errors.New("operational role name and human id are required")
	}
	return proposedRecord(scope, TypeOperationalRoleAssignment, recordID, maker, now, assignment)
}

func Decide(proposal ledger.Record, expectedVersion, confirmer, outcome string, now time.Time) (ledger.Record, error) {
	if proposal.WriteSemantic != "governed" || proposal.Status != "proposed" {
		return ledger.Record{}, errors.New("decision requires a proposed governed record")
	}
	if strings.TrimSpace(confirmer) == "" || confirmer == proposal.Maker {
		return ledger.Record{}, errors.New("decision requires a different named confirmer")
	}
	if outcome != "confirmed" && outcome != "rejected" {
		return ledger.Record{}, errors.New("decision outcome must be confirmed or rejected")
	}
	if now.IsZero() {
		return ledger.Record{}, errors.New("decision time is required")
	}
	data, err := DecodeGovernedData(proposal.Data)
	if err != nil {
		return ledger.Record{}, err
	}
	if expectedVersion == "" || data.Version != expectedVersion {
		return ledger.Record{}, errors.New("proposal version changed")
	}

	decided := proposal
	decided.Timestamp = now.UTC()
	decided.Status = outcome
	decided.Confirmer = confirmer
	decided.MakerSignature = ledger.Signature{MissingReason: "maker authenticated on immutable proposal entry"}
	decided.ConfirmerSignature = ledger.Signature{MissingReason: "confirmer authenticated by CRAB session"}
	if err := ledger.ValidateRecord(decided); err != nil {
		return ledger.Record{}, err
	}
	return decided, nil
}

func ProposalVersion(record ledger.Record) (string, error) {
	if record.WriteSemantic != "governed" || record.Status != "proposed" {
		return "", errors.New("record is not a proposal")
	}
	data, err := DecodeGovernedData(record.Data)
	if err != nil {
		return "", err
	}
	return data.Version, nil
}

func DecodeGovernedData(raw []byte) (GovernedData, error) {
	var data GovernedData
	if err := decodeStrict(raw, &data); err != nil {
		return GovernedData{}, err
	}
	if data.Version == "" || len(data.Payload) == 0 || !json.Valid(data.Payload) {
		return GovernedData{}, errors.New("invalid governed payload")
	}
	actual, err := payloadVersion(data.Payload)
	if err != nil {
		return GovernedData{}, err
	}
	if actual != data.Version {
		return GovernedData{}, errors.New("governed payload version mismatch")
	}
	return data, nil
}

func DecodePayload[T any](record ledger.Record) (T, error) {
	var zero T
	raw := record.Data
	if record.WriteSemantic == "governed" {
		data, err := DecodeGovernedData(raw)
		if err != nil {
			return zero, err
		}
		raw = data.Payload
	}
	var value T
	if err := decodeStrict(raw, &value); err != nil {
		return zero, err
	}
	return value, nil
}

func immediateRecord(scope Scope, recordType, recordID, maker string, now time.Time, payload any) (ledger.Record, error) {
	if strings.TrimSpace(recordID) == "" || strings.TrimSpace(maker) == "" || now.IsZero() {
		return ledger.Record{}, errors.New("record id, maker and time are required")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ledger.Record{}, err
	}
	record := ledger.Record{
		SchemaVersion:  1,
		Timestamp:      now.UTC(),
		TimeStatus:     "trusted",
		IncidentID:     scope.IncidentID,
		IncidentMode:   scope.Mode,
		WriteSemantic:  "record",
		RecordType:     recordType,
		RecordID:       recordID,
		Status:         "recorded",
		Maker:          maker,
		Compartment:    scope.Compartment,
		Data:           raw,
		MakerSignature: ledger.Signature{MissingReason: "maker authenticated by CRAB session; no per-record signing key"},
	}
	if err := ledger.ValidateRecord(record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func proposedRecord(scope Scope, recordType, recordID, maker string, now time.Time, payload any) (ledger.Record, error) {
	if strings.TrimSpace(recordID) == "" || strings.TrimSpace(maker) == "" || now.IsZero() {
		return ledger.Record{}, errors.New("record id, maker and time are required")
	}
	payloadRaw, err := json.Marshal(payload)
	if err != nil {
		return ledger.Record{}, err
	}
	version, err := payloadVersion(payloadRaw)
	if err != nil {
		return ledger.Record{}, err
	}
	wrapped, err := json.Marshal(GovernedData{Version: version, Payload: payloadRaw})
	if err != nil {
		return ledger.Record{}, err
	}
	record := ledger.Record{
		SchemaVersion:  1,
		Timestamp:      now.UTC(),
		TimeStatus:     "trusted",
		IncidentID:     scope.IncidentID,
		IncidentMode:   scope.Mode,
		WriteSemantic:  "governed",
		RecordType:     recordType,
		RecordID:       recordID,
		Status:         "proposed",
		Maker:          maker,
		Compartment:    scope.Compartment,
		Data:           wrapped,
		MakerSignature: ledger.Signature{MissingReason: "maker authenticated by CRAB session; no per-record signing key"},
	}
	if err := ledger.ValidateRecord(record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func payloadVersion(raw []byte) (string, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return "", err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return "", errors.New("multiple JSON values")
		}
		return "", err
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

func validateScope(scope Scope) error {
	if strings.TrimSpace(scope.IncidentID) == "" {
		return errors.New("incident id is required")
	}
	if scope.Mode != "live" && scope.Mode != "exercise" {
		return errors.New("incident mode must be live or exercise")
	}
	if strings.TrimSpace(scope.Compartment) == "" {
		return errors.New("compartment is required")
	}
	return nil
}

func decodeStrict(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}
