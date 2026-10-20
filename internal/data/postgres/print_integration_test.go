package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

func TestPaperTranscriptionTimelineAndRebuildPreserveBothTimes(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	base := time.Date(2026, 10, 20, 1, 0, 0, 0, time.UTC)
	incidentID := "INC-PAPER"
	kernel, signer := slice3Kernel(t, db)
	service := establishCommunicationIncident(t, db, kernel, signer, incidentID, base)
	principal := authz.Principal{
		Authenticated: true,
		HumanID:       "transcriber",
		Role:          authz.RoleMinor,
		Incidents:     map[string]bool{incidentID: true},
		Compartments:  map[string]map[string]bool{incidentID: {"GENERAL": true}},
	}
	scope := domain.Scope{IncidentID: incidentID, Mode: "live", Compartment: "GENERAL"}
	paperAt := base.Add(15 * time.Minute)
	transcribedAt := base.Add(6 * time.Hour)
	value := domain.PaperTranscription{Statement: "Network isolation recorded on paper", PaperOccurredAt: paperAt, SourceReference: "paper-log-page-3", TimeConfidence: "untrusted"}
	if _, err := service.RecordPaperTranscription(ctx, principal, scope, "PAPER-1", value, transcribedAt); err != nil {
		t.Fatal(err)
	}

	assertPaperTimeline := func() {
		picture, found, err := service.CommonOperatingPicture(ctx, principal, incidentID, "GENERAL")
		if err != nil || !found {
			t.Fatalf("operating picture found=%v err=%v", found, err)
		}
		for _, item := range picture.Timeline {
			if item.RecordID != "PAPER-1" {
				continue
			}
			if item.RecordType != domain.TypePaperTranscription || item.OccurredAt == nil || !item.OccurredAt.Equal(paperAt) || !item.RecordedAt.Equal(transcribedAt) || item.TimeStatus != "untrusted" {
				t.Fatalf("paper timeline=%+v", item)
			}
			return
		}
		t.Fatal("paper transcription missing from timeline")
	}
	assertPaperTimeline()

	if _, err := kernel.Rebuild(ctx, IncidentProjection{}, PaperProjection{}, DeclarationAccessProjection{}); err != nil {
		t.Fatal(err)
	}
	assertPaperTimeline()
}

func TestPrintSourceUsesCurrentLedgerPosition(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	base := time.Date(2026, 10, 20, 2, 0, 0, 0, time.UTC)
	kernel, signer := slice3Kernel(t, db)
	_ = establishCommunicationIncident(t, db, kernel, signer, "INC-PRINT-SOURCE", base)
	store, err := NewPrintStore(db.runtime)
	if err != nil {
		t.Fatal(err)
	}
	mark, err := store.LedgerSource(ctx, base.Add(time.Hour), "cfg-11")
	if err != nil {
		t.Fatal(err)
	}
	var seq int64
	var hash string
	if err := db.runtime.QueryRow(ctx, `SELECT seq,hash FROM crab.ledger_entries ORDER BY seq DESC LIMIT 1`).Scan(&seq, &hash); err != nil {
		t.Fatal(err)
	}
	if mark.LedgerSequence != uint64(seq) || mark.LedgerHash != hash || mark.ConfigurationVersion != "cfg-11" || !mark.GeneratedAt.Equal(base.Add(time.Hour)) {
		t.Fatalf("source mark=%+v database=%d/%s", mark, seq, hash)
	}
}
