package postgres

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

// Organisation configuration binds decisions to the exact proposal and
// rebuilds deterministically from the ledger.
func TestOrganisationProjectionBindsAndRebuilds(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	keys, _, serverPriv := keys(t)
	k, err := OpenKernel(ctx, db.runtime, keys)
	if err != nil {
		t.Fatal(err)
	}
	signer := Signer{KeyID: "server", PrivateKey: serverPriv}
	runtime, err := NewOrganisationRuntime(k, signer)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 26, 4, 0, 0, 0, time.UTC)
	proposal, err := domain.ProposeSeverityScale("SCALE-1", "alice", now, domain.DefaultSeverityScale())
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.AppendOrganisation(ctx, proposal); err != nil {
		t.Fatal(err)
	}
	stored, found, err := runtime.PendingOrganisation(ctx, "SCALE-1")
	if err != nil || !found {
		t.Fatalf("pending: %v %v", found, err)
	}
	version, _ := domain.ProposalVersion(stored)

	// A decision carrying a different payload is refused by the projection.
	forged := stored
	other, _ := domain.ProposeSeverityScale("SCALE-1", "alice", now, domain.SeverityScale{Levels: []domain.SeverityLevel{{Label: "A"}, {Label: "B"}}})
	forged.Data = other.Data
	otherVersion, _ := domain.ProposalVersion(other)
	if bad, err := domain.Decide(forged, otherVersion, "bob", "confirmed", now); err == nil {
		if err := runtime.AppendOrganisation(ctx, bad); err == nil {
			t.Fatal("decision with a different payload was accepted")
		}
	}
	if _, err := domain.Decide(stored, version, "alice", "confirmed", now); err == nil {
		t.Fatal("maker confirmed their own organisation change")
	}
	confirmed, err := domain.Decide(stored, version, "bob", "confirmed", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.AppendOrganisation(ctx, confirmed); err != nil {
		t.Fatal(err)
	}
	before, err := runtime.CurrentOrganisation(ctx, domain.TypeSeverityScale)
	if err != nil || len(before) != 1 {
		t.Fatalf("current: %v %v", before, err)
	}
	if _, err := k.Rebuild(ctx, RebuildProjections()...); err != nil {
		t.Fatal(err)
	}
	after, err := runtime.CurrentOrganisation(ctx, domain.TypeSeverityScale)
	if err != nil || len(after) != 1 || after[0].RecordID != before[0].RecordID || after[0].Confirmer != "bob" {
		t.Fatalf("rebuild changed current scale: %+v", after)
	}
	var scale domain.SeverityScale
	_ = json.Unmarshal(after[0].Payload, &scale)
	if !scale.Contains("Severity level 5") || after[0].TimeStatus == "" {
		t.Fatalf("rebuilt scale %+v", scale)
	}
}
