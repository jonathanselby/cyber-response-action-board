package ceremony

import (
	"crypto/ed25519"
	"errors"

	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

type HumanSigner struct {
	HumanID    string
	KeyID      string
	PrivateKey ed25519.PrivateKey
}

// SignGovernedRecords applies the two named human signatures required before
// confirmed ceremony records can enter the authoritative ledger.
func SignGovernedRecords(records []ledger.Record, maker, confirmer HumanSigner) ([]ledger.Record, error) {
	if maker.HumanID == "" || confirmer.HumanID == "" || maker.HumanID == confirmer.HumanID {
		return nil, errors.New("two distinct named human signers are required")
	}
	if maker.KeyID == "" || confirmer.KeyID == "" || len(maker.PrivateKey) != ed25519.PrivateKeySize || len(confirmer.PrivateKey) != ed25519.PrivateKeySize {
		return nil, errors.New("valid human signing keys are required")
	}
	out := make([]ledger.Record, len(records))
	for i, record := range records {
		if record.WriteSemantic != "governed" || record.Status != "confirmed" {
			return nil, errors.New("ceremony signer accepts confirmed governed records only")
		}
		if record.Maker != maker.HumanID || record.Confirmer != confirmer.HumanID {
			return nil, errors.New("ceremony record signer identities do not match record provenance")
		}
		makerSignature, err := ledger.SignRecord(record, maker.PrivateKey)
		if err != nil {
			return nil, err
		}
		confirmerSignature, err := ledger.SignRecord(record, confirmer.PrivateKey)
		if err != nil {
			return nil, err
		}
		record.MakerSignature = ledger.Signature{KeyID: maker.KeyID, Value: makerSignature}
		record.ConfirmerSignature = ledger.Signature{KeyID: confirmer.KeyID, Value: confirmerSignature}
		out[i] = record
	}
	return out, nil
}
