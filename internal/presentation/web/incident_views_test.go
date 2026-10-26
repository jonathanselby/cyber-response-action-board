package web

import (
	"strings"
	"testing"
)

func TestActionViewShowsSemanticFieldsAndOverdueState(t *testing.T) {
	s := withRuntime(t, &fakeRuntime{bootstrapped: true})
	body := get(t, s, "/incident/INC-1").Body.String()
	for _, want := range []string{"Critical actions", "Isolate affected endpoints", "carol", "IN PROGRESS", "P1", "OVERDUE", "RUNBOOK-RANSOMWARE-4.2", "checklist TPL-RANSOMWARE", "Skip"} {
		if !strings.Contains(body, want) {
			t.Fatalf("action view missing %q", want)
		}
	}
	lower := strings.ToLower(body)
	if strings.Contains(lower, "generic item") || strings.Contains(lower, "crud") || strings.Contains(body, "incident.action") {
		t.Fatal("action UX collapsed into generic storage language")
	}
}

func TestTimelineViewKeepsTimeConceptsSeparate(t *testing.T) {
	s := withRuntime(t, &fakeRuntime{bootstrapped: true})
	body := get(t, s, "/incident/INC-1").Body.String()
	i := strings.Index(body, `id="timeline"`)
	if i < 0 {
		t.Fatal("timeline section missing")
	}
	timeline := body[i:]
	for _, want := range []string{"Recorded", "Occurred", "Flagged", "20 Oct 2026 01:00 UTC", "20 Oct 2026 01:10 UTC", "20 Oct 2026 01:15 UTC", "Fact"} {
		if !strings.Contains(timeline, want) {
			t.Fatalf("timeline missing %q", want)
		}
	}
}

func TestBoardKeepsFactsUnknownsAndHypothesesDistinct(t *testing.T) {
	s := withRuntime(t, &fakeRuntime{bootstrapped: true})
	body := get(t, s, "/incident/INC-1").Body.String()
	for _, want := range []string{">FACT<", ">UNKNOWN<", ">HYPOTHESIS<", "Was data exfiltrated?", "Entry via VPN appliance", "not confirmed"} {
		if !strings.Contains(body, want) {
			t.Fatalf("board missing %q", want)
		}
	}
	facts := body[strings.Index(body, "<h3>Facts</h3>"):strings.Index(body, "<h3>Unknowns</h3>")]
	if strings.Contains(facts, "Entry via VPN appliance") {
		t.Fatal("hypothesis rendered as a fact (AC-70)")
	}
}

func TestBoardCarriesHandoverSections(t *testing.T) {
	s := withRuntime(t, &fakeRuntime{bootstrapped: true})
	body := get(t, s, "/incident/INC-1").Body.String()
	for _, want := range []string{"Situation now", "What we know", "Who is in charge", "Critical actions", "Pending decisions", "Active clocks", "Communications commitments", "Recent change", "Next objectives", "Record handover", "Incident commander"} {
		if !strings.Contains(body, want) {
			t.Fatalf("board missing handover section %q (UX 16)", want)
		}
	}
}
