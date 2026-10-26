package pdf

import (
	"bytes"
	"testing"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

// T-201: restyled PDFs embed the CRAB mark locally, reference nothing remote
// and stay deterministic in both print modes and page sizes.
func TestPrintStyleIsLocalAndDeterministic(t *testing.T) {
	brief := domain.IncidentBrief{Source: testSource(), Compartment: "GENERAL", Incident: domain.IncidentState{IncidentID: "INC-1", Mode: "exercise", Title: "Brief", Summary: "Summary", Severity: "Severity level 2"},
		Actions: []domain.ActionView{{RecordID: "A1", Action: domain.Action{Title: "Act", OwnerHumanID: "carol", Priority: "P1", Status: "open"}}}}
	for _, size := range []string{domain.PageSizeA4, domain.PageSizeLetter} {
		for _, mode := range []string{domain.PrintColour, domain.PrintMonochrome} {
			a, _, err := Renderer{}.RenderIncidentBrief(brief, size, mode)
			if err != nil {
				t.Fatal(err)
			}
			b, _, _ := Renderer{}.RenderIncidentBrief(brief, size, mode)
			if !bytes.Equal(a, b) {
				t.Fatalf("%s/%s not deterministic", size, mode)
			}
			if !bytes.Contains(a, []byte("/Subtype /Image")) {
				t.Fatalf("%s/%s: CRAB mark not embedded", size, mode)
			}
			for _, remote := range [][]byte{[]byte("http://"), []byte("https://"), []byte("/URI")} {
				if bytes.Contains(a, remote) {
					t.Fatalf("%s/%s references a remote resource", size, mode)
				}
			}
		}
	}
}
