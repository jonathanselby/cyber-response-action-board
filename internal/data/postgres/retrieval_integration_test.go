package postgres

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

func retrievalPrincipal(human string, role authz.Role, incident string, compartments ...string) authz.Principal {
	p := authz.Principal{Authenticated: true, HumanID: human, Role: role, Incidents: map[string]bool{}, Compartments: map[string]map[string]bool{}, Outputs: map[authz.Action]bool{}}
	if incident != "" {
		p.Incidents[incident] = true
		p.Compartments[incident] = map[string]bool{"GENERAL": true}
		for _, compartment := range compartments {
			p.Compartments[incident][compartment] = true
		}
	}
	return p
}

func newRetrievalService(t *testing.T, db *testDB) (*app.RetrievalService, *CorpusRuntime, *Kernel) {
	t.Helper()
	kernel, signer := slice3Kernel(t, db)
	blobs, err := NewBlobStore(db.runtime, domain.MaxCorpusSourceBytes)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewCorpusStore(db.runtime)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewCorpusRuntime(kernel, signer, blobs, store)
	if err != nil {
		t.Fatal(err)
	}
	return &app.RetrievalService{Backend: runtime}, runtime, kernel
}

func confirmCorpusSource(t *testing.T, service *app.RetrievalService, proposer, confirmer authz.Principal, scope domain.CorpusScope, sourceID, title, filename, body string, now time.Time) domain.RetrievalCitation {
	t.Helper()
	proposal, err := service.ProposeSource(context.Background(), proposer, scope, sourceID, app.CorpusSourceInput{Title: title, Filename: filename, DeclaredMediaType: "text/markdown"}, bytes.NewBufferString(body), now)
	if err != nil {
		t.Fatal(err)
	}
	version, err := domain.ProposalVersion(proposal)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.DecideSource(context.Background(), confirmer, sourceID, version, "confirmed", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	return domain.RetrievalCitation{}
}

func TestRetrievalFiltersAuthorisedCorpusBeforeRankingAndRefusesUnsupported(t *testing.T) {
	db := setupTestDB(t)
	service, _, _ := newRetrievalService(t, db)
	ctx := context.Background()
	base := time.Date(2026, 10, 20, 7, 0, 0, 0, time.UTC)
	aliceSystem := retrievalPrincipal("alice", authz.RoleMajor, "")
	bobSystem := retrievalPrincipal("bob", authz.RoleMajor, "")

	confirmCorpusSource(t, service, aliceSystem, bobSystem, domain.CorpusScope{Mode: "system", Compartment: "GENERAL"}, "RUNBOOK-GEN", "Containment runbook", "containment.md", "# Containment\nIsolate affected ransomware hosts from the network immediately.\n", base)
	confirmCorpusSource(t, service, aliceSystem, bobSystem, domain.CorpusScope{Mode: "system", Compartment: "LEGAL"}, "RUNBOOK-LEGAL", "Legal strategy", "legal.md", "# Privileged\nIsolate ransomware hosts ransomware hosts ransomware hosts and call privileged counsel.\n", base.Add(time.Minute))

	incidentB := "INC-OTHER"
	makerB := retrievalPrincipal("maker-b", authz.RoleMajor, incidentB)
	confirmerB := retrievalPrincipal("confirmer-b", authz.RoleMajor, incidentB)
	confirmCorpusSource(t, service, makerB, confirmerB, domain.CorpusScope{IncidentID: incidentB, Mode: "live", Compartment: "GENERAL"}, "RUNBOOK-OTHER", "Other incident", "other.md", "Isolate ransomware hosts ransomware hosts ransomware hosts ransomware hosts.\n", base.Add(2*time.Minute))

	general := retrievalPrincipal("responder", authz.RoleMinor, "INC-A")
	result, err := service.Retrieve(ctx, general, "INC-A", "GENERAL", "isolate ransomware hosts", 5)
	if err != nil {
		t.Fatal(err)
	}
	if result.Refused || len(result.Citations) != 1 || result.Citations[0].SourceID != "RUNBOOK-GEN" {
		t.Fatalf("pre-rank scope filter failed result=%+v", result)
	}
	if result.Citations[0].CitationID == "" || result.Citations[0].Locator != "lines:1-2" {
		t.Fatalf("unstable citation metadata=%+v", result.Citations[0])
	}

	unsupported, err := service.Retrieve(ctx, general, "INC-A", "GENERAL", "satellite telemetry orbital decay", 5)
	if err != nil || !unsupported.Refused || len(unsupported.Citations) != 0 {
		t.Fatalf("unsupported result=%+v err=%v", unsupported, err)
	}

	legal := retrievalPrincipal("lawyer", authz.RoleMinor, "INC-A", "LEGAL")
	legalResult, err := service.Retrieve(ctx, legal, "INC-A", "LEGAL", "privileged counsel", 5)
	if err != nil || legalResult.Refused || len(legalResult.Citations) == 0 || legalResult.Citations[0].SourceID != "RUNBOOK-LEGAL" {
		t.Fatalf("authorised LEGAL retrieval=%+v err=%v", legalResult, err)
	}
	if _, err := service.Retrieve(ctx, general, "INC-A", "LEGAL", "privileged counsel", 5); !errors.Is(err, app.ErrAuthorisationDenied) {
		t.Fatalf("GENERAL user searched LEGAL corpus err=%v", err)
	}
}

func TestRetrievalRebuildKeepsStableCitationAndImmutableSource(t *testing.T) {
	db := setupTestDB(t)
	service, runtime, kernel := newRetrievalService(t, db)
	ctx := context.Background()
	base := time.Date(2026, 10, 20, 7, 30, 0, 0, time.UTC)
	maker := retrievalPrincipal("alice", authz.RoleMajor, "")
	confirmer := retrievalPrincipal("bob", authz.RoleMajor, "")
	confirmCorpusSource(t, service, maker, confirmer, domain.CorpusScope{Mode: "system", Compartment: "GENERAL"}, "RUNBOOK-STABLE", "Stable runbook", "stable.md", "# Backups\nRestore systems from known-good offline backups.\n", base)
	reader := retrievalPrincipal("reader", authz.RoleObserver, "INC-STABLE")
	before, err := service.Retrieve(ctx, reader, "INC-STABLE", "GENERAL", "offline backups", 5)
	if err != nil || len(before.Citations) != 1 {
		t.Fatalf("before=%+v err=%v", before, err)
	}
	var blobID string
	if err := db.runtime.QueryRow(ctx, `SELECT blob_id FROM crab.corpus_sources WHERE source_id='RUNBOOK-STABLE'`).Scan(&blobID); err != nil {
		t.Fatal(err)
	}
	if _, err := kernel.Rebuild(ctx, CorpusProjection{}); err != nil {
		t.Fatal(err)
	}
	after, err := service.Retrieve(ctx, reader, "INC-STABLE", "GENERAL", "offline backups", 5)
	if err != nil || len(after.Citations) != 1 || before.Citations[0].CitationID != after.Citations[0].CitationID || before.Citations[0].Passage != after.Citations[0].Passage {
		t.Fatalf("citation changed across rebuild before=%+v after=%+v err=%v", before, after, err)
	}
	if _, err := runtime.Blobs.Verify(ctx, blobID); err != nil {
		t.Fatalf("rebuild touched immutable corpus blob: %v", err)
	}
	if _, err := db.runtime.Exec(ctx, `UPDATE crab.blobs SET size_bytes=size_bytes WHERE blob_id=$1`, blobID); err == nil {
		t.Fatal("runtime unexpectedly has UPDATE on immutable corpus blob")
	}
}

func TestConfirmedDeclarationEstablishesInitialSponsorAndMembership(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	kernel, signer := slice3Kernel(t, db)
	store, err := NewIncidentStore(db.runtime)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewIncidentRuntime(kernel, signer, store)
	if err != nil {
		t.Fatal(err)
	}
	service := app.IncidentService{Backend: runtime}
	base := time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)
	scope := domain.Scope{IncidentID: "INC-DECL-ACCESS", Mode: "live", Compartment: "GENERAL"}
	maker := retrievalPrincipal("declarer", authz.RoleMajor, "")
	confirmer := retrievalPrincipal("confirmer", authz.RoleMajor, "")
	proposal, err := service.ProposeIncidentDeclaration(ctx, maker, scope, "DECLARE-ACCESS", domain.IncidentDeclaration{Title: "Incident", Summary: "Confirmed incident", Severity: "High"}, base)
	if err != nil {
		t.Fatal(err)
	}
	version, _ := domain.ProposalVersion(proposal)
	if _, err := service.Decide(ctx, confirmer, scope.IncidentID, domain.TypeIncidentDeclaration, proposal.RecordID, version, "confirmed", base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	accessStore, err := NewAccessStore(db.runtime)
	if err != nil {
		t.Fatal(err)
	}
	principal, err := accessStore.PrincipalAt(ctx, "declarer", true, base.Add(2*time.Minute))
	if err != nil || !principal.Incidents[scope.IncidentID] || !principal.Compartments[scope.IncidentID]["GENERAL"] {
		t.Fatalf("declaration maker access=%+v err=%v", principal, err)
	}
	var sponsor bool
	if err := db.runtime.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM crab.incident_sponsors WHERE human_id='declarer' AND incident_id=$1)`, scope.IncidentID).Scan(&sponsor); err != nil || !sponsor {
		t.Fatalf("declaration maker sponsor=%v err=%v", sponsor, err)
	}
	if _, err := kernel.Rebuild(ctx, AccessProjection{}, IncidentProjection{}, DeclarationAccessProjection{}); err != nil {
		t.Fatal(err)
	}
	principal, err = accessStore.PrincipalAt(ctx, "declarer", true, base.Add(2*time.Minute))
	if err != nil || !principal.Incidents[scope.IncidentID] || !principal.Compartments[scope.IncidentID]["GENERAL"] {
		t.Fatalf("rebuild lost declaration access=%+v err=%v", principal, err)
	}
}
