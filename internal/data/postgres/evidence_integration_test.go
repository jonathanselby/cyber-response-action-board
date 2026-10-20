package postgres

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

type failingEvidenceReader struct{ sent bool }

func (r *failingEvidenceReader) Read(p []byte) (int, error) {
	if !r.sent {
		r.sent = true
		return copy(p, []byte("partial hostile bytes")), nil
	}
	return 0, errors.New("injected upload disconnect")
}

func evidencePrincipal(human, incident, compartment string, download bool) authz.Principal {
	p := authz.Principal{Authenticated: true, HumanID: human, Role: authz.RoleMinor, Incidents: map[string]bool{incident: true}, Compartments: map[string]map[string]bool{incident: {"GENERAL": true}}, Outputs: map[authz.Action]bool{}}
	if compartment != "GENERAL" {
		p.Compartments[incident][compartment] = true
	}
	if download {
		p.Outputs[authz.ActionDownload] = true
	}
	return p
}

func newEvidenceService(t *testing.T, db *testDB, maxBytes int64) (*app.EvidenceService, *EvidenceRuntime, *Kernel) {
	t.Helper()
	kernel, signer := slice3Kernel(t, db)
	blobs, err := NewBlobStore(db.runtime, maxBytes)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewEvidenceStore(db.runtime)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewEvidenceRuntime(kernel, signer, blobs, store)
	if err != nil {
		t.Fatal(err)
	}
	service := &app.EvidenceService{Backend: runtime}
	return service, runtime, kernel
}

func captureFixture(t *testing.T, service *app.EvidenceService, principal authz.Principal, scope domain.Scope, evidenceID, filename, declared string, raw []byte, now time.Time) {
	t.Helper()
	_, err := service.Capture(context.Background(), principal, scope, evidenceID, app.EvidenceCapture{OriginalFilename: filename, DeclaredMediaType: declared, Description: "Incident evidence", Collector: principal.HumanID, Source: "phone capture", SourceLocation: "field", CurrentHandler: principal.HumanID}, bytes.NewReader(raw), now)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEvidenceExactBytesIntegrityScopeAndSeparateDownloadPermission(t *testing.T) {
	db := setupTestDB(t)
	service, runtime, _ := newEvidenceService(t, db, 1<<20)
	ctx := context.Background()
	scopeA := domain.Scope{IncidentID: "INC-EVID-A", Mode: "live", Compartment: "GENERAL"}
	principalA := evidencePrincipal("alice", scopeA.IncidentID, "GENERAL", true)
	raw := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 80)...)
	captureFixture(t, service, principalA, scopeA, "EVID-A", "photo.png", "image/png", raw, time.Date(2026, 10, 20, 6, 0, 0, 0, time.UTC))

	view, err := service.Detail(ctx, principalA, "EVID-A")
	if err != nil {
		t.Fatal(err)
	}
	if view.Evidence.SizeBytes != int64(len(raw)) || view.Evidence.SHA256 == "" || view.Evidence.DetectedMediaType != "image/png" {
		t.Fatalf("evidence metadata=%+v", view.Evidence)
	}
	var out bytes.Buffer
	_, mode, err := service.Attachment(ctx, principalA, "EVID-A", &out)
	if err != nil || !bytes.Equal(out.Bytes(), raw) || mode != domain.EvidencePresentationInline {
		t.Fatalf("attachment mode=%q bytes=%d err=%v", mode, out.Len(), err)
	}

	noDownload := evidencePrincipal("alice", scopeA.IncidentID, "GENERAL", false)
	if _, _, err := service.Attachment(ctx, noDownload, "EVID-A", io.Discard); !errors.Is(err, app.ErrAuthorisationDenied) {
		t.Fatalf("read permission unexpectedly implied download: %v", err)
	}

	scopeB := domain.Scope{IncidentID: "INC-EVID-B", Mode: "live", Compartment: "GENERAL"}
	principalB := evidencePrincipal("bob", scopeB.IncidentID, "GENERAL", true)
	captureFixture(t, service, principalB, scopeB, "EVID-B", "same.png", "image/png", raw, time.Date(2026, 10, 20, 6, 1, 0, 0, time.UTC))
	viewB, _ := service.Detail(ctx, principalB, "EVID-B")
	if view.Evidence.SHA256 != viewB.Evidence.SHA256 || view.Evidence.BlobID == viewB.Evidence.BlobID {
		t.Fatalf("cross-incident dedupe occurred A=%+v B=%+v", view.Evidence, viewB.Evidence)
	}
	if _, _, err := service.Attachment(ctx, principalA, "EVID-B", io.Discard); !errors.Is(err, app.ErrAuthorisationDenied) {
		t.Fatalf("guessed cross-scope evidence id disclosed attachment: %v", err)
	}

	if _, err := runtime.VerifyEvidenceBlob(ctx, view.Evidence.BlobID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.runtime.Exec(ctx, `UPDATE crab.blobs SET size_bytes=size_bytes WHERE blob_id=$1`, view.Evidence.BlobID); err == nil {
		t.Fatal("runtime unexpectedly has UPDATE on immutable blob metadata")
	}
	if _, err := db.runtime.Exec(ctx, `DELETE FROM crab.blob_chunks WHERE blob_id=$1`, view.Evidence.BlobID); err == nil {
		t.Fatal("runtime unexpectedly has DELETE on immutable blob chunks")
	}
}

func TestEvidencePartialOversizeAndCorruptionFailClosed(t *testing.T) {
	db := setupTestDB(t)
	service, runtime, _ := newEvidenceService(t, db, 32)
	ctx := context.Background()
	scope := domain.Scope{IncidentID: "INC-EVID-FAIL", Mode: "live", Compartment: "GENERAL"}
	principal := evidencePrincipal("alice", scope.IncidentID, "GENERAL", true)
	input := app.EvidenceCapture{OriginalFilename: "capture.bin", Collector: "alice", Source: "phone", CurrentHandler: "alice"}

	if _, err := service.Capture(ctx, principal, scope, "EVID-PARTIAL", input, &failingEvidenceReader{}, time.Now().UTC()); err == nil {
		t.Fatal("partial upload unexpectedly succeeded")
	}
	if _, err := service.Capture(ctx, principal, scope, "EVID-LARGE", input, bytes.NewReader(bytes.Repeat([]byte{'x'}, 33)), time.Now().UTC()); !errors.Is(err, ErrBlobTooLarge) {
		t.Fatalf("oversize err=%v", err)
	}
	var blobs, chunks, evidence, ledgerRows int
	if err := db.runtime.QueryRow(ctx, `SELECT count(*) FROM crab.blobs`).Scan(&blobs); err != nil {
		t.Fatal(err)
	}
	if err := db.runtime.QueryRow(ctx, `SELECT count(*) FROM crab.blob_chunks`).Scan(&chunks); err != nil {
		t.Fatal(err)
	}
	if err := db.runtime.QueryRow(ctx, `SELECT count(*) FROM crab.evidence_register`).Scan(&evidence); err != nil {
		t.Fatal(err)
	}
	if err := db.runtime.QueryRow(ctx, `SELECT count(*) FROM crab.ledger_entries`).Scan(&ledgerRows); err != nil {
		t.Fatal(err)
	}
	if blobs != 0 || chunks != 0 || evidence != 0 || ledgerRows != 0 {
		t.Fatalf("failed upload left state blobs=%d chunks=%d evidence=%d ledger=%d", blobs, chunks, evidence, ledgerRows)
	}

	service, runtime, _ = newEvidenceService(t, db, 1<<20)
	html := []byte("<!doctype html><html><script>alert(1)</script></html>")
	captureFixture(t, service, principal, scope, "EVID-HTML", "photo.png", "image/png", html, time.Now().UTC())
	view, err := service.Detail(ctx, principal, "EVID-HTML")
	if err != nil {
		t.Fatal(err)
	}
	if domain.EvidencePresentationMode(view.Evidence.DeclaredMediaType, view.Evidence.DetectedMediaType) != domain.EvidencePresentationDownload {
		t.Fatalf("mislabelled active media allowed inline: %+v", view.Evidence)
	}
	if _, err := db.owner.Exec(ctx, `UPDATE crab.blob_chunks SET bytes='corrupt'::bytea WHERE blob_id=$1 AND chunk_no=0`, view.Evidence.BlobID); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, _, err := service.Attachment(ctx, principal, "EVID-HTML", &out); !errors.Is(err, ErrBlobIntegrity) || out.Len() != 0 {
		t.Fatalf("corrupt blob result err=%v bytes=%d", err, out.Len())
	}
	if _, err := runtime.VerifyEvidenceBlob(ctx, view.Evidence.BlobID); !errors.Is(err, ErrBlobIntegrity) {
		t.Fatalf("corrupt verify err=%v", err)
	}
}

func TestEvidenceHandlingHistoryAndRegisterRebuildWithoutTouchingBlobs(t *testing.T) {
	db := setupTestDB(t)
	service, runtime, kernel := newEvidenceService(t, db, 1<<20)
	ctx := context.Background()
	scope := domain.Scope{IncidentID: "INC-EVID-HISTORY", Mode: "live", Compartment: "LEGAL"}
	principal := evidencePrincipal("alice", scope.IncidentID, "LEGAL", true)
	base := time.Date(2026, 10, 20, 6, 20, 0, 0, time.UTC)
	captureFixture(t, service, principal, scope, "EVID-HISTORY", "capture.bin", "application/octet-stream", []byte("hostile bytes remain opaque"), base)

	if _, err := service.RecordHandling(ctx, principal, "EVID-HISTORY", "HANDLE-1", domain.EvidenceHandling{Action: "transferred", Handler: "bob", Location: "sealed cabinet", Note: "handover witnessed"}, base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	view, err := service.Detail(ctx, principal, "EVID-HISTORY")
	if err != nil || view.Evidence.CurrentHandler != "bob" {
		t.Fatalf("current handler=%q err=%v", view.Evidence.CurrentHandler, err)
	}
	history, err := service.History(ctx, principal, "EVID-HISTORY")
	if err != nil || len(history) != 1 || history[0].Handling.Action != "transferred" {
		t.Fatalf("history=%+v err=%v", history, err)
	}
	blobID := view.Evidence.BlobID
	if _, err := kernel.Rebuild(ctx, EvidenceProjection{}); err != nil {
		t.Fatal(err)
	}
	rebuilt, err := service.Detail(ctx, principal, "EVID-HISTORY")
	if err != nil || rebuilt.Evidence.CurrentHandler != "bob" || rebuilt.Evidence.BlobID != blobID {
		t.Fatalf("rebuilt evidence=%+v err=%v", rebuilt, err)
	}
	if _, err := runtime.VerifyEvidenceBlob(ctx, blobID); err != nil {
		t.Fatalf("projection rebuild touched durable blob: %v", err)
	}

	general := evidencePrincipal("carol", scope.IncidentID, "GENERAL", false)
	register, err := service.Register(ctx, general, scope.IncidentID, "GENERAL")
	if err != nil || len(register) != 0 {
		t.Fatalf("GENERAL register leaked LEGAL evidence=%+v err=%v", register, err)
	}
	register, err = service.Register(ctx, principal, scope.IncidentID, "LEGAL")
	if err != nil || len(register) != 1 || register[0].EvidenceID != "EVID-HISTORY" {
		t.Fatalf("LEGAL register=%+v err=%v", register, err)
	}
}
