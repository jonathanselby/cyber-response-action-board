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

	"github.com/jonathanselby/cyber-response-action-board/internal/data/postgres"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/ceremony"
)

type recoveryRequest struct {
	PreparedBy          string            `json:"prepared_by"`
	ConfirmedBy         string            `json:"confirmed_by"`
	Reason              string            `json:"reason"`
	RestoreCustodianIDs []string          `json:"restore_custodian_ids"`
	PresentedBreakGlass map[string]string `json:"presented_break_glass"`
}

func runRecover(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("recover", flag.ContinueOnError)
	fs.SetOutput(stderr)
	requestFile := fs.String("request-file", "", "private JSON Custodian recovery request")
	if err := fs.Parse(args); err != nil || strings.TrimSpace(*requestFile) == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: crab recover --request-file <path>")
		return exitUsage
	}
	var req recoveryRequest
	if err := readPrivateJSONFile(*requestFile, &req); err != nil {
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	now := time.Now().UTC()
	active, err := runtime.access.ActiveCustodianSessionCount(ctx, now)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	if active >= 2 {
		fmt.Fprintln(stderr, "Custodian recovery refused: a normal Custodian quorum is demonstrably usable")
		return exitRefused
	}
	breakGlass, err := runtime.security.ActiveBreakGlassRecords(ctx)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitOperational
	}
	records, err := ceremony.CustodianRecoveryRecords(ceremony.RecoveryPlan{
		PreparedBy:          strings.TrimSpace(req.PreparedBy),
		ConfirmedBy:         strings.TrimSpace(req.ConfirmedBy),
		Reason:              strings.TrimSpace(req.Reason),
		RestoreCustodianIDs: req.RestoreCustodianIDs,
		PresentedBreakGlass: req.PresentedBreakGlass,
	}, breakGlass, now)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitRefused
	}
	entries, err := runtime.kernel.AppendBatch(ctx, records, runtime.signer, postgres.AccessProjection{})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitRefused
	}
	last := entries[len(entries)-1]
	return writeJSON(stdout, stderr, map[string]any{
		"status":              "custodian quorum restored",
		"restored_custodians": req.RestoreCustodianIDs,
		"ledger_sequence":     last.Seq,
		"ledger_hash":         last.Hash,
	})
}

func readPrivateJSONFile(path string, target any) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("recovery request file: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return errors.New("recovery request file must be a non-empty regular file")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return errors.New("recovery request file must not be readable or writable by group/other")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read recovery request file: %w", err)
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return fmt.Errorf("parse recovery request file: %w", err)
	}
	return nil
}
