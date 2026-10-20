package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/buildinfo"
	"github.com/jonathanselby/cyber-response-action-board/internal/config"
	"github.com/jonathanselby/cyber-response-action-board/internal/data/postgres"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/presentation/web"
)

const (
	exitOK          = 0
	exitRefused     = 1
	exitUsage       = 2
	exitOperational = 3
)

type operationsState struct {
	LastBackup  *postgres.BackupMetadata `json:"last_backup,omitempty"`
	LastRestore *time.Time               `json:"last_restore,omitempty"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return exitUsage
	}
	switch args[0] {
	case "version":
		fmt.Fprintf(stdout, "crab %s commit=%s built=%s\n", buildinfo.Version, buildinfo.Commit, buildinfo.BuiltAt)
		return exitOK
	case "readiness":
		return runReadiness(stdout, stderr)
	case "verify":
		return runVerify(args[1:], stdout, stderr)
	case "rebuild":
		return runRebuild(stdout, stderr)
	case "export-ledger":
		return runExportLedger(args[1:], stdout, stderr)
	case "backup":
		return runBackup(args[1:], stdout, stderr)
	case "restore":
		return runRestore(args[1:], stdout, stderr)
	case "serve":
		return runServe(stdout, stderr)
	case "setup", "mcp", "print", "recover":
		fmt.Fprintf(stderr, "crab %s: not built in checkpoint %s; no alternate control path is enabled\n", args[0], buildinfo.Version)
		return exitRefused
	case "help", "-h", "--help":
		usage(stdout)
		return exitOK
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		usage(stderr)
		return exitUsage
	}
}

func runReadiness(stdout, stderr io.Writer) int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	r := app.CurrentReadiness(cfg)
	if cfg.DatabaseURL != "" && cfg.LedgerKeyringFile != "" {
		check := app.Check{Name: "postgresql", Status: "fail"}
		keys, err := loadKeyring(cfg.LedgerKeyringFile)
		if err != nil {
			check.Message = err.Error()
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			pool, openErr := postgres.OpenPool(ctx, cfg.DatabaseURL)
			if openErr != nil {
				check.Message = openErr.Error()
			} else {
				kernel, kernelErr := postgres.OpenKernel(ctx, pool, keys)
				if kernelErr != nil {
					check.Message = kernelErr.Error()
				} else if kernel.IntegrityFailed() {
					check.Message = "ledger verification failed; read-only integrity mode"
				} else {
					check.Status = "pass"
					check.Message = "PostgreSQL reachable; schema, runtime role and full ledger verification passed"
				}
				pool.Close()
			}
			cancel()
		}
		setCheck(&r, check)
	}

	keyCheck := app.Check{Name: "key_custody", Status: "pass", Message: "configured local key files are readable and non-empty"}
	for name, path := range map[string]string{
		"ledger keyring":       cfg.LedgerKeyringFile,
		"session key":          cfg.SessionKeyFile,
		"server signing key":   cfg.ServerSigningKeyFile,
		"credential wrap key":  cfg.CredentialWrapKeyFile,
	} {
		if err := readableNonEmptyFile(path); err != nil {
			keyCheck.Status = "fail"
			keyCheck.Message = fmt.Sprintf("%s: %v", name, err)
			break
		}
	}
	setCheck(&r, keyCheck)

	backupCheck := app.Check{Name: "backup_restore", Status: "pending", Message: "no successful backup and restore rehearsal recorded"}
	if state, err := readOperationsState(cfg); err == nil && state.LastBackup != nil && state.LastRestore != nil {
		backupCheck.Status = "pass"
		backupCheck.Message = fmt.Sprintf("last backup %s; last restore/rebuild rehearsal %s", state.LastBackup.CreatedAt.UTC().Format(time.RFC3339), state.LastRestore.UTC().Format(time.RFC3339))
	} else if err != nil {
		backupCheck.Message = err.Error()
	}
	setCheck(&r, backupCheck)
	r = app.FinaliseReadiness(r)

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	if !r.Ready {
		return exitRefused
	}
	return exitOK
}

func setCheck(r *app.Readiness, check app.Check) {
	for i := range r.Checks {
		if r.Checks[i].Name == check.Name {
			r.Checks[i] = check
			return
		}
	}
	r.Checks = append(r.Checks, check)
}

func readableNonEmptyFile(path string) error {
	if path == "" {
		return errors.New("not configured")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return errors.New("must be a non-empty regular file")
	}
	return nil
}

func operationsStatePath(cfg config.Config) (string, error) {
	if cfg.ExportDir == "" {
		return "", errors.New("CRAB_EXPORT_DIR is required for backup/restore readiness evidence")
	}
	return filepath.Join(cfg.ExportDir, ".crab-operations-state.json"), nil
}

func readOperationsState(cfg config.Config) (operationsState, error) {
	path, err := operationsStatePath(cfg)
	if err != nil {
		return operationsState{}, err
	}
	var state operationsState
	if err := readJSONFile(path, &state); err != nil {
		return operationsState{}, err
	}
	return state, nil
}

func writeOperationsState(cfg config.Config, state operationsState) error {
	path, err := operationsStatePath(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".crab-operations-state-*.tmp")
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
	if err != nil || !verify.Valid {
		if err != nil {
			fmt.Fprintln(stderr, err)
		} else {
			fmt.Fprintln(stderr, "ledger verification failed; backup refused")
		}
		return exitRefused
	}
	meta, err := postgres.Backup(ctx, cfg.DatabaseURL, *output)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	state, _ := readOperationsState(cfg)
	state.LastBackup = &meta
	if err := writeOperationsState(cfg, state); err != nil {
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
	kernel, err := postgres.OpenKernel(ctx, pool, keys)
	if err != nil {
		pool.Close()
		fmt.Fprintln(stderr, err)
		return exitRefused
	}
	if kernel.IntegrityFailed() {
		pool.Close()
		fmt.Fprintln(stderr, "restored ledger failed independent verification")
		return exitRefused
	}
	if _, err := kernel.Rebuild(ctx, postgres.RebuildProjections()...); err != nil {
		pool.Close()
		fmt.Fprintln(stderr, err)
		return exitRefused
	}
	verify, err := kernel.Verify(ctx)
	pool.Close()
	if err != nil || !verify.Valid {
		if err != nil {
			fmt.Fprintln(stderr, err)
		} else {
			fmt.Fprintln(stderr, "post-rebuild ledger verification failed")
		}
		return exitRefused
	}
	now := time.Now().UTC()
	state, _ := readOperationsState(cfg)
	state.LastRestore = &now
	if err := writeOperationsState(cfg, state); err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	return runReadiness(stdout, stderr)
}

func runRebuild(stdout, stderr io.Writer) int {
	keys, pool, code := openLedgerDatabase(stderr)
	if code != exitOK {
		return code
	}
	defer pool.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	kernel, err := postgres.OpenKernel(ctx, pool, keys)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	res, err := kernel.Rebuild(ctx, postgres.RebuildProjections()...)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitRefused
	}
	return writeJSON(stdout, stderr, res)
}

func runExportLedger(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("export-ledger", flag.ContinueOnError)
	fs.SetOutput(stderr)
	output := fs.String("output", "", "portable ledger JSONL output path")
	if err := fs.Parse(args); err != nil || *output == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: crab export-ledger --output <file>")
		return exitUsage
	}
	keys, pool, code := openLedgerDatabase(stderr)
	if code != exitOK {
		return code
	}
	defer pool.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	kernel, err := postgres.OpenKernel(ctx, pool, keys)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	dir := filepath.Dir(*output)
	tmp, err := os.CreateTemp(dir, ".crab-ledger-*.tmp")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
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
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	res, err := kernel.Export(ctx, tmp)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitRefused
	}
	if err := tmp.Sync(); err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	if err := tmp.Close(); err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	if err := os.Rename(tmpName, *output); err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	committed = true
	if !res.Valid {
		fmt.Fprintf(stderr, "exported %d ledger entries to %s with integrity verification FAILURE\n", res.Entries, *output)
		return exitRefused
	}
	fmt.Fprintf(stdout, "exported %d verified ledger entries to %s\n", res.Entries, *output)
	return exitOK
}

func openLedgerDatabase(stderr io.Writer) (ledger.Keyring, *pgxpool.Pool, int) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return nil, nil, exitOperational
	}
	if cfg.DatabaseURL == "" || cfg.LedgerKeyringFile == "" {
		fmt.Fprintln(stderr, "CRAB_DATABASE_URL and CRAB_LEDGER_KEYRING_FILE are required")
		return nil, nil, exitRefused
	}
	keys, err := loadKeyring(cfg.LedgerKeyringFile)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return nil, nil, exitOperational
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := postgres.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return nil, nil, exitOperational
	}
	return keys, pool, exitOK
}

func loadKeyring(path string) (ledger.Keyring, error) {
	var keys ledger.Keyring
	if err := readJSONFile(path, &keys); err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, errors.New("ledger keyring is empty")
	}
	return keys, nil
}

func writeJSON(stdout, stderr io.Writer, v any) int {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	return exitOK
}

func runVerify(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	ledgerPath := fs.String("ledger", "", "portable ledger JSONL file")
	keyringPath := fs.String("keyring", "", "public keyring JSON file")
	if err := fs.Parse(args); err != nil || *ledgerPath == "" || *keyringPath == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: crab verify --ledger <file> --keyring <file>")
		return exitUsage
	}
	var keys ledger.Keyring
	if err := readJSONFile(*keyringPath, &keys); err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	f, err := os.Open(*ledgerPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	defer f.Close()
	res := ledger.Verify(f, keys)
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(res); err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	if !res.Valid {
		return exitRefused
	}
	return exitOK
}

func runServe(stdout, stderr io.Writer) int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	ui, err := web.New()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           ui.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	fmt.Fprintf(stdout, "CRAB build checkpoint %s listening on %s\n", buildinfo.Version, cfg.ListenAddr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	return exitOK
}

func readJSONFile(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func usage(w io.Writer) {
	fmt.Fprintln(w, `CRAB — Cyber Response Action Board

Implemented through Slice 13:
  crab serve
  crab readiness
  crab verify --ledger <file> --keyring <file>
  crab rebuild
  crab export-ledger --output <file>
  crab backup --output <path>
  crab restore --input <path>
  crab version

Frozen CLI surface, intentionally refused until its implementation slice lands:
  crab setup
  crab mcp
  crab print incident-brief --incident <id> [--page-size A4|LETTER]
  crab print emergency-pack [--page-size A4|LETTER]`)
}
