package web

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

func TestActionViewShowsSemanticFieldsAndOverdueState(t *testing.T) {
	due := time.Date(2026, 10, 20, 4, 0, 0, 0, time.UTC)
	actions := []domain.ActionView{{
		RecordID: "ACT-1",
		Action: domain.Action{
			Title:               "Isolate affected endpoints",
			OwnerHumanID:        "carol",
			Status:              domain.ActionStatusInProgress,
			Priority:            "P1",
			DueAt:               &due,
			SourceRef:           "RUNBOOK-RANSOMWARE-4.2",
			Notes:               "Preserve volatile evidence first.",
			EvidenceRefs:        []string{"EVIDENCE-42"},
			ChecklistTemplateID: "TPL-RANSOMWARE",
			ChecklistStepID:     "isolate",
		},
		Overdue: true,
	}}
	var out bytes.Buffer
	if err := RenderActions(&out, actions); err != nil {
		t.Fatal(err)
	}
	body := out.String()
	for _, want := range []string{"Actions", "Isolate affected endpoints", "Owner", "carol", "Status", "in_progress", "Priority", "P1", "Overdue", "Source", "RUNBOOK-RANSOMWARE-4.2", "TPL-RANSOMWARE / isolate", "EVIDENCE-42"} {
		if !strings.Contains(body, want) {
			t.Fatalf("Action view missing %q: %s", want, body)
		}
	}
	if strings.Contains(strings.ToLower(body), "generic item") || strings.Contains(strings.ToLower(body), "crud") {
		t.Fatalf("Action UX collapsed into generic storage language: %s", body)
	}
}

func TestTimelineViewKeepsTimeConceptsSeparate(t *testing.T) {
	occurred := time.Date(2026, 10, 20, 1, 0, 0, 0, time.UTC)
	flagged := occurred.Add(10 * time.Minute)
	recorded := flagged.Add(5 * time.Minute)
	aware := flagged.Add(2 * time.Minute)
	entries := []domain.TimelineEntry{{
		RecordType: "incident.fact",
		Governance: "recorded",
		Summary:    "Payroll files encrypted",
		OccurredAt: &occurred,
		FlaggedAt:  &flagged,
		RecordedAt: recorded,
		Awareness:  []domain.AwarenessTime{{Subject: "Incident Major", At: aware}},
	}}
	var out bytes.Buffer
	if err := RenderTimeline(&out, entries); err != nil {
		t.Fatal(err)
	}
	body := out.String()
	for _, want := range []string{"Occurred", "Flagged", "Recorded", "Aware — Incident Major", "incident.fact / recorded"} {
		if !strings.Contains(body, want) {
			t.Fatalf("Timeline view missing %q: %s", want, body)
		}
	}
}
