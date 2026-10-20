package ledger

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type fixtureKeys struct {
	makerPriv     ed25519.PrivateKey
	confirmerPriv ed25519.PrivateKey
	serverPriv    ed25519.PrivateKey
	keyring       Keyring
}

func newFixtureKeys(t *testing.T) fixtureKeys {
	t.Helper()
	mkPub, mkPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cfPub, cfPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	svPub, svPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return fixtureKeys{
		makerPriv:     mkPriv,
		confirmerPriv: cfPriv,
		serverPriv:    svPriv,
		keyring: Keyring{
			"maker":     base64.StdEncoding.EncodeToString(mkPub),
			"confirmer": base64.StdEncoding.EncodeToString(cfPub),
			"server":    base64.StdEncoding.EncodeToString(svPub),
		},
	}
}

func governedRecord() Record {
	return Record{
		SchemaVersion: 1,
		Timestamp:     time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC),
		TimeStatus:    "trusted",
		IncidentID:    "INC-001",
		IncidentMode:  "live",
		WriteSemantic: "governed",
		RecordType:    "incident_declaration",
		RecordID:      "REC-001",
		Status:        "confirmed",
		Maker:         "alice",
		Confirmer:     "bob",
		Compartment:   "GENERAL",
		Data:          json.RawMessage(`{"title":"Bafflewick","severity":"high"}`),
	}
}

func signGoverned(t *testing.T, r Record, k fixtureKeys) Record {
	t.Helper()
	sig, err := SignRecord(r, k.makerPriv)
	if err != nil {
		t.Fatal(err)
	}
	r.MakerSignature = Signature{KeyID: "maker", Value: sig}
	sig, err = SignRecord(r, k.confirmerPriv)
	if err != nil {
		t.Fatal(err)
	}
	r.ConfirmerSignature = Signature{KeyID: "confirmer", Value: sig}
	return r
}

func encodeEntry(t *testing.T, e Entry) []byte {
	t.Helper()
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	return append(b, '\n')
}

func TestVerifyValidLedger(t *testing.T) {
	k := newFixtureKeys(t)
	r := signGoverned(t, governedRecord(), k)
	e, err := BuildEntry(1, "", r, "server", k.serverPriv, k.keyring)
	if err != nil {
		t.Fatal(err)
	}
	res := Verify(bytes.NewReader(encodeEntry(t, e)), k.keyring)
	if !res.Valid || res.Entries != 1 {
		t.Fatalf("verify = %+v", res)
	}
}

func TestVerifyDetectsTamper(t *testing.T) {
	k := newFixtureKeys(t)
	r := signGoverned(t, governedRecord(), k)
	e, err := BuildEntry(1, "", r, "server", k.serverPriv, k.keyring)
	if err != nil {
		t.Fatal(err)
	}
	b := encodeEntry(t, e)
	b = bytes.Replace(b, []byte("Bafflewick"), []byte("Tamperedxx"), 1)
	res := Verify(bytes.NewReader(b), k.keyring)
	if res.Valid {
		t.Fatal("tampered ledger verified")
	}
}

func TestVerifyDetectsBrokenChain(t *testing.T) {
	k := newFixtureKeys(t)
	r1 := signGoverned(t, governedRecord(), k)
	e1, err := BuildEntry(1, "", r1, "server", k.serverPriv, k.keyring)
	if err != nil {
		t.Fatal(err)
	}
	r2 := governedRecord()
	r2.RecordID = "REC-002"
	r2.Data = json.RawMessage(`{"title":"Second","severity":"medium"}`)
	r2 = signGoverned(t, r2, k)
	e2, err := BuildEntry(2, "not-the-tail", r2, "server", k.serverPriv, k.keyring)
	if err != nil {
		t.Fatal(err)
	}
	var stream bytes.Buffer
	stream.Write(encodeEntry(t, e1))
	stream.Write(encodeEntry(t, e2))
	res := Verify(&stream, k.keyring)
	if res.Valid || !containsError(res.Errors, "previous hash mismatch") {
		t.Fatalf("expected chain failure: %+v", res)
	}
}

func TestRecordCannotCarryGovernedDecision(t *testing.T) {
	r := governedRecord()
	r.WriteSemantic = "record"
	r.Status = "recorded"
	if err := ValidateRecord(r); err == nil {
		t.Fatal("record semantic accepted confirmer fields")
	}
}

func TestSelfApprovalRequiresExplicitReason(t *testing.T) {
	r := governedRecord()
	r.Confirmer = r.Maker
	r.SelfApproval = true
	r.SelfApprovalReason = ""
	if err := ValidateRecord(r); err == nil {
		t.Fatal("self-approval without reason accepted")
	}
}

func TestCanonicalDataMakesWhitespaceIrrelevant(t *testing.T) {
	a := governedRecord()
	b := governedRecord()
	a.Data = json.RawMessage(`{"b":2,"a":1}`)
	b.Data = json.RawMessage("{\n  \"a\": 1, \"b\": 2\n}")
	da, err := RecordDigest(a)
	if err != nil {
		t.Fatal(err)
	}
	db, err := RecordDigest(b)
	if err != nil {
		t.Fatal(err)
	}
	if da != db {
		t.Fatal("equivalent JSON produced different digest")
	}
}

func containsError(xs []string, want string) bool {
	for _, x := range xs {
		if strings.Contains(x, want) {
			return true
		}
	}
	return false
}
