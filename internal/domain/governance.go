package domain

import (
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

// ProposeGoverned exposes the common governed-record envelope to security and
// other application subsystems without exposing a generic operator-facing API.
// Semantic callers must validate their own payload before calling it.
func ProposeGoverned(scope Scope, recordType, recordID, maker string, now time.Time, payload any) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	return proposedRecord(scope, recordType, recordID, maker, now, payload)
}

// ProposeSystemGoverned is the envelope for organisation-level governed
// changes that belong to no incident (output permissions, organisation
// configuration). They are SYSTEM-compartment proposals decided through the
// same maker-checker as everything else.
func ProposeSystemGoverned(recordType, recordID, maker string, now time.Time, payload any) (ledger.Record, error) {
	return proposedRecord(Scope{Mode: "system", Compartment: "SYSTEM"}, recordType, recordID, maker, now, payload)
}
