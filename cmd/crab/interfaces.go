package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/adapters/artifacts"
	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/buildinfo"
	"github.com/jonathanselby/cyber-response-action-board/internal/config"
	"github.com/jonathanselby/cyber-response-action-board/internal/data/postgres"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	mcppresentation "github.com/jonathanselby/cyber-response-action-board/internal/presentation/mcp"
	pdfpresentation "github.com/jonathanselby/cyber-response-action-board/internal/presentation/pdf"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

type printEvidenceReader struct{ store *postgres.EvidenceStore }

func (r printEvidenceReader) Register(ctx context.Context, _ authz.Principal, incidentID, compartment string) ([]domain.EvidenceView, error) {
	return r.store.Register(ctx, incidentID, compartment)
}

func runMCP(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	sessionFile := fs.String("session-file", "", "path to a local CRAB operator session token file")
	if err := fs.Parse(args); err != nil || strings.TrimSpace(*sessionFile) == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: crab mcp --session-file <path>")
		return exitUsage
	}
	token, err := readOperatorSessionFile(*sessionFile)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitRefused
	}
	runtimeValue, err := newOperatorRuntime()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	runtime, ok := runtimeValue.(*operatorRuntime)
	if !ok {
		fmt.Fprintln(stderr, "operator runtime unavailable")
		return exitOperational
	}
	if _, authenticated, err := runtime.principal(context.Background(), token); err != nil || !authenticated {
		fmt.Fprintln(stderr, "operator session is invalid or expired")
		return exitRefused
	}
	server := mcppresentation.Server{Runtime: runtime, Token: token, Version: buildinfo.Version}
	if err := server.Serve(context.Background(), stdin, stdout); err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	return exitOK
}

func runPrint(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: crab print incident-brief|emergency-pack [options]")
		return exitUsage
	}
	switch args[0] {
	case "incident-brief":
		return runPrintIncidentBrief(args[1:], stdout, stderr)
	case "emergency-pack":
		return runPrintEmergencyPack(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown print document %q\n", args[0])
		return exitUsage
	}
}

func runPrintIncidentBrief(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("print incident-brief", flag.ContinueOnError)
	fs.SetOutput(stderr)
	incidentID := fs.String("incident", "", "incident identifier")
	compartment := fs.String("compartment", "GENERAL", "authorised incident compartment")
	pageSize := fs.String("page-size", "", "A4 or LETTER")
	mode := fs.String("mode", domain.PrintColour, "colour or monochrome")
	sessionFile := fs.String("session-file", "", "path to a local CRAB operator session token file")
	if err := fs.Parse(args); err != nil || strings.TrimSpace(*incidentID) == "" || strings.TrimSpace(*sessionFile) == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: crab print incident-brief --incident <id> --session-file <path> [--compartment GENERAL] [--page-size A4|LETTER] [--mode colour|monochrome]")
		return exitUsage
	}
	service, principal, cfg, err := authenticatedPrintService(*sessionFile)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitRefused
	}
	selectedPageSize := *pageSize
	if selectedPageSize == "" {
		selectedPageSize = cfg.PageSizeDefault
	}
	result, err := service.GenerateIncidentBrief(context.Background(), principal, strings.TrimSpace(*incidentID), strings.TrimSpace(*compartment), nil, selectedPageSize, *mode, time.Now().UTC())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitRefused
	}
	return writeJSON(stdout, stderr, result)
}

func runPrintEmergencyPack(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("print emergency-pack", flag.ContinueOnError)
	fs.SetOutput(stderr)
	organisation := fs.String("organisation", "", "organisation label for the contingency pack")
	mode := fs.String("incident-mode", "live", "live or exercise")
	pageSize := fs.String("page-size", "", "A4 or LETTER")
	printMode := fs.String("mode", domain.PrintColour, "colour or monochrome")
	sessionFile := fs.String("session-file", "", "path to a local CRAB operator session token file")
	if err := fs.Parse(args); err != nil || strings.TrimSpace(*organisation) == "" || strings.TrimSpace(*sessionFile) == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: crab print emergency-pack --organisation <name> --session-file <path> [--incident-mode live|exercise] [--page-size A4|LETTER] [--mode colour|monochrome]")
		return exitUsage
	}
	service, principal, cfg, err := authenticatedPrintService(*sessionFile)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitRefused
	}
	selectedPageSize := *pageSize
	if selectedPageSize == "" {
		selectedPageSize = cfg.PageSizeDefault
	}
	pack := domain.EmergencyResponsePack{
		Organisation: strings.TrimSpace(*organisation),
		Mode:         strings.TrimSpace(*mode),
		Sections: []domain.PackSection{
			{Title: "Operating rule", Body: "The model proposes. One person makes. Another confirms. Nothing is erased."},
			{Title: "Degraded operation", Body: "If live CRAB or PostgreSQL is unavailable, use these paper forms. Preserve original occurrence times and source references. Reconcile named-human transcriptions into CRAB after restoration; do not backdate the digital ledger."},
			{Title: "Authority", Body: "Paper operation does not create new authority. Existing incident roles, compartment boundaries and maker-checker responsibilities continue to apply."},
		},
		Forms: domain.DefaultContingencyForms(),
	}
	result, err := service.GenerateEmergencyResponsePack(context.Background(), principal, pack, selectedPageSize, *printMode, time.Now().UTC())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitRefused
	}
	return writeJSON(stdout, stderr, result)
}

func authenticatedPrintService(sessionFile string) (app.PrintService, authz.Principal, config.Config, error) {
	token, err := readOperatorSessionFile(sessionFile)
	if err != nil {
		return app.PrintService{}, authz.Principal{}, config.Config{}, err
	}
	cfg, err := config.Load()
	if err != nil {
		return app.PrintService{}, authz.Principal{}, config.Config{}, err
	}
	if strings.TrimSpace(cfg.ExportDir) == "" {
		return app.PrintService{}, authz.Principal{}, config.Config{}, errors.New("CRAB_EXPORT_DIR is required for print output")
	}
	runtimeValue, err := newOperatorRuntime()
	if err != nil {
		return app.PrintService{}, authz.Principal{}, config.Config{}, err
	}
	runtime, ok := runtimeValue.(*operatorRuntime)
	if !ok {
		return app.PrintService{}, authz.Principal{}, config.Config{}, errors.New("operator runtime unavailable")
	}
	principal, authenticated, err := runtime.principal(context.Background(), token)
	if err != nil || !authenticated {
		return app.PrintService{}, authz.Principal{}, config.Config{}, errors.New("operator session is invalid or expired")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := postgres.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return app.PrintService{}, authz.Principal{}, config.Config{}, err
	}
	// The short-lived print command owns this auxiliary read pool; service calls
	// complete synchronously before process exit.
	printSource, err := postgres.NewPrintStore(pool)
	if err != nil {
		pool.Close()
		return app.PrintService{}, authz.Principal{}, config.Config{}, err
	}
	evidenceStore, err := postgres.NewEvidenceStore(pool)
	if err != nil {
		pool.Close()
		return app.PrintService{}, authz.Principal{}, config.Config{}, err
	}
	artifactStore, err := artifacts.New(cfg.ExportDir, cfg.SecondaryExportDir)
	if err != nil {
		pool.Close()
		return app.PrintService{}, authz.Principal{}, config.Config{}, err
	}
	service := app.PrintService{
		Incident:             runtime.incident,
		Evidence:             printEvidenceReader{store: evidenceStore},
		Source:               printSource,
		Renderer:             pdfpresentation.Renderer{},
		Artifacts:            artifactStore,
		ConfigurationVersion: buildinfo.Version,
		TimeStatus:           func() string { return runtime.kernel.Clock().Read().Status },
		Organisation:         runtime.organisation,
	}
	return service, principal, cfg, nil
}

func readOperatorSessionFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("operator session file: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return "", errors.New("operator session file must be a non-empty regular file")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return "", errors.New("operator session file must not be readable or writable by group/other")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read operator session file: %w", err)
	}
	var wrapped struct {
		Token string `json:"token"`
	}
	if json.Unmarshal(b, &wrapped) == nil && strings.TrimSpace(wrapped.Token) != "" {
		return strings.TrimSpace(wrapped.Token), nil
	}
	token := strings.TrimSpace(string(b))
	if token == "" || strings.ContainsAny(token, "\r\n\t ") {
		return "", errors.New("operator session file contains an invalid token")
	}
	return token, nil
}
