package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

const (
	TypeObligationDefinition = "system.obligation_definition"
	TypeObligationTrigger    = "incident.obligation_trigger"
	TypeObligationChange     = "incident.obligation_change"

	ObligationUnitElapsedHours = "elapsed_hours"
	ObligationUnitCalendarDays = "calendar_days"
	ObligationUnitBusinessDays = "business_days"

	ObligationLifecycleProvisional = "provisional"
	ObligationLifecycleFirm        = "firm"

	ObligationStatusActive = "active"
	ObligationStatusWaived = "waived"
)

type BusinessCalendar struct {
	Timezone   string   `json:"timezone"`
	Weekend    []string `json:"weekend,omitempty"`
	Holidays   []string `json:"holidays,omitempty"`
	Adjustment string   `json:"adjustment,omitempty"`
}

type ObligationDefinition struct {
	Name                   string           `json:"name"`
	TriggerField           string           `json:"trigger_field"`
	Duration               int              `json:"duration"`
	Unit                   string           `json:"unit"`
	Calendar               BusinessCalendar `json:"calendar"`
	ApplicabilityCondition string           `json:"applicability_condition"`
	Recipient              string           `json:"recipient"`
	Owner                  string           `json:"owner"`
	AuthoritySource        string           `json:"authority_source"`
	WarningBeforeMinutes   []int            `json:"warning_before_minutes,omitempty"`
}

type ObligationTrigger struct {
	DefinitionID        string    `json:"definition_id"`
	SourceRecordID      string    `json:"source_record_id"`
	SourceField         string    `json:"source_field"`
	TriggerAt           time.Time `json:"trigger_at"`
	ApplicabilityReason string    `json:"applicability_reason"`
}

type ObligationChange struct {
	ClockID                 string     `json:"clock_id"`
	Kind                    string     `json:"kind"`
	AuthoritySource         string     `json:"authority_source"`
	Reason                  string     `json:"reason"`
	NewDeadline             *time.Time `json:"new_deadline,omitempty"`
	ReplacementDefinitionID string     `json:"replacement_definition_id,omitempty"`
}

type ObligationDeadline struct {
	Deadline time.Time
	Adjusted bool
}

type ObligationClockView struct {
	IncidentID           string
	ClockID              string
	DefinitionID         string
	Name                 string
	Lifecycle            string
	Status               string
	TriggerAt            time.Time
	SourceRecordID       string
	SourceField          string
	OriginalDefinitionID string
	OriginalDeadline     time.Time
	Deadline             time.Time
	Recipient            string
	Owner                string
	AuthoritySource      string
	LastChangeKind       string
	LastChangeReason     string
	TimeStatus           string
	Simulated            bool
	Overdue              bool
}

func ValidateObligationDefinition(def ObligationDefinition) error {
	if strings.TrimSpace(def.Name) == "" || strings.TrimSpace(def.TriggerField) == "" {
		return errors.New("obligation name and trigger field are required")
	}
	if def.Duration < 0 {
		return errors.New("obligation duration cannot be negative")
	}
	switch def.Unit {
	case ObligationUnitElapsedHours, ObligationUnitCalendarDays, ObligationUnitBusinessDays:
	default:
		return errors.New("obligation unit must be elapsed_hours, calendar_days or business_days")
	}
	if strings.TrimSpace(def.Calendar.Timezone) == "" {
		return errors.New("obligation timezone is required")
	}
	loc, err := time.LoadLocation(def.Calendar.Timezone)
	if err != nil {
		return fmt.Errorf("obligation timezone: %w", err)
	}
	if _, err := obligationWeekend(def.Calendar.Weekend); err != nil {
		return err
	}
	if _, err := obligationHolidays(def.Calendar.Holidays, loc); err != nil {
		return err
	}
	if err := validateObligationAdjustment(def.Calendar.Adjustment); err != nil {
		return err
	}
	if strings.TrimSpace(def.ApplicabilityCondition) == "" {
		return errors.New("obligation applicability condition is required")
	}
	if strings.TrimSpace(def.Recipient) == "" || strings.TrimSpace(def.Owner) == "" || strings.TrimSpace(def.AuthoritySource) == "" {
		return errors.New("obligation recipient, owner and authority source are required")
	}
	if len(def.WarningBeforeMinutes) > 16 {
		return errors.New("obligation may configure at most 16 warning wake-ups")
	}
	seenWarnings := map[int]struct{}{}
	for _, minutes := range def.WarningBeforeMinutes {
		if minutes <= 0 {
			return errors.New("obligation warning minutes must be positive")
		}
		if _, exists := seenWarnings[minutes]; exists {
			return errors.New("duplicate obligation warning")
		}
		seenWarnings[minutes] = struct{}{}
	}
	return nil
}

// CalculateObligationDeadline treats trigger as an instant and applies the
// configured business timezone to calendar arithmetic. This remains stable
// after PostgreSQL timestamptz normalises the same instant to UTC.
func CalculateObligationDeadline(def ObligationDefinition, trigger time.Time) (ObligationDeadline, error) {
	if err := ValidateObligationDefinition(def); err != nil {
		return ObligationDeadline{}, err
	}
	if trigger.IsZero() {
		return ObligationDeadline{}, errors.New("obligation trigger time is required")
	}
	loc, _ := time.LoadLocation(def.Calendar.Timezone)
	current := trigger.In(loc)
	weekend, _ := obligationWeekend(def.Calendar.Weekend)
	holidays, _ := obligationHolidays(def.Calendar.Holidays, loc)

	var deadline time.Time
	var err error
	switch def.Unit {
	case ObligationUnitElapsedHours:
		deadline = current.Add(time.Duration(def.Duration) * time.Hour)
	case ObligationUnitCalendarDays:
		deadline, err = addObligationCalendarDays(current, def.Duration)
	case ObligationUnitBusinessDays:
		deadline = current
		for counted := 0; counted < def.Duration; {
			deadline, err = addObligationCalendarDays(deadline, 1)
			if err != nil {
				return ObligationDeadline{}, err
			}
			if obligationBusinessDay(deadline, weekend, holidays) {
				counted++
			}
		}
	}
	if err != nil {
		return ObligationDeadline{}, err
	}

	adjusted := false
	switch def.Calendar.Adjustment {
	case "", "none":
	case "next_business_day":
		for !obligationBusinessDay(deadline, weekend, holidays) {
			deadline, err = addObligationCalendarDays(deadline, 1)
			if err != nil {
				return ObligationDeadline{}, err
			}
			adjusted = true
		}
	case "previous_business_day":
		for !obligationBusinessDay(deadline, weekend, holidays) {
			deadline, err = addObligationCalendarDays(deadline, -1)
			if err != nil {
				return ObligationDeadline{}, err
			}
			adjusted = true
		}
	}
	return ObligationDeadline{Deadline: deadline, Adjusted: adjusted}, nil
}

func ValidateObligationTrigger(trigger ObligationTrigger) error {
	if strings.TrimSpace(trigger.DefinitionID) == "" || strings.TrimSpace(trigger.SourceRecordID) == "" || strings.TrimSpace(trigger.SourceField) == "" {
		return errors.New("obligation definition, source record and source field are required")
	}
	if trigger.TriggerAt.IsZero() {
		return errors.New("obligation trigger time is required")
	}
	if strings.TrimSpace(trigger.ApplicabilityReason) == "" {
		return errors.New("obligation applicability reason is required")
	}
	return nil
}

func ValidateObligationChange(change ObligationChange) error {
	if strings.TrimSpace(change.ClockID) == "" || strings.TrimSpace(change.AuthoritySource) == "" || strings.TrimSpace(change.Reason) == "" {
		return errors.New("obligation clock, authority source and reason are required")
	}
	switch change.Kind {
	case "amend", "extend":
		if change.NewDeadline == nil || change.NewDeadline.IsZero() {
			return errors.New("obligation amendment or extension requires a new deadline")
		}
		if strings.TrimSpace(change.ReplacementDefinitionID) != "" {
			return errors.New("obligation amendment or extension cannot replace the definition")
		}
	case "waive":
		if change.NewDeadline != nil || strings.TrimSpace(change.ReplacementDefinitionID) != "" {
			return errors.New("obligation waiver cannot set a deadline or replacement definition")
		}
	case "replace":
		if strings.TrimSpace(change.ReplacementDefinitionID) == "" {
			return errors.New("obligation replacement requires a replacement definition")
		}
		if change.NewDeadline != nil {
			return errors.New("replacement deadline is calculated from the replacement definition")
		}
	default:
		return errors.New("obligation change kind must be amend, extend, waive or replace")
	}
	return nil
}

func ProposeObligationDefinition(recordID, maker string, now time.Time, def ObligationDefinition) (ledger.Record, error) {
	if err := ValidateObligationDefinition(def); err != nil {
		return ledger.Record{}, err
	}
	return proposedSystemRecord(TypeObligationDefinition, recordID, maker, now, def)
}

func ProposeObligationTrigger(scope Scope, recordID, maker string, now time.Time, trigger ObligationTrigger) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if err := ValidateObligationTrigger(trigger); err != nil {
		return ledger.Record{}, err
	}
	trigger.TriggerAt = trigger.TriggerAt.Round(0)
	return proposedRecord(scope, TypeObligationTrigger, recordID, maker, now, trigger)
}

func ProposeObligationChange(scope Scope, recordID, maker string, now time.Time, change ObligationChange) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if err := ValidateObligationChange(change); err != nil {
		return ledger.Record{}, err
	}
	if change.NewDeadline != nil {
		deadline := change.NewDeadline.Round(0)
		change.NewDeadline = &deadline
	}
	return proposedRecord(scope, TypeObligationChange, recordID, maker, now, change)
}

func ObligationOverdue(status string, deadline, now time.Time) bool {
	return status == ObligationStatusActive && !deadline.IsZero() && !now.IsZero() && now.After(deadline)
}

func proposedSystemRecord(recordType, recordID, maker string, now time.Time, payload any) (ledger.Record, error) {
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
		TimeStatus:     ledger.TimeTrusted,
		IncidentMode:   "system",
		WriteSemantic:  "governed",
		RecordType:     recordType,
		RecordID:       recordID,
		Status:         "proposed",
		Maker:          maker,
		Compartment:    "SYSTEM",
		Data:           wrapped,
		MakerSignature: ledger.Signature{MissingReason: "maker authenticated by CRAB session; no per-record signing key"},
	}
	if err := ledger.ValidateRecord(record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func validateObligationAdjustment(value string) error {
	switch value {
	case "", "none", "next_business_day", "previous_business_day":
		return nil
	default:
		return errors.New("invalid obligation deadline adjustment")
	}
}

func addObligationCalendarDays(value time.Time, days int) (time.Time, error) {
	result := value.AddDate(0, 0, days)
	if result.Hour() != value.Hour() || result.Minute() != value.Minute() || result.Second() != value.Second() || result.Nanosecond() != value.Nanosecond() {
		return time.Time{}, errors.New("calendar arithmetic lands on a nonexistent local wall time")
	}
	return result, nil
}

func obligationWeekend(values []string) (map[time.Weekday]bool, error) {
	if len(values) == 0 {
		values = []string{"Saturday", "Sunday"}
	}
	result := map[time.Weekday]bool{}
	for _, value := range values {
		matched := false
		for day := time.Sunday; day <= time.Saturday; day++ {
			if day.String() != value {
				continue
			}
			if result[day] {
				return nil, fmt.Errorf("duplicate weekend day %q", value)
			}
			result[day] = true
			matched = true
			break
		}
		if !matched {
			return nil, fmt.Errorf("invalid weekend day %q", value)
		}
	}
	if len(result) == 7 {
		return nil, errors.New("business calendar must contain at least one business weekday")
	}
	return result, nil
}

func obligationHolidays(values []string, loc *time.Location) (map[string]bool, error) {
	result := map[string]bool{}
	for _, value := range values {
		if _, err := time.ParseInLocation("2006-01-02", value, loc); err != nil {
			return nil, fmt.Errorf("holiday %q: %w", value, err)
		}
		if result[value] {
			return nil, fmt.Errorf("duplicate holiday %q", value)
		}
		result[value] = true
	}
	return result, nil
}

func obligationBusinessDay(value time.Time, weekend map[time.Weekday]bool, holidays map[string]bool) bool {
	return !weekend[value.Weekday()] && !holidays[value.Format("2006-01-02")]
}
