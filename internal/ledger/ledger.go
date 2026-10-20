package ledger

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

const MaxEntryBytes = 4 << 20

type Signature struct {
	KeyID         string `json:"key_id,omitempty"`
	Value         string `json:"value,omitempty"`
	MissingReason string `json:"missing_reason,omitempty"`
}

// Record is the signed semantic envelope used by the portable ledger format.
// Business meaning stays in typed application commands; this is not a generic
// public CRUD API.
type Record struct {
	SchemaVersion      int             `json:"schema_version"`
	Timestamp          time.Time       `json:"timestamp"`
	TimeStatus         string          `json:"time_status"`
	IncidentID         string          `json:"incident_id,omitempty"`
	IncidentMode       string          `json:"incident_mode"`
	WriteSemantic      string          `json:"write_semantic"`
	RecordType         string          `json:"record_type"`
	RecordID           string          `json:"record_id"`
	Status             string          `json:"status"`
	Maker              string          `json:"maker"`
	Confirmer          string          `json:"confirmer,omitempty"`
	SelfApproval       bool            `json:"self_approval,omitempty"`
	SelfApprovalReason string          `json:"self_approval_reason,omitempty"`
	Compartment        string          `json:"compartment"`
	Data               json.RawMessage `json:"data"`
	MakerSignature     Signature       `json:"maker_signature"`
	ConfirmerSignature Signature       `json:"confirmer_signature,omitempty"`
}

type recordCore struct {
	SchemaVersion      int             `json:"schema_version"`
	Timestamp          time.Time       `json:"timestamp"`
	TimeStatus         string          `json:"time_status"`
	IncidentID         string          `json:"incident_id,omitempty"`
	IncidentMode       string          `json:"incident_mode"`
	WriteSemantic      string          `json:"write_semantic"`
	RecordType         string          `json:"record_type"`
	RecordID           string          `json:"record_id"`
	Status             string          `json:"status"`
	Maker              string          `json:"maker"`
	Confirmer          string          `json:"confirmer,omitempty"`
	SelfApproval       bool            `json:"self_approval,omitempty"`
	SelfApprovalReason string          `json:"self_approval_reason,omitempty"`
	Compartment        string          `json:"compartment"`
	Data               json.RawMessage `json:"data"`
}

type chainCore struct {
	Seq      uint64     `json:"seq"`
	PrevHash string     `json:"prev_hash"`
	Record   recordCore `json:"record"`
}

type Entry struct {
	Seq             uint64 `json:"seq"`
	PrevHash        string `json:"prev_hash"`
	Record          Record `json:"record"`
	Hash            string `json:"hash"`
	ServerKeyID     string `json:"server_key_id"`
	ServerSignature string `json:"server_signature"`
}

type Keyring map[string]string

type VerifyResult struct {
	Valid   bool     `json:"valid"`
	Entries int      `json:"entries"`
	Errors  []string `json:"errors,omitempty"`
}

func (r *VerifyResult) add(s string) {
	r.Valid = false
	r.Errors = append(r.Errors, s)
}

func ValidateRecord(r Record) error {
	if r.SchemaVersion <= 0 {
		return errors.New("schema_version must be positive")
	}
	if r.Timestamp.IsZero() || r.RecordID == "" || r.Maker == "" {
		return errors.New("timestamp, record_id and maker are required")
	}
	if r.TimeStatus != "trusted" && r.TimeStatus != "drifting" && r.TimeStatus != "untrusted" {
		return errors.New("invalid time_status")
	}
	switch r.IncidentMode {
	case "live", "exercise":
		if r.IncidentID == "" {
			return errors.New("live/exercise record requires incident_id")
		}
	case "system":
		// System/setup records may legitimately exist before any incident exists.
	default:
		return errors.New("invalid incident_mode")
	}
	if r.WriteSemantic != "record" && r.WriteSemantic != "governed" {
		return errors.New("invalid write_semantic")
	}
	if r.RecordType == "" || r.Compartment == "" {
		return errors.New("record_type and compartment are required")
	}
	if len(r.Data) == 0 || !json.Valid(r.Data) {
		return errors.New("data must be valid JSON")
	}
	if r.WriteSemantic == "record" {
		if r.Status != "recorded" {
			return errors.New("record semantic requires recorded status")
		}
		if hasConfirmer(r) || r.SelfApproval || r.SelfApprovalReason != "" {
			return errors.New("record semantic cannot carry confirmer or self-approval")
		}
		return nil
	}

	if r.Status != "proposed" && r.Status != "confirmed" && r.Status != "rejected" {
		return errors.New("governed semantic requires proposed, confirmed or rejected status")
	}
	if r.Status == "proposed" {
		if hasConfirmer(r) || r.SelfApproval || r.SelfApprovalReason != "" {
			return errors.New("proposed record cannot carry confirmer or self-approval")
		}
		return nil
	}
	if r.Confirmer == "" {
		return errors.New("decided governed record requires confirmer")
	}
	if r.Maker == r.Confirmer {
		if !r.SelfApproval || r.SelfApprovalReason == "" {
			return errors.New("self-confirmation requires self_approval and reason")
		}
	} else if r.SelfApproval || r.SelfApprovalReason != "" {
		return errors.New("self-approval fields only valid when maker equals confirmer")
	}
	return nil
}

func hasConfirmer(r Record) bool {
	return r.Confirmer != "" || r.ConfirmerSignature.KeyID != "" || r.ConfirmerSignature.Value != "" || r.ConfirmerSignature.MissingReason != ""
}

func RecordDigest(r Record) ([32]byte, error) {
	core, err := unsignedRecord(r)
	if err != nil {
		return [32]byte{}, err
	}
	b, err := json.Marshal(core)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(b), nil
}

func unsignedRecord(r Record) (recordCore, error) {
	canonical, err := canonicalJSON(r.Data)
	if err != nil {
		return recordCore{}, err
	}
	return recordCore{
		SchemaVersion:      r.SchemaVersion,
		Timestamp:          r.Timestamp.UTC(),
		TimeStatus:         r.TimeStatus,
		IncidentID:         r.IncidentID,
		IncidentMode:       r.IncidentMode,
		WriteSemantic:      r.WriteSemantic,
		RecordType:         r.RecordType,
		RecordID:           r.RecordID,
		Status:             r.Status,
		Maker:              r.Maker,
		Confirmer:          r.Confirmer,
		SelfApproval:       r.SelfApproval,
		SelfApprovalReason: r.SelfApprovalReason,
		Compartment:        r.Compartment,
		Data:               canonical,
	}, nil
}

func canonicalJSON(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, errors.New("empty JSON")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("multiple JSON values")
		}
		return nil, err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func SignRecord(r Record, priv ed25519.PrivateKey) (string, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return "", errors.New("invalid Ed25519 private key")
	}
	d, err := RecordDigest(r)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ed25519.Sign(priv, d[:])), nil
}

func BuildEntry(seq uint64, prevHash string, r Record, serverKeyID string, serverPriv ed25519.PrivateKey, keys Keyring) (Entry, error) {
	if seq == 0 {
		return Entry{}, errors.New("sequence must start at 1")
	}
	if len(serverPriv) != ed25519.PrivateKeySize || serverKeyID == "" {
		return Entry{}, errors.New("valid server signing key is required")
	}
	if err := ValidateRecord(r); err != nil {
		return Entry{}, err
	}
	canonical, err := canonicalJSON(r.Data)
	if err != nil {
		return Entry{}, err
	}
	r.Data = canonical
	r.Timestamp = r.Timestamp.UTC()
	digest, err := RecordDigest(r)
	if err != nil {
		return Entry{}, err
	}
	if err := verifyHumanSignature(r.MakerSignature, digest, keys); err != nil {
		return Entry{}, fmt.Errorf("maker signature: %w", err)
	}
	if r.WriteSemantic == "governed" && r.Status != "proposed" {
		if err := verifyHumanSignature(r.ConfirmerSignature, digest, keys); err != nil {
			return Entry{}, fmt.Errorf("confirmer signature: %w", err)
		}
	}
	serverPub, err := publicKey(keys, serverKeyID)
	if err != nil {
		return Entry{}, fmt.Errorf("server public key: %w", err)
	}
	if !bytes.Equal(serverPub, serverPriv.Public().(ed25519.PublicKey)) {
		return Entry{}, errors.New("server public key does not match private key")
	}
	core, err := unsignedRecord(r)
	if err != nil {
		return Entry{}, err
	}
	cc := chainCore{Seq: seq, PrevHash: prevHash, Record: core}
	h := entryHash(cc, r.MakerSignature, r.ConfirmerSignature)
	return Entry{
		Seq:             seq,
		PrevHash:        prevHash,
		Record:          r,
		Hash:            hex.EncodeToString(h[:]),
		ServerKeyID:     serverKeyID,
		ServerSignature: base64.StdEncoding.EncodeToString(ed25519.Sign(serverPriv, h[:])),
	}, nil
}

func Verify(r io.Reader, keys Keyring) VerifyResult {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), MaxEntryBytes+1024)
	res := VerifyResult{Valid: true}
	var expectedSeq uint64
	var prev string
	for sc.Scan() {
		res.Entries++
		if len(sc.Bytes()) > MaxEntryBytes {
			res.add(fmt.Sprintf("entry %d exceeds maximum size", res.Entries))
			continue
		}
		var e Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			res.add(fmt.Sprintf("entry %d malformed: %v", res.Entries, err))
			continue
		}
		expectedSeq++
		if e.Seq != expectedSeq {
			res.add(fmt.Sprintf("entry %d sequence mismatch", res.Entries))
		}
		if e.PrevHash != prev {
			res.add(fmt.Sprintf("entry %d previous hash mismatch", res.Entries))
		}
		if err := ValidateRecord(e.Record); err != nil {
			res.add(fmt.Sprintf("entry %d record: %v", res.Entries, err))
		}
		digest, err := RecordDigest(e.Record)
		if err != nil {
			res.add(fmt.Sprintf("entry %d record digest: %v", res.Entries, err))
		} else {
			if err := verifyHumanSignature(e.Record.MakerSignature, digest, keys); err != nil {
				res.add(fmt.Sprintf("entry %d maker signature: %v", res.Entries, err))
			}
			if e.Record.WriteSemantic == "governed" && e.Record.Status != "proposed" {
				if err := verifyHumanSignature(e.Record.ConfirmerSignature, digest, keys); err != nil {
					res.add(fmt.Sprintf("entry %d confirmer signature: %v", res.Entries, err))
				}
			}
		}
		core, err := unsignedRecord(e.Record)
		if err == nil {
			cc := chainCore{Seq: e.Seq, PrevHash: e.PrevHash, Record: core}
			h := entryHash(cc, e.Record.MakerSignature, e.Record.ConfirmerSignature)
			if e.Hash != hex.EncodeToString(h[:]) {
				res.add(fmt.Sprintf("entry %d hash mismatch", res.Entries))
			}
			pub, pubErr := publicKey(keys, e.ServerKeyID)
			sig, sigErr := base64.StdEncoding.DecodeString(e.ServerSignature)
			if pubErr != nil || sigErr != nil || !ed25519.Verify(pub, h[:], sig) {
				res.add(fmt.Sprintf("entry %d server signature invalid", res.Entries))
			}
		}
		prev = e.Hash
	}
	if err := sc.Err(); err != nil {
		res.add(err.Error())
	}
	return res
}

func verifyHumanSignature(s Signature, digest [32]byte, keys Keyring) error {
	if s.Value == "" {
		if s.MissingReason == "" {
			return errors.New("signature or missing_reason required")
		}
		return nil
	}
	if s.MissingReason != "" {
		return errors.New("signature and missing_reason are mutually exclusive")
	}
	pub, err := publicKey(keys, s.KeyID)
	if err != nil {
		return err
	}
	sig, err := base64.StdEncoding.DecodeString(s.Value)
	if err != nil || !ed25519.Verify(pub, digest[:], sig) {
		return errors.New("invalid signature")
	}
	return nil
}

func publicKey(keys Keyring, id string) (ed25519.PublicKey, error) {
	raw, ok := keys[id]
	if !ok || id == "" {
		return nil, errors.New("key not found: " + id)
	}
	b, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(b) != ed25519.PublicKeySize {
		return nil, errors.New("invalid public key")
	}
	return ed25519.PublicKey(b), nil
}

func entryHash(c chainCore, maker Signature, confirmer Signature) [32]byte {
	b, _ := json.Marshal(struct {
		Core      chainCore `json:"core"`
		Maker     Signature `json:"maker_signature"`
		Confirmer Signature `json:"confirmer_signature"`
	}{Core: c, Maker: maker, Confirmer: confirmer})
	return sha256.Sum256(b)
}
