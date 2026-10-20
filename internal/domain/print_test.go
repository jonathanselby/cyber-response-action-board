package domain

import (
	"testing"
	"time"
)

func TestPaperTranscriptionKeepsPaperTimeSeparateFromDigitalRecordedAt(t *testing.T) {
	paperAt := time.Date(2026, 10, 20, 1, 2, 3, 0, time.UTC)
	transcribedAt := paperAt.Add(5 * time.Hour)
	value := PaperTranscription{Statement: "Firewall isolated by hand", PaperOccurredAt: paperAt, SourceReference: "paper-log-7", TimeConfidence: "untrusted"}
	record, err := RecordPaperTranscription(Scope{IncidentID: "INC-1", Mode: "live", Compartment: "GENERAL"}, "PAPER-1", "alice", transcribedAt, value)
	if err != nil {
		t.Fatal(err)
	}
	if !record.Timestamp.Equal(transcribedAt) || record.Timestamp.Equal(paperAt) {
		t.Fatalf("digital recorded_at was backdated: %s", record.Timestamp)
	}
	if record.TimeStatus != "untrusted" {
		t.Fatalf("time status=%s", record.TimeStatus)
	}
	got, err := DecodePayload[PaperTranscription](record)
	if err != nil {
		t.Fatal(err)
	}
	if !got.PaperOccurredAt.Equal(paperAt) || got.SourceReference != "paper-log-7" {
		t.Fatalf("payload=%+v", got)
	}
}

func TestDefaultContingencyFormsCoverFrozenPaperSet(t *testing.T) {
	forms := DefaultContingencyForms()
	if len(forms) != 5 {
		t.Fatalf("forms=%d", len(forms))
	}
	want := map[string]bool{"Incident Record": false, "Decision Record": false, "Action Record": false, "Communication Record": false, "Evidence Record": false}
	for _, form := range forms {
		if _, ok := want[form.Title]; ok {
			want[form.Title] = true
		}
	}
	for title, found := range want {
		if !found {
			t.Fatalf("missing %s", title)
		}
	}
}
