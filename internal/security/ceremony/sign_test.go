package ceremony

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

func TestBootstrapRecordsCanBeHumanSignedAndLedgered(t *testing.T) {
	records, err := BootstrapRecords(validBootstrap(), time.Date(2026, 10, 20, 5, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	makerPub, makerPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	confirmerPub, confirmerPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serverPub, serverPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := SignGovernedRecords(records,
		HumanSigner{HumanID: "alice", KeyID: "alice-key", PrivateKey: makerPriv},
		HumanSigner{HumanID: "bob", KeyID: "bob-key", PrivateKey: confirmerPriv},
	)
	if err != nil {
		t.Fatal(err)
	}
	keyring := ledger.Keyring{
		"alice-key":  base64.StdEncoding.EncodeToString(makerPub),
		"bob-key":    base64.StdEncoding.EncodeToString(confirmerPub),
		"server-key": base64.StdEncoding.EncodeToString(serverPub),
	}
	var stream bytes.Buffer
	previous := ""
	for i, record := range signed {
		entry, err := ledger.BuildEntry(uint64(i+1), previous, record, "server-key", serverPriv, keyring)
		if err != nil {
			t.Fatalf("build ceremony ledger entry %d: %v", i+1, err)
		}
		encoded, err := encodeLedgerEntry(entry)
		if err != nil {
			t.Fatal(err)
		}
		stream.Write(encoded)
		previous = entry.Hash
	}
	result := ledger.Verify(&stream, keyring)
	if !result.Valid || result.Entries != len(signed) {
		t.Fatalf("signed ceremony ledger verify=%+v", result)
	}
}

func TestCeremonySignerRefusesPrincipalMismatch(t *testing.T) {
	records, err := BootstrapRecords(validBootstrap(), time.Date(2026, 10, 20, 5, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	_, makerPriv, _ := ed25519.GenerateKey(rand.Reader)
	_, confirmerPriv, _ := ed25519.GenerateKey(rand.Reader)
	if _, err := SignGovernedRecords(records,
		HumanSigner{HumanID: "mallory", KeyID: "m", PrivateKey: makerPriv},
		HumanSigner{HumanID: "bob", KeyID: "b", PrivateKey: confirmerPriv},
	); err == nil {
		t.Fatal("ceremony signer accepted mismatched maker identity")
	}
}

func encodeLedgerEntry(entry ledger.Entry) ([]byte, error) {
	return appendJSONLine(entry)
}
