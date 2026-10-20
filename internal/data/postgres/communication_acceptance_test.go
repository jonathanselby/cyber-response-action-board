package postgres

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

func TestGovernedCommunicationCannotUseImmediateRecordSemantic(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	tx, err := db.runtime.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	draft := domain.CommunicationDraft{Kind: domain.CommunicationHoldingStatement, Subject: "Holding statement", Body: "Approved wording", Audience: []string{"media"}}
	digest, err := domain.CommunicationDraftDigest(draft)
	if err != nil {
		t.Fatal(err)
	}
	issue, err := domain.CommunicationIssueFromDraft("DRAFT-1", digest, draft)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(issue)
	if err != nil {
		t.Fatal(err)
	}
	record := ledger.Record{
		SchemaVersion: 1,
		Timestamp:     time.Date(2026, 10, 20, 11, 0, 0, 0, time.UTC),
		TimeStatus:    "trusted",
		IncidentID:    "INC-COMM",
		IncidentMode:  "live",
		WriteSemantic: "record",
		RecordType:    domain.TypeCommunicationIssue,
		RecordID:      "COMM-FORGED",
		Status:        "recorded",
		Maker:         "alice",
		Compartment:   "GENERAL",
		Data:          payload,
	}
	if err := (CommunicationProjection{}).Apply(ctx, tx, ledger.Entry{Record: record}); err == nil {
		t.Fatal("governed communication accepted through immediate-record semantic")
	}
}

func TestIncomingEnquiryAssignmentPersistsAndRebuilds(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	base := time.Date(2026, 10, 20, 11, 30, 0, 0, time.UTC)
	incidentID := "INC-ENQUIRY"
	commRuntime, kernel, signer := newCommunicationRuntime(t, db)
	_ = establishCommunicationIncident(t, db, kernel, signer, incidentID, base)
	service := app.CommunicationService{Backend: commRuntime}
	principal := authz.Principal{
		Authenticated: true,
		HumanID:       "responder",
		Role:          authz.RoleMinor,
		Incidents:     map[string]bool{incidentID: true},
		Compartments:  map[string]map[string]bool{incidentID: {"GENERAL": true}},
		Outputs:       map[authz.Action]bool{},
	}
	scope := domain.Scope{IncidentID: incidentID, Mode: "live", Compartment: "GENERAL"}
	enquiry := domain.IncomingEnquiry{
		Source:            "media",
		Contact:           "newsdesk@example.invalid",
		Subject:           "Request for comment",
		Message:           "Please confirm current service status.",
		ReceivedAt:        base.Add(5 * time.Minute),
		AssignedToHumanID: "comms-lead",
		RelatedRecordIDs:  []string{"DECLARE-COMM"},
	}
	if _, err := service.RecordIncomingEnquiry(ctx, principal, scope, "ENQ-1", enquiry, base.Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	stored, err := commRuntime.Store.IncomingEnquiries(ctx, incidentID, "GENERAL")
	if err != nil || len(stored) != 1 || stored[0].Enquiry.AssignedToHumanID != "comms-lead" || stored[0].Enquiry.Subject != enquiry.Subject {
		t.Fatalf("stored enquiries=%+v err=%v", stored, err)
	}
	if _, err := kernel.Rebuild(ctx, IncidentProjection{}, DeclarationAccessProjection{}, CommunicationProjection{}); err != nil {
		t.Fatal(err)
	}
	rebuilt, err := commRuntime.Store.IncomingEnquiries(ctx, incidentID, "GENERAL")
	if err != nil || len(rebuilt) != 1 || rebuilt[0].Enquiry.AssignedToHumanID != "comms-lead" || rebuilt[0].Enquiry.Message != enquiry.Message {
		t.Fatalf("rebuilt enquiries=%+v err=%v", rebuilt, err)
	}
}

var _ pgx.Tx
