package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/jonathanselby/cyber-response-action-board/internal/adapters/clock"
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
	case "setup":
		return runSetup(stdout, stderr)
	case "serve":
		return runServe(stdout, stderr)
	case "mcp":
		return runMCP(args[1:], os.Stdin, stdout, stderr)
	case "print":
		return runPrint(args[1:], stdout, stderr)
	case "recover":
		return runRecover(args[1:], stdout, stderr)
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
					check.Message = "schema, runtime role and full ledger verification passed"
				}
				pool.Close()
			}
			cancel()
		}
		for i := range r.Checks {
			if r.Checks[i].Name == "postgresql" {
				r.Checks[i] = check
			}
		}
	}
	enrichReadiness(cfg, &r, clock.NewHost(), time.Time{})
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

func runSetup(stdout, stderr io.Writer) int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	if !cfg.IsLoopbackListen() {
		fmt.Fprintln(stderr, "crab setup is local-only; configure a loopback listen address")
		return exitRefused
	}
	fmt.Fprintln(stdout, "CRAB setup uses the local two-person browser ceremony; no credentials are accepted on the command line.")
	return runServe(stdout, stderr)
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

	if cfg.TLSCertFile != "" {
		fmt.Fprintf(stdout, "CRAB build checkpoint %s listening with HTTPS on %s\n", buildinfo.Version, cfg.ListenAddr)
		if err := srv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(stderr, err)
			return exitOperational
		}
		return exitOK
	}
	fmt.Fprintf(stdout, "CRAB build checkpoint %s listening locally on %s\n", buildinfo.Version, cfg.ListenAddr)
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

Implemented through the current verified checkpoint:
  crab setup
  crab serve
  crab readiness
  crab verify --ledger <file> --keyring <file>
  crab rebuild
  crab export-ledger --output <file>
  crab backup --output <path>
  crab restore --input <path>
  crab mcp --session-file <path>
  crab print incident-brief --incident <id> --session-file <path> [--page-size A4|LETTER]
  crab print emergency-pack --organisation <name> --session-file <path> [--page-size A4|LETTER]
  crab recover --request-file <path>
  crab version`)
}
