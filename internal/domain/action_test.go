package domain

import (
	"testing"
	"time"
)

func completeAction() Action {
	return Action{
		Title:        "Isolate affected endpoints",
		OwnerHumanID: "bob",
		Status:       ActionStatusNotStarted,
		Priority:     "P1",
		SourceRef:    "DEC-1",
	}
}

func TestActionRequiresOwnerStatusPriorityAndSource(t *testing.T) {
	action := completeAction()
	for name, mutate := range map[string]func(*Action){
		"owner":    func(a *Action) { a.OwnerHumanID = "" },
		"status":   func(a *Action) { a.Status = "" },
		"priority": func(a *Action) { a.Priority = "" },
		"source":   func(a *Action) { a.SourceRef = "" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := action
			mutate(&candidate)
			if err := ValidateAction(candidate); err == nil {
				t.Fatal("invalid action accepted")
			}
		})
	}
}

func TestSkippedActionRequiresReason(t *testing.T) {
	action := completeAction()
	action.Status = ActionStatusSkipped
	if err := ValidateAction(action); err == nil {
		t.Fatal("skip without reason accepted")
	}
	action.SkipReason = "Not applicable to isolated SaaS service"
	if err := ValidateAction(action); err != nil {
		t.Fatal(err)
	}
}

func TestActionOverdueIsDerived(t *testing.T) {
	due := time.Date(2026, 10, 20, 4, 0, 0, 0, time.UTC)
	action := completeAction()
	action.DueAt = &due
	if ActionOverdue(action, due.Add(-time.Minute)) {
		t.Fatal("action overdue before due time")
	}
	if !ActionOverdue(action, due.Add(time.Minute)) {
		t.Fatal("due action did not derive overdue")
	}
	action.Status = ActionStatusComplete
	if ActionOverdue(action, due.Add(time.Hour)) {
		t.Fatal("completed action remained overdue")
	}
}

func TestChecklistInstantiationCreatesOrdinaryActionsWithProvenance(t *testing.T) {
	activated := time.Date(2026, 10, 20, 4, 0, 0, 0, time.UTC)
	template := ChecklistTemplate{
		Name:      "Ransomware first hour",
		SourceRef: "RUNBOOK-RANSOMWARE-4.2",
		Steps: []ChecklistTemplateStep{
			{StepID: "isolate", Title: "Isolate affected endpoints", Priority: "P1", DueAfterMinutes: 15},
			{StepID: "preserve", Title: "Preserve endpoint images", DefaultOwnerHumanID: "forensics", Priority: "P2"},
		},
	}
	actions, err := InstantiateChecklist("TPL-1", "ACTIVATION-7", template, map[string]string{"isolate": "alice"}, activated)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 2 {
		t.Fatalf("actions=%d", len(actions))
	}
	first := actions[0]
	if first.RecordID != "ACTIVATION-7:isolate" || first.Action.ChecklistTemplateID != "TPL-1" || first.Action.ChecklistStepID != "isolate" || first.Action.SourceRef != template.SourceRef {
		t.Fatalf("checklist provenance lost: %+v", first)
	}
	if first.Action.DueAt == nil || !first.Action.DueAt.Equal(activated.Add(15*time.Minute)) {
		t.Fatalf("due time=%v", first.Action.DueAt)
	}
}

func TestChecklistInstantiationRequiresOwnerForEveryStep(t *testing.T) {
	template := ChecklistTemplate{Name: "x", SourceRef: "runbook", Steps: []ChecklistTemplateStep{{StepID: "a", Title: "Do thing", Priority: "P2"}}}
	if _, err := InstantiateChecklist("TPL", "RUN", template, nil, time.Now().UTC()); err == nil {
		t.Fatal("ownerless checklist step activated")
	}
}
