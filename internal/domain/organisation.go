package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

// Organisation configuration (UAT2-BUILD-NOTES 4): small, governed records
// holding organisational policy. Each is a SYSTEM proposal confirmed by a
// different person; the latest confirmed version of each type is current.
const (
	TypeSeverityScale          = "system.severity_scale"
	TypeEmergencyConfiguration = "system.emergency_configuration"
	TypeChecklistTemplate      = "system.checklist_template"
	TypeSinglePersonWindow     = "system.single_person_window"
)

// IsOrganisationType reports whether a record type belongs to the
// organisation configuration family.
func IsOrganisationType(recordType string) bool {
	switch recordType {
	case TypeSeverityScale, TypeEmergencyConfiguration, TypeChecklistTemplate, TypeSinglePersonWindow:
		return true
	}
	return false
}

// ---- severity scale --------------------------------------------------------

type SeverityLevel struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// SeverityScale is ordered from least to most severe.
type SeverityScale struct {
	Levels []SeverityLevel `json:"levels"`
}

const (
	minSeverityLevels = 2
	maxSeverityLevels = 7
)

// DefaultSeverityScale is the seed offered at setup. It is confirmed by the
// two setup people like any other policy and can be changed through
// governance; CRAB does not hard-code it anywhere else.
func DefaultSeverityScale() SeverityScale {
	return SeverityScale{Levels: []SeverityLevel{
		{Label: "Severity level 1", Description: "Least severe"},
		{Label: "Severity level 2"},
		{Label: "Severity level 3"},
		{Label: "Severity level 4"},
		{Label: "Severity level 5", Description: "Most severe"},
	}}
}

func ValidateSeverityScale(scale SeverityScale) error {
	if len(scale.Levels) < minSeverityLevels || len(scale.Levels) > maxSeverityLevels {
		return fmt.Errorf("a severity scale needs between %d and %d levels", minSeverityLevels, maxSeverityLevels)
	}
	seen := map[string]bool{}
	for _, level := range scale.Levels {
		label := strings.TrimSpace(level.Label)
		if label == "" || len(label) > 60 {
			return errors.New("each severity level needs a label of at most 60 characters")
		}
		key := strings.ToLower(label)
		if seen[key] {
			return fmt.Errorf("severity level %q appears twice", label)
		}
		seen[key] = true
		if len(level.Description) > 300 {
			return errors.New("severity level descriptions are limited to 300 characters")
		}
	}
	return nil
}

// Contains reports whether value is exactly one of the scale's labels.
func (s SeverityScale) Contains(value string) bool {
	for _, level := range s.Levels {
		if level.Label == value {
			return true
		}
	}
	return false
}

// Labels returns the labels in order.
func (s SeverityScale) Labels() []string {
	out := make([]string, 0, len(s.Levels))
	for _, level := range s.Levels {
		out = append(out, level.Label)
	}
	return out
}

// ---- emergency configuration ----------------------------------------------

type EmergencyContact struct {
	Name        string `json:"name"`
	Role        string `json:"role"`
	Primary     string `json:"primary"`
	Alternative string `json:"alternative,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

type ExternalParty struct {
	Category    string `json:"category"`
	Name        string `json:"name"`
	Primary     string `json:"primary"`
	Alternative string `json:"alternative,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

type RoleCard struct {
	Title            string `json:"title"`
	Responsibilities string `json:"responsibilities"`
	Deputy           string `json:"deputy,omitempty"`
}

type Procedure struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// EmergencyConfiguration is the emergency-operating information the printed
// pack needs when everything else is unavailable. It is deliberately not an
// asset register or document store.
type EmergencyConfiguration struct {
	Organisation string             `json:"organisation"`
	Contacts     []EmergencyContact `json:"contacts,omitempty"`
	Parties      []ExternalParty    `json:"parties,omitempty"`
	Escalation   []string           `json:"escalation,omitempty"`
	RoleCards    []RoleCard         `json:"role_cards,omitempty"`
	Procedures   []Procedure        `json:"procedures,omitempty"`
}

var partyCategories = map[string]bool{"insurer": true, "regulator": true, "counsel": true, "adviser": true, "supplier": true, "other": true}

const (
	maxEmergencyRows   = 60
	maxProcedures      = 20
	maxProcedureLength = 4000
	maxFieldLength     = 200
)

func ValidateEmergencyConfiguration(c EmergencyConfiguration) error {
	if strings.TrimSpace(c.Organisation) == "" || len(c.Organisation) > maxFieldLength {
		return errors.New("organisation name is required")
	}
	if len(c.Contacts) > maxEmergencyRows || len(c.Parties) > maxEmergencyRows || len(c.Escalation) > maxEmergencyRows || len(c.RoleCards) > maxEmergencyRows {
		return fmt.Errorf("each list is limited to %d entries", maxEmergencyRows)
	}
	short := func(values ...string) bool {
		for _, v := range values {
			if len(v) > maxFieldLength {
				return false
			}
		}
		return true
	}
	for _, v := range c.Contacts {
		if strings.TrimSpace(v.Name) == "" || strings.TrimSpace(v.Role) == "" || strings.TrimSpace(v.Primary) == "" || !short(v.Name, v.Role, v.Primary, v.Alternative, v.Notes) {
			return errors.New("each contact needs a name, a role and a primary route")
		}
	}
	for _, v := range c.Parties {
		if !partyCategories[v.Category] {
			return fmt.Errorf("external party category %q is not one of insurer, regulator, counsel, adviser, supplier or other", v.Category)
		}
		if strings.TrimSpace(v.Name) == "" || strings.TrimSpace(v.Primary) == "" || !short(v.Name, v.Primary, v.Alternative, v.Notes) {
			return errors.New("each external party needs a name and a primary route")
		}
	}
	for _, line := range c.Escalation {
		if strings.TrimSpace(line) == "" || len(line) > 400 {
			return errors.New("escalation steps must be non-empty and at most 400 characters")
		}
	}
	for _, v := range c.RoleCards {
		if strings.TrimSpace(v.Title) == "" || strings.TrimSpace(v.Responsibilities) == "" || len(v.Responsibilities) > maxProcedureLength || !short(v.Title, v.Deputy) {
			return errors.New("each role card needs a title and responsibilities")
		}
	}
	if len(c.Procedures) > maxProcedures {
		return fmt.Errorf("at most %d procedures", maxProcedures)
	}
	for _, v := range c.Procedures {
		if strings.TrimSpace(v.Title) == "" || strings.TrimSpace(v.Body) == "" || len(v.Body) > maxProcedureLength || !short(v.Title) {
			return fmt.Errorf("each procedure needs a title and text of at most %d characters", maxProcedureLength)
		}
	}
	return nil
}

// ---- proposals ---------------------------------------------------------------

func ProposeSeverityScale(recordID, maker string, now time.Time, scale SeverityScale) (ledger.Record, error) {
	if err := ValidateSeverityScale(scale); err != nil {
		return ledger.Record{}, err
	}
	return ProposeSystemGoverned(TypeSeverityScale, recordID, maker, now, scale)
}

func ProposeEmergencyConfiguration(recordID, maker string, now time.Time, c EmergencyConfiguration) (ledger.Record, error) {
	if err := ValidateEmergencyConfiguration(c); err != nil {
		return ledger.Record{}, err
	}
	return ProposeSystemGoverned(TypeEmergencyConfiguration, recordID, maker, now, c)
}

// OrganisationView is one confirmed or pending organisation record.
type OrganisationView struct {
	RecordID    string
	RecordType  string
	Version     string
	Payload     []byte
	Maker       string
	Confirmer   string
	At          time.Time
	TimeStatus  string
	SelfApprove bool
}

// OrganisationRecordKey is the identity under which a confirmed record
// replaces the previous one: one current scale and one current emergency
// configuration; checklist templates and windows by their own identifier.
func OrganisationRecordKey(recordType string, payload []byte) (string, error) {
	switch recordType {
	case TypeSeverityScale, TypeEmergencyConfiguration:
		return "current", nil
	case TypeChecklistTemplate:
		var v struct {
			TemplateID string `json:"template_id"`
		}
		if err := jsonUnmarshalLoose(payload, &v); err != nil || strings.TrimSpace(v.TemplateID) == "" {
			return "", errors.New("checklist template requires template_id")
		}
		return v.TemplateID, nil
	case TypeSinglePersonWindow:
		var v struct {
			WindowID string `json:"window_id"`
		}
		if err := jsonUnmarshalLoose(payload, &v); err != nil || strings.TrimSpace(v.WindowID) == "" {
			return "", errors.New("single-person window requires window_id")
		}
		return v.WindowID, nil
	}
	return "", fmt.Errorf("%s is not organisation configuration", recordType)
}

// ValidateOrganisationPayload checks a stored payload against its type, so a
// projection never holds something the domain would refuse.
func ValidateOrganisationPayload(recordType string, payload []byte) error {
	switch recordType {
	case TypeSeverityScale:
		var v SeverityScale
		if err := decodeStrict(payload, &v); err != nil {
			return err
		}
		return ValidateSeverityScale(v)
	case TypeEmergencyConfiguration:
		var v EmergencyConfiguration
		if err := decodeStrict(payload, &v); err != nil {
			return err
		}
		return ValidateEmergencyConfiguration(v)
	case TypeChecklistTemplate:
		var v GovernedChecklistTemplate
		if err := decodeStrict(payload, &v); err != nil {
			return err
		}
		return ValidateGovernedChecklistTemplate(v)
	case TypeSinglePersonWindow:
		var v SinglePersonWindow
		if err := decodeStrict(payload, &v); err != nil {
			return err
		}
		return ValidateSinglePersonWindowShape(v)
	}
	return fmt.Errorf("%s is not organisation configuration", recordType)
}

func jsonUnmarshalLoose(raw []byte, v any) error { return json.Unmarshal(raw, v) }

// EmergencyPackSections turns the confirmed configuration into printed pack
// sections: tables for people and parties, ordered escalation, role cards and
// procedures.
func EmergencyPackSections(c EmergencyConfiguration) []PackSection {
	var out []PackSection
	if len(c.Contacts) > 0 {
		rows := make([][]string, 0, len(c.Contacts))
		for _, v := range c.Contacts {
			rows = append(rows, []string{v.Role, v.Name, v.Primary, orDash(v.Alternative), v.Notes})
		}
		out = append(out, PackSection{Title: "Emergency contacts", Columns: []string{"Role", "Name", "Primary", "Alternative", "Notes"}, Widths: []float64{0.2, 0.2, 0.2, 0.2, 0.2}, Rows: rows})
	}
	if len(c.Parties) > 0 {
		rows := make([][]string, 0, len(c.Parties))
		for _, v := range c.Parties {
			rows = append(rows, []string{strings.ToUpper(v.Category[:1]) + v.Category[1:], v.Name, v.Primary, orDash(v.Alternative), v.Notes})
		}
		out = append(out, PackSection{Title: "Insurers, regulators and advisers", Columns: []string{"Type", "Name", "Primary", "Alternative", "Notes"}, Widths: []float64{0.14, 0.24, 0.2, 0.2, 0.22}, Rows: rows})
	}
	if len(c.Escalation) > 0 {
		rows := make([][]string, 0, len(c.Escalation))
		for i, v := range c.Escalation {
			rows = append(rows, []string{fmt.Sprint(i + 1), v})
		}
		out = append(out, PackSection{Title: "Escalation route", Columns: []string{"Step", "Escalate to"}, Widths: []float64{0.1, 0.9}, Rows: rows})
	}
	for _, v := range c.RoleCards {
		body := v.Responsibilities
		if v.Deputy != "" {
			body += "\nDeputy: " + v.Deputy
		}
		out = append(out, PackSection{Title: "Role card: " + v.Title, Body: body})
	}
	for _, v := range c.Procedures {
		out = append(out, PackSection{Title: "Procedure: " + v.Title, Body: v.Body})
	}
	return out
}

// BriefContacts lists the people and parties the Incident Brief prints.
func BriefContacts(c EmergencyConfiguration) []PrintContact {
	var out []PrintContact
	for _, v := range c.Contacts {
		out = append(out, PrintContact{Role: v.Role, Name: v.Name, Primary: v.Primary, Alternate: v.Alternative})
	}
	for _, v := range c.Parties {
		out = append(out, PrintContact{Role: strings.ToUpper(v.Category[:1]) + v.Category[1:], Name: v.Name, Primary: v.Primary, Alternate: v.Alternative})
	}
	return out
}

func orDash(v string) string {
	if strings.TrimSpace(v) == "" {
		return "—"
	}
	return v
}
