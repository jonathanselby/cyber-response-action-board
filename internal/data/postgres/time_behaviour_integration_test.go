package postgres

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/adapters/clock"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

// T-244 (R-19): a write whose time is earlier than the last ledger record is
// kept with its stated time but marked untrusted; the ledger still verifies.
func TestBackwardsClockWritesAreMarkedUntrusted(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	kernel, err := OpenKernel(ctx, db.runtime, ledger.Keyring{"server": base64.StdEncoding.EncodeToString(pub)})
	if err != nil {
		t.Fatal(err)
	}
	signer := Signer{KeyID: "server", PrivateKey: priv}
	scope := domain.Scope{IncidentID: "INC-T", Mode: "live", Compartment: "GENERAL"}
	t1 := time.Date(2026, 11, 2, 3, 0, 0, 0, time.UTC)
	write := func(id string, at time.Time) ledger.Record {
		t.Helper()
		kernel.SetClock(clock.Fixed{At: at, Status: clock.Trusted})
		rec, err := domain.ProposeDecision(scope, id, "alice", at, domain.Decision{Decision: id, Rationale: "r"})
		if err != nil {
			t.Fatal(err)
		}
		entry, err := kernel.Append(ctx, rec, signer)
		if err != nil {
			t.Fatal(err)
		}
		return entry.Record
	}
	if got := write("D-1", t1).TimeStatus; got != ledger.TimeTrusted {
		t.Fatalf("first write status %q", got)
	}
	back := write("D-2", t1.Add(-time.Hour))
	if back.TimeStatus != ledger.TimeUntrusted || !back.Timestamp.Equal(t1.Add(-time.Hour)) {
		t.Fatalf("backwards write: status %q at %s; want untrusted with its stated time", back.TimeStatus, back.Timestamp)
	}
	if got := write("D-3", t1.Add(time.Hour)).TimeStatus; got != ledger.TimeTrusted {
		t.Fatalf("forward write after recovery status %q", got)
	}
	var raw []byte
	if err := db.admin.QueryRow(ctx, `SELECT entry_bytes FROM crab.ledger_entries WHERE seq=2`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var entry ledger.Entry
	if err := json.Unmarshal(raw, &entry); err != nil || entry.Record.TimeStatus != ledger.TimeUntrusted {
		t.Fatalf("stored backwards record not marked: %v %q", err, entry.Record.TimeStatus)
	}
	if res, err := kernel.Verify(ctx); err != nil || !res.Valid {
		t.Fatalf("ledger does not verify after a backwards write: %v %+v", err, res)
	}
}
