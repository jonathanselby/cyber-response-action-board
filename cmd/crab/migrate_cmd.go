package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jonathanselby/cyber-response-action-board/internal/config"
	"github.com/jonathanselby/cyber-response-action-board/internal/data/postgres"
)

type migrateResult struct {
	FromVersion    int  `json:"from_version"`
	ToVersion      int  `json:"to_version"`
	LedgerVerified bool `json:"ledger_verified"`
	LedgerEntries  int  `json:"ledger_entries"`
}

// runMigrate brings the schema to this release's version (R-15). It uses the
// schema owner's connection (CRAB_MAINTENANCE_DATABASE_URL) and grants the
// runtime role named in CRAB_DATABASE_URL. All migrations run in one
// transaction: a failure or interruption leaves the previous schema intact.
// The ledger is verified afterwards with the runtime role.
func runMigrate(stdout, stderr io.Writer) int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	if cfg.DatabaseURL == "" || cfg.MaintenanceDatabaseURL == "" || cfg.LedgerKeyringFile == "" {
		fmt.Fprintln(stderr, "CRAB_DATABASE_URL, CRAB_MAINTENANCE_DATABASE_URL and CRAB_LEDGER_KEYRING_FILE are required")
		return exitRefused
	}
	runtimeCfg, err := pgx.ParseConfig(cfg.DatabaseURL)
	if err != nil || runtimeCfg.User == "" {
		fmt.Fprintln(stderr, "CRAB_DATABASE_URL must name the runtime role")
		return exitRefused
	}
	keys, err := loadKeyring(cfg.LedgerKeyringFile)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	owner, err := postgres.OpenPool(ctx, cfg.MaintenanceDatabaseURL)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	defer owner.Close()
	from, err := postgres.SchemaVersion(ctx, owner)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	if from > postgres.CurrentSchemaVersion() {
		fmt.Fprintf(stderr, "schema version %d is newer than this release (%d); refusing to migrate\n", from, postgres.CurrentSchemaVersion())
		return exitRefused
	}
	if err := postgres.Migrate(ctx, owner, runtimeCfg.User); err != nil {
		fmt.Fprintf(stderr, "migration failed; the schema is unchanged at version %d: %v\n", from, err)
		return exitOperational
	}
	to, err := postgres.SchemaVersion(ctx, owner)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	runtime, err := postgres.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	defer runtime.Close()
	kernel, err := postgres.OpenKernel(ctx, runtime, keys)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	verify, err := kernel.Verify(ctx)
	if err != nil || !verify.Valid {
		fmt.Fprintf(stderr, "ledger verification failed after migration: %v\n", err)
		return exitRefused
	}
	return writeJSON(stdout, stderr, migrateResult{FromVersion: from, ToVersion: to, LedgerVerified: true, LedgerEntries: verify.Entries})
}
