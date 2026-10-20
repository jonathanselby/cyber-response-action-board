package app

import (
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

func TestCommonOperatingPictureFiltersUnauthorisedCompartments(t *testing.T) {
	now := time.Now().UTC()
	picture := domain.OperatingPicture{
		Facts: []domain.FactView{
			{RecordID: "F-G", Compartment: "GENERAL"},
			{RecordID: "F-L", Compartment: "LEGAL"},
			{RecordID: "F-H", Compartment: "HR"},
		},
		Actions: []domain.ActionView{
			{RecordID: "A-G", Compartment: "GENERAL"},
			{RecordID: "A-L", Compartment: "LEGAL"},
			{RecordID: "A-H", Compartment: "HR"},
		},
		Pending: []domain.PendingProposal{
			{RecordID: "P-G", Compartment: "GENERAL"},
			{RecordID: "P-L", Compartment: "LEGAL"},
			{RecordID: "P-H", Compartment: "HR"},
		},
		Timeline: []domain.TimelineEntry{
			{RecordID: "T-G", Compartment: "GENERAL", RecordedAt: now},
			{RecordID: "T-L", Compartment: "LEGAL", RecordedAt: now},
			{RecordID: "T-H", Compartment: "HR", RecordedAt: now},
		},
	}

	general := filterOperatingPicture(picture, "GENERAL")
	if len(general.Facts) != 1 || general.Facts[0].RecordID != "F-G" || len(general.Actions) != 1 || general.Actions[0].RecordID != "A-G" || len(general.Pending) != 1 || general.Pending[0].RecordID != "P-G" || len(general.Timeline) != 1 || general.Timeline[0].RecordID != "T-G" {
		t.Fatalf("GENERAL picture leaked restricted content: %+v", general)
	}

	legal := filterOperatingPicture(picture, "LEGAL")
	if len(legal.Facts) != 2 || len(legal.Actions) != 2 || len(legal.Pending) != 2 || len(legal.Timeline) != 2 {
		t.Fatalf("LEGAL picture should contain GENERAL + LEGAL only: %+v", legal)
	}
	for _, fact := range legal.Facts {
		if fact.Compartment == "HR" {
			t.Fatal("LEGAL picture leaked HR fact")
		}
	}
}
