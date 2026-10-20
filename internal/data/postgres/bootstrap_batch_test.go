package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/security/ceremony"
)

func TestBootstrapBatchProjectsMinimumAuthority(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	keyring, _, serverPriv := keys(t)
	kernel, err := OpenKernel(ctx, db.runtime, keyring)
	if err != nil {
		t.Fatal(err)
	}
	records, err := ceremony.BootstrapRecords(ceremony.BootstrapPlan{
		PreparedBy: "alice", ConfirmedBy: "bob",
		Identities: []ceremony.BootstrapIdentity{
			{HumanID: "alice", Custodian: true, MajorEligible: true, LocalAuthConfigured: true},
			{HumanID: "bob", Custodian: true, MajorEligible: true, LocalAuthConfigured: true},
		},
		BreakGlassKeyholderIDs: []string{"alice", "bob"},
	}, time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	for i := range records {
		records[i].MakerSignature.MissingReason = "maker authenticated in two-person bootstrap ceremony"
		if records[i].WriteSemantic == "governed" && records[i].Status != "proposed" {
			records[i].ConfirmerSignature.MissingReason = "confirmer authenticated in two-person bootstrap ceremony"
		}
	}
	if _, err := kernel.AppendBatch(ctx, records, Signer{KeyID: "server", PrivateKey: serverPriv}, AccessProjection{}); err != nil {
		t.Fatal(err)
	}
	store, err := NewAccessStore(db.runtime)
	if err != nil {
		t.Fatal(err)
	}
	ready, err := store.Bootstrapped(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !ready {
		t.Fatal("bootstrap authority was not projected")
	}
}
