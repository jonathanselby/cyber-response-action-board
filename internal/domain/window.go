package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

// SinglePersonWindow is a time-bounded, explicitly approved exception to dual
// control (PRD single-person windows, AC-07, UAT 2 item 4). Opening, changing
// or closing a window is itself dual-controlled.
type SinglePersonWindow struct {
	WindowID string    `json:"window_id"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
	Humans   []string  `json:"humans"`
	Kinds    []string  `json:"kinds"`
	Reason   string    `json:"reason"`
	Active   bool      `json:"active"`
}

// MaxWindowLength is the longest a window may stay open.
const MaxWindowLength = 24 * time.Hour

// Decision kinds a window may cover. Access, isolate mode, Custodian
// authority, organisation configuration (including windows), closure and
// retention can never be self-approved.
var WindowCoverableKinds = map[string]string{
	TypeDecision:                  "Decisions",
	TypeAction:                    "Actions",
	TypeOperationalRoleAssignment: "Operational roles",
	TypeSeverityChange:            "Severity changes",
	TypeCommunicationIssue:        "Communication issues",
	TypeCommunicationCadence:      "Report cadence",
	TypeObligationTrigger:         "Obligation clocks",
	TypeObligationChange:          "Obligation changes",
	TypeStandDown:                 "Stand-down",
}

func ValidateSinglePersonWindowShape(w SinglePersonWindow) error {
	if !templateIDPattern.MatchString(w.WindowID) {
		return errors.New("window identifier must be 2 to 60 lower-case letters, digits or hyphens")
	}
	if strings.TrimSpace(w.Reason) == "" {
		return errors.New("a window needs a reason")
	}
	if !w.Active {
		return nil
	}
	if w.StartsAt.IsZero() || w.EndsAt.IsZero() || !w.EndsAt.After(w.StartsAt) {
		return errors.New("a window needs a start and an end after it")
	}
	if w.EndsAt.Sub(w.StartsAt) > MaxWindowLength {
		return errors.New("a window may last at most 24 hours")
	}
	if len(w.Humans) == 0 {
		return errors.New("a window must name who may use it")
	}
	seen := map[string]bool{}
	for _, h := range w.Humans {
		if strings.TrimSpace(h) == "" || seen[h] {
			return errors.New("window people must be distinct and named")
		}
		seen[h] = true
	}
	if len(w.Kinds) == 0 {
		return errors.New("a window must name the decisions it covers")
	}
	for _, k := range w.Kinds {
		if _, ok := WindowCoverableKinds[k]; !ok {
			return fmt.Errorf("%s can never be self-approved", k)
		}
	}
	return nil
}

// Covers reports whether the window permits human to self-approve a proposal
// of recordType at time at.
func (w SinglePersonWindow) Covers(human, recordType string, at time.Time) bool {
	if !w.Active || at.Before(w.StartsAt) || !at.Before(w.EndsAt) {
		return false
	}
	named := false
	for _, h := range w.Humans {
		if h == human {
			named = true
		}
	}
	if !named {
		return false
	}
	for _, k := range w.Kinds {
		if k == recordType {
			return true
		}
	}
	return false
}

func ProposeSinglePersonWindow(recordID, maker string, now time.Time, w SinglePersonWindow) (ledger.Record, error) {
	if err := ValidateSinglePersonWindowShape(w); err != nil {
		return ledger.Record{}, err
	}
	if w.Active && w.StartsAt.Before(now.Add(-5*time.Minute)) {
		return ledger.Record{}, errors.New("a window cannot start in the past")
	}
	w.StartsAt, w.EndsAt = w.StartsAt.UTC(), w.EndsAt.UTC()
	return ProposeSystemGoverned(TypeSinglePersonWindow, recordID, maker, now, w)
}

// DecideSelf records a maker confirming or rejecting their own proposal
// inside a permitted single-person window. The caller must already have
// checked the window; this only builds the flagged record (AC-07).
func DecideSelf(proposal ledger.Record, expectedVersion, confirmer, outcome, reason string, now time.Time) (ledger.Record, error) {
	if proposal.WriteSemantic != "governed" || proposal.Status != "proposed" {
		return ledger.Record{}, errors.New("decision requires a proposed governed record")
	}
	if confirmer == "" || confirmer != proposal.Maker {
		return ledger.Record{}, errors.New("self-approval is only for the maker's own proposal")
	}
	if strings.TrimSpace(reason) == "" {
		return ledger.Record{}, errors.New("self-approval needs a reason")
	}
	if outcome != "confirmed" && outcome != "rejected" {
		return ledger.Record{}, errors.New("decision outcome must be confirmed or rejected")
	}
	data, err := DecodeGovernedData(proposal.Data)
	if err != nil {
		return ledger.Record{}, err
	}
	if expectedVersion == "" || data.Version != expectedVersion {
		return ledger.Record{}, errors.New("proposal version changed")
	}
	decided := proposal
	decided.Timestamp = now.UTC()
	decided.Status = outcome
	decided.Confirmer = confirmer
	decided.SelfApproval = true
	decided.SelfApprovalReason = strings.TrimSpace(reason)
	decided.MakerSignature = ledger.Signature{MissingReason: "maker authenticated on immutable proposal entry"}
	decided.ConfirmerSignature = ledger.Signature{MissingReason: "confirmer authenticated by CRAB session inside a single-person window"}
	if err := ledger.ValidateRecord(decided); err != nil {
		return ledger.Record{}, err
	}
	return decided, nil
}
