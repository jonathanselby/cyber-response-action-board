package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	ActionStatusNotStarted = "not_started"
	ActionStatusInProgress = "in_progress"
	ActionStatusComplete   = "complete"
	ActionStatusSkipped    = "skipped"
)

type ChecklistTemplate struct {
	Name      string                  `json:"name"`
	SourceRef string                  `json:"source_ref"`
	Steps     []ChecklistTemplateStep `json:"steps"`
}

type ChecklistTemplateStep struct {
	StepID              string   `json:"step_id"`
	Title               string   `json:"title"`
	DefaultOwnerHumanID string   `json:"default_owner_human_id,omitempty"`
	DefaultRole         string   `json:"default_role,omitempty"`
	Priority            string   `json:"priority"`
	DueAfterMinutes     int      `json:"due_after_minutes,omitempty"`
	Notes               string   `json:"notes,omitempty"`
	EvidenceRefs        []string `json:"evidence_refs,omitempty"`
}

type ChecklistAction struct {
	RecordID string
	Action   Action
}

func ValidateAction(action Action) error {
	if strings.TrimSpace(action.Title) == "" {
		return errors.New("action title is required")
	}
	if strings.TrimSpace(action.OwnerHumanID) == "" {
		return errors.New("action owner is required")
	}
	if strings.TrimSpace(action.Priority) == "" {
		return errors.New("action priority is required")
	}
	if strings.TrimSpace(action.SourceRef) == "" {
		return errors.New("action source reference is required")
	}
	switch action.Status {
	case ActionStatusNotStarted, ActionStatusInProgress, ActionStatusComplete, ActionStatusSkipped:
	default:
		return errors.New("invalid action status")
	}
	if action.Status == ActionStatusSkipped {
		if strings.TrimSpace(action.SkipReason) == "" {
			return errors.New("skipped action requires reason")
		}
	} else if strings.TrimSpace(action.SkipReason) != "" {
		return errors.New("skip reason is valid only for skipped action")
	}
	if action.DueAt != nil && action.DueAt.IsZero() {
		return errors.New("action due time cannot be zero")
	}
	if (action.ChecklistTemplateID == "") != (action.ChecklistStepID == "") {
		return errors.New("checklist action requires both template and step identifiers")
	}
	for _, ref := range action.EvidenceRefs {
		if strings.TrimSpace(ref) == "" {
			return errors.New("action evidence references cannot be blank")
		}
	}
	return nil
}

func ActionOverdue(action Action, now time.Time) bool {
	if action.DueAt == nil || now.IsZero() {
		return false
	}
	if action.Status == ActionStatusComplete || action.Status == ActionStatusSkipped {
		return false
	}
	return now.After(action.DueAt.UTC())
}

func DecodeChecklistTemplate(raw []byte) (ChecklistTemplate, error) {
	var template ChecklistTemplate
	if err := decodeStrict(raw, &template); err != nil {
		return ChecklistTemplate{}, err
	}
	if err := ValidateChecklistTemplate(template); err != nil {
		return ChecklistTemplate{}, err
	}
	return template, nil
}

func ValidateChecklistTemplate(template ChecklistTemplate) error {
	if strings.TrimSpace(template.Name) == "" || strings.TrimSpace(template.SourceRef) == "" {
		return errors.New("checklist template name and source reference are required")
	}
	if len(template.Steps) == 0 {
		return errors.New("checklist template requires at least one step")
	}
	seen := make(map[string]struct{}, len(template.Steps))
	for _, step := range template.Steps {
		if strings.TrimSpace(step.StepID) == "" || strings.TrimSpace(step.Title) == "" || strings.TrimSpace(step.Priority) == "" {
			return errors.New("checklist step id, title and priority are required")
		}
		if step.DueAfterMinutes < 0 {
			return errors.New("checklist step due offset cannot be negative")
		}
		if _, exists := seen[step.StepID]; exists {
			return fmt.Errorf("duplicate checklist step id %q", step.StepID)
		}
		seen[step.StepID] = struct{}{}
		for _, ref := range step.EvidenceRefs {
			if strings.TrimSpace(ref) == "" {
				return errors.New("checklist evidence references cannot be blank")
			}
		}
	}
	return nil
}

func InstantiateChecklist(templateID, activationID string, template ChecklistTemplate, ownerOverrides map[string]string, activatedAt time.Time) ([]ChecklistAction, error) {
	if err := ValidateChecklistTemplate(template); err != nil {
		return nil, err
	}
	if strings.TrimSpace(templateID) == "" || strings.TrimSpace(activationID) == "" || activatedAt.IsZero() {
		return nil, errors.New("template id, activation id and activation time are required")
	}
	result := make([]ChecklistAction, 0, len(template.Steps))
	for _, step := range template.Steps {
		owner := strings.TrimSpace(step.DefaultOwnerHumanID)
		if override := strings.TrimSpace(ownerOverrides[step.StepID]); override != "" {
			owner = override
		}
		if owner == "" {
			return nil, fmt.Errorf("checklist step %q requires an owner", step.StepID)
		}
		action := Action{
			Title:               step.Title,
			OwnerHumanID:        owner,
			Status:              ActionStatusNotStarted,
			Priority:            step.Priority,
			SourceRef:           template.SourceRef,
			Notes:               step.Notes,
			EvidenceRefs:        append([]string(nil), step.EvidenceRefs...),
			ChecklistTemplateID: templateID,
			ChecklistStepID:     step.StepID,
		}
		if step.DueAfterMinutes > 0 {
			due := activatedAt.UTC().Add(time.Duration(step.DueAfterMinutes) * time.Minute)
			action.DueAt = &due
		}
		if err := ValidateAction(action); err != nil {
			return nil, fmt.Errorf("checklist step %q: %w", step.StepID, err)
		}
		result = append(result, ChecklistAction{RecordID: activationID + ":" + step.StepID, Action: action})
	}
	return result, nil
}
