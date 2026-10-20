package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/config"
	"github.com/jonathanselby/cyber-response-action-board/internal/data/postgres"
)

type restoreResult struct {
	BackupVerified   bool      `json:"backup_verified"`
	LedgerValid      bool      `json:"ledger_valid"`
	LedgerEntries    int       `json:"ledger_entries"`
	RestoredAt       time.Time `json:"restored_at"`
	EvidenceRecorded bool      `json:"evidence_recorded"`
	EvidenceWarning  string    `json:"evidence_warning,omitempty"`
}

func runBackup(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	output := fs.String("output", "", "PostgreSQL custom backup output path")
	if err := fs.Parse(args); err != nil || *output == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: crab backup --output <path>")
		return exitUsage
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	keys, pool, code := openLedgerDatabase(stderr)
	if code != exitOK {
		return code
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	kernel, err := postgres.OpenKernel(ctx, pool, keys)
	if err != nil {
		pool.Close()
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	verify, err := kernel.Verify(ctx)
	pool.Close()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	if !verify.Valid {
		fmt.Fprintln(stderr, "ledger verification failed; backup refused")
		return exitRefused
	}
	meta, err := postgres.Backup(ctx, cfg.DatabaseURL, *output)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	return writeJSON(stdout, stderr, meta)
}

func runRestore(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	fs.SetOutput(stderr)
	input := fs.String("input", "", "PostgreSQL custom backup input path")
	if err := fs.Parse(args); err != nil || *input == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: crab restore --input <path>")
		return exitUsage
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	if cfg.DatabaseURL == "" || cfg.MaintenanceDatabaseURL == "" || cfg.LedgerKeyringFile == "" {
		fmt.Fprintln(stderr, "CRAB_DATABASE_URL, CRAB_MAINTENANCE_DATABASE_URL and CRAB_LEDGER_KEYRING_FILE are required")
		return exitRefused
	}
	keys, err := loadKeyring(cfg.LedgerKeyringFile)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	if err := postgres.VerifyBackup(*input); err != nil {
		fmt.Fprintln(stderr, err)
		return exitRefused
	}
	var backupMeta postgres.BackupMetadata
	if err := readJSONFile(*input+".json", &backupMeta); err != nil {
		fmt.Fprintln(stderr, err)
		return exitRefused
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	if err := postgres.Restore(ctx, cfg.MaintenanceDatabaseURL, *input); err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	pool, err := postgres.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	defer pool.Close()
	kernel, err := postgres.OpenKernel(ctx, pool, keys)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitRefused
	}
	if kernel.IntegrityFailed() {
		fmt.Fprintln(stderr, "restored ledger failed independent verification")
		return exitRefused
	}
	if _, err := kernel.Rebuild(ctx, postgres.RebuildProjections()...); err != nil {
		fmt.Fprintln(stderr, err)
		return exitRefused
	}
	verify, err := kernel.Verify(ctx)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	if !verify.Valid {
		fmt.Fprintln(stderr, "post-rebuild ledger verification failed")
		return exitRefused
	}

	restoredAt := time.Now().UTC()
	result := restoreResult{BackupVerified: true, LedgerValid: true, LedgerEntries: verify.Entries, RestoredAt: restoredAt}
	evidence := restoreEvidence{Kind: "restore-rehearsal", RestoredAt: restoredAt, BackupSHA256: backupMeta.SHA256, LedgerEntries: verify.Entries, LedgerVerified: true}
	if err := writeRestoreEvidence(cfg.ExportDir, evidence); err != nil {
		result.EvidenceWarning = err.Error()
	} else {
		result.EvidenceRecorded = true
	}
	return writeJSON(stdout, stderr, result)
}
