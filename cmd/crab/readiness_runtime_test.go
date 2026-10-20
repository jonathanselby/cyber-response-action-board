package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/config"
	"github.com/jonathanselby/cyber-response-action-board/internal/data/postgres"
)

func TestBackupRestoreReadinessUsesRealArtefactEvidence(t *testing.T) {
	dir := t.TempDir()
	dump := []byte("backup-bytes")
	dumpPath := filepath.Join(dir, "crab.dump")
	if err := os.WriteFile(dumpPath, dump, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(dump)
	meta := postgres.BackupMetadata{Format: "postgresql-custom", SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(dump)), CreatedAt: time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)}
	b, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dumpPath+".json", b, 0o600); err != nil {
		t.Fatal(err)
	}
	restoredAt := time.Date(2026, 10, 20, 11, 0, 0, 0, time.UTC)
	if err := writeRestoreEvidence(dir, restoreEvidence{Kind: "restore-rehearsal", RestoredAt: restoredAt, BackupSHA256: meta.SHA256, LedgerEntries: 42, LedgerVerified: true}); err != nil {
		t.Fatal(err)
	}
	check := backupRestoreReadiness(config.Config{ExportDir: dir})
	if check.Status != "pass" {
		t.Fatalf("check = %+v", check)
	}
}

func TestBackupRestoreReadinessFailsWithoutEvidence(t *testing.T) {
	check := backupRestoreReadiness(config.Config{ExportDir: t.TempDir()})
	if check.Status != "fail" {
		t.Fatalf("check = %+v", check)
	}
}
