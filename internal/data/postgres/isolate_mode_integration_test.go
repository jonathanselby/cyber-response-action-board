package postgres

import (
	"context"
	"testing"

	"github.com/jonathanselby/cyber-response-action-board/internal/security/access"
)

func TestIsolateModeIsGovernedAndRebuildable(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	keys := newAccessKeys(t)
	kernel, err := OpenKernel(ctx, db.runtime, keys.keyring)
	if err != nil {
		t.Fatal(err)
	}
	projection := AccessProjection{}
	signer := Signer{KeyID: "server", PrivateKey: keys.serverPriv}
	record := confirmedAccessRecord(t, keys, access.RecordIsolateMode, "isolate-1", "", "system", "SYSTEM", map[string]any{"active": true})
	if _, err := kernel.Append(ctx, record, signer, projection); err != nil {
		t.Fatal(err)
	}
	store, err := NewAccessStore(db.runtime)
	if err != nil {
		t.Fatal(err)
	}
	active, err := store.IsolateMode(ctx)
	if err != nil || !active {
		t.Fatalf("isolate active=%v err=%v", active, err)
	}
	if _, err := db.owner.Exec(ctx, `TRUNCATE crab.security_state`); err != nil {
		t.Fatal(err)
	}
	active, err = store.IsolateMode(ctx)
	if err != nil || active {
		t.Fatalf("truncated isolate active=%v err=%v", active, err)
	}
	if _, err := kernel.Rebuild(ctx, projection); err != nil {
		t.Fatal(err)
	}
	active, err = store.IsolateMode(ctx)
	if err != nil || !active {
		t.Fatalf("rebuilt isolate active=%v err=%v", active, err)
	}
}
