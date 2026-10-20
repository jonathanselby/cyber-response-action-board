package pdf

import (
	"bytes"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

func testSource() domain.PrintSourceMark { return domain.PrintSourceMark{LedgerSequence: 42, LedgerHash: "0123456789abcdef0123456789abcdef", ConfigurationVersion: "cfg-7", GeneratedAt: time.Date(2026,10,20,7,0,0,0,time.UTC), TimeStatus: "trusted"} }

func TestIncidentBriefRendersA4AndLetterLocally(t *testing.T) {
	brief := domain.IncidentBrief{Source:testSource(), Incident:domain.IncidentState{IncidentID:"INC-1",Mode:"live",Title:"Bafflewick",Summary:"Identity service unavailable",Severity:"High"}, Compartment:"GENERAL", Actions:[]domain.ActionView{{RecordID:"A-1",Action:domain.Action{Title:"Validate recovery",OwnerHumanID:"alice",Status:"in_progress",Priority:"high"}}}}
	for _, size := range []string{domain.PageSizeA4, domain.PageSizeLetter} {
		raw, pages, err := (Renderer{}).RenderIncidentBrief(brief,size,domain.PrintMonochrome)
		if err != nil { t.Fatalf("%s: %v", size, err) }
		if pages < 1 || pages > 2 || !bytes.HasPrefix(raw, []byte("%PDF-")) { t.Fatalf("%s pages=%d pdf=%q", size,pages,raw[:min(len(raw),8)]) }
	}
}

func TestExerciseMarkerAndMonochromeDoNotDependOnColour(t *testing.T) {
	pack := domain.EmergencyResponsePack{Source:testSource(),Organisation:"CRAB Test",Mode:"exercise",Sections:[]domain.PackSection{{Title:"Runbook",Body:"Use the printed runbook."}},Forms:domain.DefaultContingencyForms()}
	mono, monoPages, err := (Renderer{}).RenderEmergencyResponsePack(pack,domain.PageSizeA4,domain.PrintMonochrome)
	if err != nil { t.Fatal(err) }
	colour, colourPages, err := (Renderer{}).RenderEmergencyResponsePack(pack,domain.PageSizeA4,domain.PrintColour)
	if err != nil { t.Fatal(err) }
	if monoPages != colourPages || monoPages < 6 || len(mono)==0 || len(colour)==0 { t.Fatalf("mono=%d colour=%d", monoPages,colourPages) }
}

func TestRendererIsDeterministicForFixedInputs(t *testing.T) {
	doc := domain.TimelineDocument{Source:testSource(),Incident:domain.IncidentState{IncidentID:"INC-1",Mode:"live",Title:"Bafflewick"},Compartment:"GENERAL",Entries:[]domain.TimelineEntry{{Sequence:1,RecordID:"R1",RecordType:"incident.fact",Governance:"recorded",Summary:"Service unavailable",RecordedAt:time.Date(2026,10,20,1,0,0,0,time.UTC),TimeStatus:"trusted"}}}
	a, _, err := (Renderer{}).RenderTimeline(doc,domain.PageSizeA4,domain.PrintMonochrome); if err != nil { t.Fatal(err) }
	b, _, err := (Renderer{}).RenderTimeline(doc,domain.PageSizeA4,domain.PrintMonochrome); if err != nil { t.Fatal(err) }
	if !bytes.Equal(a,b) { t.Fatal("fixed print input produced non-deterministic PDF bytes") }
}
