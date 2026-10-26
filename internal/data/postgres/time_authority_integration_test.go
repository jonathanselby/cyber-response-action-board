package postgres

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/adapters/clock"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

func unsignedTestRecord(id, status string) ledger.Record {
	return ledger.Record{
		SchemaVersion:  1,
		Timestamp:      time.Date(2026, 10, 26, 1, 0, 0, 0, time.UTC),
		TimeStatus:     status,
		IncidentID:     "INC-T",
		IncidentMode:   "exercise",
		WriteSemantic:  "record",
		RecordType:     "test-record",
		RecordID:       id,
		Status:         "recorded",
		Maker:          "maker",
		Compartment:    "GENERAL",
		Data:           json.RawMessage(`{"a":1}`),
		MakerSignature: ledger.Signature{MissingReason: "test"},
	}
}

// T-194 / T-195 / T-196: every append is capped by the one clock; a signed
// record is left byte-identical; nothing is strengthened.
func TestKernelCapsTimeStatusByClock(t *testing.T) {
	db := setupTestDB(t)
	prepareProjection(t, db)
	ctx := context.Background()
	keys, makerPriv, serverPriv := keys(t)
	k, err := OpenKernel(ctx, db.runtime, keys)
	if err != nil {
		t.Fatal(err)
	}
	signer := Signer{KeyID: "server", PrivateKey: serverPriv}
	cases := []struct {
		clock, record, want string
	}{
		{clock.Untrusted, ledger.TimeTrusted, ledger.TimeUntrusted},
		{clock.Drifting, ledger.TimeTrusted, ledger.TimeDrifting},
		{clock.Trusted, ledger.TimeTrusted, ledger.TimeTrusted},
		{clock.Trusted, ledger.TimeUntrusted, ledger.TimeUntrusted},
		{clock.Trusted, ledger.TimeDrifting, ledger.TimeDrifting},
	}
	for i, c := range cases {
		k.SetClock(clock.Fixed{Status: c.clock})
		entry, err := k.Append(ctx, unsignedTestRecord("R-"+string(rune('a'+i)), c.record), signer, tableProjection{})
		if err != nil {
			t.Fatal(err)
		}
		if entry.Record.TimeStatus != c.want {
			t.Errorf("clock=%s record=%s stored=%s want %s", c.clock, c.record, entry.Record.TimeStatus, c.want)
		}
	}
	k.SetClock(clock.Fixed{Status: clock.Untrusted})
	batch, err := k.AppendBatch(ctx, []ledger.Record{unsignedTestRecord("B-2", ledger.TimeTrusted)}, signer, tableProjection{})
	if err != nil {
		t.Fatal(err)
	}
	if batch[0].Record.TimeStatus != ledger.TimeUntrusted {
		t.Fatalf("batch path not capped: %s", batch[0].Record.TimeStatus)
	}
	signed := signedRecord(t, makerPriv, "S-1")
	entry, err := k.Append(ctx, signed, signer, tableProjection{})
	if err != nil {
		t.Fatalf("signed record must stay verifiable: %v", err)
	}
	if entry.Record.TimeStatus != signed.TimeStatus {
		t.Fatalf("signed record changed: %s", entry.Record.TimeStatus)
	}
	if res, err := k.Verify(ctx); err != nil || !res.Valid {
		t.Fatalf("ledger invalid after stamping: %+v %v", res, err)
	}
}
