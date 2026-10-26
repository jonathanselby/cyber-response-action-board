package app

import (
	"context"
	"testing"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

// T-195: printed time trust is the weaker of the ledger tail and the clock at
// generation, so a PDF never claims more than CRAB can prove.
func TestPrintTimeMarkIsCappedByClock(t *testing.T) {
	incident := &fakePrintIncident{picture: domain.OperatingPicture{Incident: domain.IncidentState{IncidentID: "INC-PRINT", Mode: "live"}}}
	renderer := &fakePDFRenderer{}
	mark := printTestSource()
	mark.TimeStatus = ledger.TimeTrusted
	service := printService(incident, renderer, &fakeArtifactStore{}, mark)
	service.TimeStatus = func() string { return ledger.TimeUntrusted }
	result, err := service.GenerateIncidentBrief(context.Background(), printPrincipal(true), "INC-PRINT", "GENERAL", nil, domain.PageSizeA4, domain.PrintColour, mark.GeneratedAt)
	if err != nil {
		t.Fatal(err)
	}
	if result.Source.TimeStatus != ledger.TimeUntrusted || renderer.brief.Source.TimeStatus != ledger.TimeUntrusted {
		t.Fatalf("print mark not capped: result=%s brief=%s", result.Source.TimeStatus, renderer.brief.Source.TimeStatus)
	}
}
