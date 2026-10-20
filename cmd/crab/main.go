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
	"syscall"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/buildinfo"
	"github.com/jonathanselby/cyber-response-action-board/internal/config"
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
	case "serve":
		return runServe(stdout, stderr)
	case "setup", "mcp", "backup", "restore", "rebuild", "export-ledger", "print", "recover":
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

Implemented in 0.0.1-dev:
  crab serve
  crab readiness
  crab verify --ledger <file> --keyring <file>
  crab version

Frozen CLI surface, intentionally refused until its implementation slice lands:
  crab setup
  crab mcp
  crab backup --output <path>
  crab restore --input <path>
  crab rebuild
  crab export-ledger --output <file>
  crab print incident-brief --incident <id> [--page-size A4|LETTER]
  crab print emergency-pack [--page-size A4|LETTER]`)
}
