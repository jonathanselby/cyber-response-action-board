package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/config"
	"github.com/jonathanselby/cyber-response-action-board/internal/data/postgres"
)

type restoreEvidence struct {
	Kind           string    `json:"kind"`
	RestoredAt     time.Time `json:"restored_at"`
	BackupSHA256   string    `json:"backup_sha256"`
	LedgerEntries  int       `json:"ledger_entries"`
	LedgerVerified bool      `json:"ledger_verified"`
}

func setReadinessCheck(r *app.Readiness, check app.Check) {
	for i := range r.Checks {
		if r.Checks[i].Name == check.Name {
			r.Checks[i] = check
			return
		}
	}
	r.Checks = append(r.Checks, check)
}

func enrichReadiness(cfg config.Config, r *app.Readiness) {
	setReadinessCheck(r, keyCustodyCheck(cfg))
	setReadinessCheck(r, backupRestoreReadiness(cfg))
	setReadinessCheck(r, exportPathCheck(cfg))
	setReadinessCheck(r, packFreshnessCheck(cfg))
	setReadinessCheck(r, storageCheck(cfg))
	setReadinessCheck(r, app.Check{Name: "time_status", Status: "warn", Message: "host clock is in use; incident records preserve trusted/drifting/untrusted time status explicitly"})
	setReadinessCheck(r, notificationDeploymentReadiness(cfg))
	setReadinessCheck(r, modelDeploymentReadiness(cfg))
	if strings.HasPrefix(strings.ToLower(cfg.PublicBaseURL), "https://") {
		setReadinessCheck(r, app.Check{Name: "certificate_hostname", Status: "pass", Message: "HTTPS public base URL configured"})
	} else {
		setReadinessCheck(r, app.Check{Name: "certificate_hostname", Status: "warn", Message: "no HTTPS public base URL configured; current listener remains loopback-only"})
	}
}

func keyCustodyCheck(cfg config.Config) app.Check {
	files := []struct {
		name string
		path string
	}{
		{"ledger keyring", cfg.LedgerKeyringFile},
		{"session key", cfg.SessionKeyFile},
		{"server signing key", cfg.ServerSigningKeyFile},
		{"credential wrap key", cfg.CredentialWrapKeyFile},
	}
	for _, item := range files {
		if item.path == "" {
			return app.Check{Name: "key_custody", Status: "not-configured", Message: item.name + " is not configured"}
		}
		info, err := os.Stat(item.path)
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			return app.Check{Name: "key_custody", Status: "fail", Message: fmt.Sprintf("%s is not a readable non-empty regular file", item.name)}
		}
	}
	return app.Check{Name: "key_custody", Status: "pass", Message: "configured local key files are present and non-empty"}
}

func backupRestoreReadiness(cfg config.Config) app.Check {
	if cfg.ExportDir == "" {
		return app.Check{Name: "backup_restore", Status: "not-configured", Message: "CRAB_EXPORT_DIR is required for local backup and restore evidence"}
	}
	backup, backupAt, backupErr := latestValidBackup(cfg.ExportDir)
	restore, restoreErr := latestRestoreEvidence(cfg.ExportDir)
	if backupErr != nil {
		return app.Check{Name: "backup_restore", Status: "fail", Message: "backup evidence: " + backupErr.Error()}
	}
	if restoreErr != nil {
		return app.Check{Name: "backup_restore", Status: "fail", Message: "restore rehearsal evidence: " + restoreErr.Error()}
	}
	return app.Check{Name: "backup_restore", Status: "pass", Message: fmt.Sprintf("latest verified backup %s sha256=%s; latest restore/rebuild rehearsal %s", backupAt.UTC().Format(time.RFC3339), backup.SHA256, restore.RestoredAt.UTC().Format(time.RFC3339))}
}

func latestValidBackup(dir string) (postgres.BackupMetadata, time.Time, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return postgres.BackupMetadata{}, time.Time{}, err
	}
	var best postgres.BackupMetadata
	var bestAt time.Time
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || strings.HasPrefix(entry.Name(), "restore-rehearsal-") {
			continue
		}
		metaPath := filepath.Join(dir, entry.Name())
		b, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var meta postgres.BackupMetadata
		if json.Unmarshal(b, &meta) != nil || meta.Format != "postgresql-custom" || meta.CreatedAt.IsZero() {
			continue
		}
		dumpPath := strings.TrimSuffix(metaPath, ".json")
		if postgres.VerifyBackup(dumpPath) != nil {
			continue
		}
		if bestAt.IsZero() || meta.CreatedAt.After(bestAt) {
			best, bestAt = meta, meta.CreatedAt
		}
	}
	if bestAt.IsZero() {
		return postgres.BackupMetadata{}, time.Time{}, fmt.Errorf("no verified PostgreSQL backup metadata found in %s", dir)
	}
	return best, bestAt, nil
}

func latestRestoreEvidence(dir string) (restoreEvidence, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return restoreEvidence{}, err
	}
	var best restoreEvidence
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "restore-rehearsal-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var evidence restoreEvidence
		if json.Unmarshal(b, &evidence) != nil || evidence.Kind != "restore-rehearsal" || !evidence.LedgerVerified || evidence.RestoredAt.IsZero() {
			continue
		}
		if best.RestoredAt.IsZero() || evidence.RestoredAt.After(best.RestoredAt) {
			best = evidence
		}
	}
	if best.RestoredAt.IsZero() {
		return restoreEvidence{}, fmt.Errorf("no successful restore/rebuild rehearsal evidence found in %s", dir)
	}
	return best, nil
}

func writeRestoreEvidence(dir string, evidence restoreEvidence) error {
	if dir == "" {
		return fmt.Errorf("CRAB_EXPORT_DIR is not configured")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	name := "restore-rehearsal-" + evidence.RestoredAt.UTC().Format("20060102T150405Z") + ".json"
	path := filepath.Join(dir, name)
	tmp, err := os.CreateTemp(dir, ".crab-restore-evidence-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	committed = true
	return nil
}

func exportPathCheck(cfg config.Config) app.Check {
	if cfg.ExportDir == "" {
		return app.Check{Name: "export_paths", Status: "not-configured", Message: "primary export directory is not configured"}
	}
	if err := writableDirectory(cfg.ExportDir); err != nil {
		return app.Check{Name: "export_paths", Status: "fail", Message: "primary export directory: " + err.Error()}
	}
	if cfg.SecondaryExportDir != "" {
		if err := writableDirectory(cfg.SecondaryExportDir); err != nil {
			return app.Check{Name: "export_paths", Status: "warn", Message: "primary export path available; secondary path unavailable: " + err.Error()}
		}
	}
	return app.Check{Name: "export_paths", Status: "pass", Message: "configured export paths are available"}
}

func writableDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}
	return nil
}

func packFreshnessCheck(cfg config.Config) app.Check {
	if cfg.ExportDir == "" {
		return app.Check{Name: "emergency_pack", Status: "not-configured", Message: "primary export directory is not configured"}
	}
	entries, err := os.ReadDir(cfg.ExportDir)
	if err != nil {
		return app.Check{Name: "emergency_pack", Status: "fail", Message: err.Error()}
	}
	var newest time.Time
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".pdf") {
			continue
		}
		info, err := entry.Info()
		if err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	if newest.IsZero() {
		return app.Check{Name: "emergency_pack", Status: "warn", Message: "no generated PDF found in the primary export directory"}
	}
	return app.Check{Name: "emergency_pack", Status: "pass", Message: "latest local PDF generated " + newest.UTC().Format(time.RFC3339)}
}

func storageCheck(cfg config.Config) app.Check {
	path := cfg.ExportDir
	if path == "" {
		path = "."
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return app.Check{Name: "storage_headroom", Status: "warn", Message: err.Error()}
	}
	free := uint64(stat.Bavail) * uint64(stat.Bsize)
	return app.Check{Name: "storage_headroom", Status: "pass", Message: fmt.Sprintf("%d MiB available on the checked filesystem", free>>20)}
}
