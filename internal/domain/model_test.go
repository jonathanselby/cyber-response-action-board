package domain

import (
	"strings"
	"testing"
)

func TestStaticModelDraftSpecsAreVersionedAndLaneBounded(t *testing.T) {
	cases := []string{"retrieval-summary", "situation-report-draft", "board-update-draft", "notification-language-draft", "pir-summary-draft"}
	for _, id := range cases {
		spec, ok := ModelDraftSpecByID(id)
		if !ok || spec.PromptVersion == "" || spec.Purpose == "" || spec.OutputType == "" || spec.InputRule != ModelInputRulePolicyGate || len(spec.AllowedLanes) == 0 || spec.MaxOutputBytes <= 0 {
			t.Fatalf("invalid static model spec %q: %+v", id, spec)
		}
		for _, lane := range spec.AllowedLanes {
			if !lane.Valid() {
				t.Fatalf("spec %q has invalid lane %q", id, lane)
			}
		}
	}
	if spec, _ := ModelDraftSpecByID("board-update-draft"); ModelSpecAllowsLane(spec, ModelLaneComplex) {
		t.Fatal("board update unexpectedly allowed complex lane")
	}
}

func TestModelPromptCarriesStableCitationProvenanceWithoutChangingSources(t *testing.T) {
	spec, _ := ModelDraftSpecByID("retrieval-summary")
	citations := []RetrievalCitation{{CitationID: "corpus:RUNBOOK:abc:lines:1-2", SourceID: "RUNBOOK", Title: "Containment", Locator: "lines:1-2", Passage: "Isolate affected hosts.", Score: 2}}
	first, firstHash, err := BuildModelPrompt(spec, "Summarise containment steps.", citations)
	if err != nil {
		t.Fatal(err)
	}
	second, secondHash, err := BuildModelPrompt(spec, "Summarise containment steps.", citations)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || firstHash != secondHash || !strings.Contains(first, citations[0].CitationID) || !strings.Contains(first, citations[0].Passage) {
		t.Fatalf("prompt provenance unstable hash1=%q hash2=%q prompt=%q", firstHash, secondHash, first)
	}
}

func TestModelOutputValidationFailsClosed(t *testing.T) {
	spec, _ := ModelDraftSpecByID("retrieval-summary")
	if err := ValidateModelOutput(spec, "  "); err == nil {
		t.Fatal("empty model output unexpectedly accepted")
	}
	if err := ValidateModelOutput(spec, strings.Repeat("x", spec.MaxOutputBytes+1)); err == nil {
		t.Fatal("oversized model output unexpectedly accepted")
	}
	if err := ValidateModelOutput(spec, "draft\x00text"); err == nil {
		t.Fatal("NUL-containing model output unexpectedly accepted")
	}
	if err := ValidateModelOutput(spec, "Untrusted draft based on supplied sources."); err != nil {
		t.Fatal(err)
	}
}
