package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

const (
	TypeRecoveryUpdate = "incident.recovery_update"
	TypeStandDown      = "incident.stand_down"
	TypePIR            = "incident.pir"
	TypeRetention      = "incident.retention"
	TypeClosure        = "incident.closure"

	PhaseActiveResponse = "ACTIVE RESPONSE"
	PhaseRecovery       = "RECOVERY"
	PhaseStoodDown      = "STOOD DOWN / REVIEW"
	PhaseClosureReady   = "CLOSURE READY"
	PhaseClosed         = "CLOSED"
)

var (
	ErrIncidentClosed       = errors.New("incident is closed and read-only")
	ErrClosurePrerequisites = errors.New("incident closure prerequisites are not met")
)

type RecoveryUpdate struct {
	ItemID              string `json:"item_id"`
	Summary             string `json:"summary"`
	OwnerHumanID        string `json:"owner_human_id"`
	RestorationPriority string `json:"restoration_priority"`
	Verified            bool   `json:"verified"`
	ResidualRisk        string `json:"residual_risk,omitempty"`
	HandoffRef          string `json:"handoff_ref,omitempty"`
}

type StandDown struct {
	Reason string `json:"reason"`
}

type PIR struct {
	ExecutiveSummary string   `json:"executive_summary"`
	HumanLessons     []string `json:"human_lessons"`
	LongTermWork     []string `json:"long_term_work,omitempty"`
}

type PIRDraft struct {
	Incident         IncidentState
	ExecutiveSummary string
	Timeline         []TimelineEntry
	Decisions        []DecisionView
	Actions          []ActionView
}

type RetentionDisposition struct {
	RetentionClass           string     `json:"retention_class"`
	Hold                     bool       `json:"hold"`
	HoldReason               string     `json:"hold_reason,omitempty"`
	DispositionEligibleAfter *time.Time `json:"disposition_eligible_after,omitempty"`
	HandoffRef               string     `json:"handoff_ref"`
}

type Closure struct {
	Reason                 string `json:"reason"`
	PrivilegedCompartment  string `json:"privileged_compartment,omitempty"`
	PrivilegedSummary      string `json:"privileged_summary,omitempty"`
	ShareableSummary       string `json:"shareable_summary"`
}

type ClosureDocument struct {
	Reason  string
	Summary string
}

type ClosureOutputs struct {
	Privileged ClosureDocument
	Shareable  ClosureDocument
}

type RecoveryUpdateView struct {
	RecordID    string
	Compartment string
	Update      RecoveryUpdate
	Maker       string
	RecordedAt  time.Time
}

type RecoveryState struct {
	IncidentID      string
	Phase           string
	Updates         []RecoveryUpdateView
	StandDown       *StandDown
	StandDownAt     *time.Time
	PIR             *PIR
	PIRAt           *time.Time
	Retention       *RetentionDisposition
	RetentionAt     *time.Time
	Closure         *Closure
	ClosedAt        *time.Time
	ClosureReady    bool
	UnresolvedCount int
}

func RecordRecoveryUpdate(scope Scope, recordID, maker string, now time.Time, update RecoveryUpdate) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if err := ValidateRecoveryUpdate(update); err != nil {
		return ledger.Record{}, err
	}
	return immediateRecord(scope, TypeRecoveryUpdate, recordID, maker, now, update)
}

func ProposeStandDown(scope Scope, recordID, maker string, now time.Time, value StandDown) (ledger.Record, error) {
	if err := validateMilestoneScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if strings.TrimSpace(value.Reason) == "" {
		return ledger.Record{}, errors.New("stand-down reason is required")
	}
	return proposedRecord(scope, TypeStandDown, recordID, maker, now, value)
}

func ProposePIR(scope Scope, recordID, maker string, now time.Time, value PIR) (ledger.Record, error) {
	if err := validateMilestoneScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if err := ValidatePIR(value); err != nil {
		return ledger.Record{}, err
	}
	return proposedRecord(scope, TypePIR, recordID, maker, now, value)
}

func ProposeRetention(scope Scope, recordID, maker string, now time.Time, value RetentionDisposition) (ledger.Record, error) {
	if err := validateMilestoneScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if err := ValidateRetention(value); err != nil {
		return ledger.Record{}, err
	}
	if value.DispositionEligibleAfter != nil {
		utc := value.DispositionEligibleAfter.UTC()
		value.DispositionEligibleAfter = &utc
	}
	return proposedRecord(scope, TypeRetention, recordID, maker, now, value)
}

func ProposeClosure(scope Scope, recordID, maker string, now time.Time, value Closure) (ledger.Record, error) {
	if err := ValidateClosure(value); err != nil {
		return ledger.Record{}, err
	}
	if err := ValidateClosureScope(scope, value); err != nil {
		return ledger.Record{}, err
	}
	return proposedRecord(scope, TypeClosure, recordID, maker, now, value)
}

func ValidateRecoveryUpdate(value RecoveryUpdate) error {
	if strings.TrimSpace(value.ItemID) == "" || strings.TrimSpace(value.Summary) == "" || strings.TrimSpace(value.OwnerHumanID) == "" || strings.TrimSpace(value.RestorationPriority) == "" {
		return errors.New("recovery item, summary, owner and restoration priority are required")
	}
	return nil
}

func ValidatePIR(value PIR) error {
	if strings.TrimSpace(value.ExecutiveSummary) == "" {
		return errors.New("PIR executive summary is required")
	}
	if len(value.HumanLessons) == 0 {
		return errors.New("PIR requires at least one human lesson")
	}
	for _, lesson := range value.HumanLessons {
		if strings.TrimSpace(lesson) == "" {
			return errors.New("PIR lessons cannot be blank")
		}
	}
	return nil
}

func ValidateRetention(value RetentionDisposition) error {
	if strings.TrimSpace(value.RetentionClass) == "" || strings.TrimSpace(value.HandoffRef) == "" {
		return errors.New("retention class and handoff reference are required")
	}
	if value.Hold && strings.TrimSpace(value.HoldReason) == "" {
		return errors.New("hold reason is required while a hold is active")
	}
	if value.DispositionEligibleAfter != nil && value.DispositionEligibleAfter.IsZero() {
		return errors.New("disposition eligibility time cannot be zero")
	}
	return nil
}

func ValidateClosure(value Closure) error {
	if strings.TrimSpace(value.Reason) == "" || strings.TrimSpace(value.ShareableSummary) == "" {
		return errors.New("closure reason and shareable summary are required")
	}
	privileged := strings.TrimSpace(value.PrivilegedSummary)
	compartment := strings.TrimSpace(value.PrivilegedCompartment)
	if (privileged == "") != (compartment == "") {
		return errors.New("privileged closure summary and compartment must be supplied together")
	}
	if compartment == "GENERAL" {
		return errors.New("privileged closure material cannot use GENERAL compartment")
	}
	return nil
}

func ValidateClosureScope(scope Scope, value Closure) error {
	if err := validateScope(scope); err != nil {
		return err
	}
	if strings.TrimSpace(value.PrivilegedSummary) == "" {
		if scope.Compartment != "GENERAL" {
			return errors.New("shareable-only closure must use GENERAL compartment")
		}
		return nil
	}
	if scope.Compartment != value.PrivilegedCompartment {
		return errors.New("closure record must use its privileged compartment")
	}
	return nil
}

func IsRecoveryType(recordType string) bool {
	switch recordType {
	case TypeRecoveryUpdate, TypeStandDown, TypePIR, TypeRetention, TypeClosure:
		return true
	default:
		return false
	}
}

func ClosurePrerequisites(state RecoveryState) error {
	if state.ClosedAt != nil {
		return ErrIncidentClosed
	}
	if state.StandDownAt == nil || state.PIRAt == nil || state.RetentionAt == nil {
		return ErrClosurePrerequisites
	}
	for _, update := range state.Updates {
		if !update.Update.Verified && strings.TrimSpace(update.Update.HandoffRef) == "" {
			return ErrClosurePrerequisites
		}
	}
	return nil
}

func DeriveOperationalPhase(state RecoveryState) string {
	if state.ClosedAt != nil {
		return PhaseClosed
	}
	if ClosurePrerequisites(state) == nil {
		return PhaseClosureReady
	}
	if state.StandDownAt != nil {
		return PhaseStoodDown
	}
	if len(state.Updates) > 0 {
		return PhaseRecovery
	}
	return PhaseActiveResponse
}

func FinaliseRecoveryState(state RecoveryState) RecoveryState {
	state.UnresolvedCount = 0
	for _, update := range state.Updates {
		if !update.Update.Verified && strings.TrimSpace(update.Update.HandoffRef) == "" {
			state.UnresolvedCount++
		}
	}
	state.ClosureReady = ClosurePrerequisites(state) == nil
	state.Phase = DeriveOperationalPhase(state)
	return state
}

func GeneratePIRDraft(picture OperatingPicture) PIRDraft {
	summary := strings.TrimSpace(picture.Incident.Summary)
	if summary == "" {
		summary = picture.Incident.Title
	}
	if strings.TrimSpace(picture.Incident.Severity) != "" {
		summary = fmt.Sprintf("%s Severity: %s.", strings.TrimSpace(summary), picture.Incident.Severity)
	}
	return PIRDraft{
		Incident:         picture.Incident,
		ExecutiveSummary: strings.TrimSpace(summary),
		Timeline:         append([]TimelineEntry(nil), picture.Timeline...),
		Decisions:        append([]DecisionView(nil), picture.Decisions...),
		Actions:          append([]ActionView(nil), picture.Actions...),
	}
}

func BuildClosureOutputs(value Closure) ClosureOutputs {
	privileged := strings.TrimSpace(value.PrivilegedSummary)
	if privileged == "" {
		privileged = value.ShareableSummary
	}
	return ClosureOutputs{
		Privileged: ClosureDocument{Reason: value.Reason, Summary: privileged},
		Shareable:  ClosureDocument{Reason: value.Reason, Summary: value.ShareableSummary},
	}
}

func validateMilestoneScope(scope Scope) error {
	if err := validateScope(scope); err != nil {
		return err
	}
	if scope.Compartment != "GENERAL" {
		return errors.New("incident recovery milestones must use GENERAL compartment")
	}
	return nil
}
