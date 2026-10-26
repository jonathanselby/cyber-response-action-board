package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

// GovernedChecklistTemplate is an organisation-approved checklist (UAT 2
// item 6). Withdrawal is a new version with Active false; nothing is deleted.
type GovernedChecklistTemplate struct {
	TemplateID string `json:"template_id"`
	Active     bool   `json:"active"`
	ChecklistTemplate
}

var templateIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,59}$`)

const maxChecklistSteps = 40

func ValidateGovernedChecklistTemplate(t GovernedChecklistTemplate) error {
	if !templateIDPattern.MatchString(t.TemplateID) {
		return errors.New("template identifier must be 2 to 60 lower-case letters, digits or hyphens")
	}
	if !t.Active {
		if strings.TrimSpace(t.Name) == "" {
			return errors.New("a withdrawn template still needs its name")
		}
		return nil
	}
	if len(t.Steps) > maxChecklistSteps {
		return errors.New("a checklist template is limited to 40 steps")
	}
	return ValidateChecklistTemplate(t.ChecklistTemplate)
}

func ProposeChecklistTemplate(recordID, maker string, now time.Time, t GovernedChecklistTemplate) (ledger.Record, error) {
	if err := ValidateGovernedChecklistTemplate(t); err != nil {
		return ledger.Record{}, err
	}
	return ProposeSystemGoverned(TypeChecklistTemplate, recordID, maker, now, t)
}
