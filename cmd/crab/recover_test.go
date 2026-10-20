package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecoveryRequestFileRequiresPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recover.json")
	body := []byte(`{"prepared_by":"alice","confirmed_by":"bob","reason":"lost normal quorum","restore_custodian_ids":["alice","bob"],"presented_break_glass":{"alice":"proof-a","bob":"proof-b"}}`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	var req recoveryRequest
	if err := readPrivateJSONFile(path, &req); err == nil {
		t.Fatal("expected group/other-readable recovery request to be refused")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := readPrivateJSONFile(path, &req); err != nil {
		t.Fatal(err)
	}
	if req.PreparedBy != "alice" || req.ConfirmedBy != "bob" || len(req.RestoreCustodianIDs) != 2 {
		t.Fatalf("request=%+v", req)
	}
}
