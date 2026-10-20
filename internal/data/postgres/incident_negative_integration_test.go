package postgres

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

func TestControlledActionCannotUseImmediateRecordSemantic(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	kernel, signer := slice3Kernel(t, db)
	payload, err := json.Marshal(domain.Action{Title: "Forged immediate shutdown"})
	if err != nil {
		t.Fatal(err)
	}
	forged := ledger.Record{
		SchemaVersion:  1,
		Timestamp:      time.Date(2026, 10, 20, 3, 45, 0, 0, time.UTC),
		TimeStatus:     "trusted",
		IncidentID:     "INC-FORGE",
		IncidentMode:   "live",
		WriteSemantic:  "record",
		RecordType:     domain.TypeAction,
		RecordID:       "ACT-FORGE",
		Status:         "recorded",
		Maker:          "alice",
		Compartment:    "GENERAL",
		Data:           payload,
		MakerSignature: ledger.Signature{MissingReason: "maker authenticated by CRAB session; no per-record signing key"},
	}
	if _, err := kernel.Append(ctx, forged, signer, IncidentProjection{}); err == nil {
		t.Fatal("controlled action accepted through immediate Record semantic")
	}
	var count int
	if err := db.runtime.QueryRow(ctx, `SELECT count(*) FROM crab.ledger_entries`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("rejected controlled Record still committed ledger entries=%d", count)
	}
}

func TestDecisionCannotChangeProposalIncidentMode(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	kernel, signer := slice3Kernel(t, db)
	scope := domain.Scope{IncidentID: "INC-MODE", Mode: "exercise", Compartment: "GENERAL"}
	proposal, err := domain.ProposeAction(scope, "ACT-MODE", "alice", time.Date(2026, 10, 20, 3, 50, 0, 0, time.UTC), domain.Action{
		Title:        "Exercise containment",
		OwnerHumanID: "alice",
		Status:       domain.ActionStatusNotStarted,
		Priority:     "P1",
		SourceRef:    "EXERCISE-PLAN",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kernel.Append(ctx, proposal, signer, IncidentProjection{}); err != nil {
		t.Fatal(err)
	}
	version, _ := domain.ProposalVersion(proposal)
	decided, err := domain.Decide(proposal, version, "bob", "confirmed", proposal.Timestamp.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	decided.IncidentMode = "live"
	if _, err := kernel.Append(ctx, decided, signer, IncidentProjection{}); err == nil {
		t.Fatal("decision changed exercise proposal into live incident state")
	}
	var actions int
	if err := db.runtime.QueryRow(ctx, `SELECT count(*) FROM crab.incident_actions WHERE incident_id='INC-MODE'`).Scan(&actions); err != nil {
		t.Fatal(err)
	}
	if actions != 0 {
		t.Fatalf("mode-swapped decision became effective actions=%d", actions)
	}
	var ledgerEntries int
	if err := db.runtime.QueryRow(ctx, `SELECT count(*) FROM crab.ledger_entries`).Scan(&ledgerEntries); err != nil {
		t.Fatal(err)
	}
	if ledgerEntries != 1 {
		t.Fatalf("failed mode-swapped decision committed ledger entry count=%d", ledgerEntries)
	}
}
