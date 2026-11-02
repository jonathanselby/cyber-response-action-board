package access

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

// RecordIntakeKey is the governed record of a machine intake key (contract
// addendum A4). An intake key is a machine credential, not a person: it is
// bound to one accountable named human and authorises only first-report
// creation. Its secret never enters the ledger.
const RecordIntakeKey = "security.intake_key"

// Intake key actions.
const (
	IntakeKeyIssue  = "issue"
	IntakeKeyRotate = "rotate"
	IntakeKeyRevoke = "revoke"
)

// Intake key states.
const (
	IntakeKeyActive  = "active"
	IntakeKeyRevoked = "revoked"
)

// MaxIntakeKeyLifetime bounds how far ahead an intake key's expiry may be set.
// Renewal is a governed rotation.
const MaxIntakeKeyLifetime = 365 * 24 * time.Hour

// MaxIntakeKeyOverlap bounds how long the previous secret keeps working after
// a rotation is confirmed.
const MaxIntakeKeyOverlap = 24 * time.Hour

var intakeKeyName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$`)

var (
	ErrIntakeKeyName         = errors.New("an intake key name is 3 to 40 lower-case letters, digits or hyphens")
	ErrIntakeKeyExists       = errors.New("an intake key with that name already exists; names are never reused")
	ErrIntakeKeyNotActive    = errors.New("that intake key is not active")
	ErrIntakeKeyNeedsExpiry  = errors.New("an intake key needs an expiry in the future")
	ErrIntakeKeyExpiryTooFar = errors.New("an intake key expiry may be at most 365 days ahead")
	ErrIntakeKeyOverlap      = errors.New("a rotation overlap is between 0 and 24 hours")
	ErrIntakeKeyReason       = errors.New("a reason is required")
)

// IntakeKeyChange is the payload of a security.intake_key proposal.
// AccountableHuman is set on issue only; rotation and revocation keep the
// accountable human the key was issued to.
type IntakeKeyChange struct {
	KeyName          string     `json:"key_name"`
	Action           string     `json:"action"`
	AccountableHuman string     `json:"accountable_human,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	OverlapMinutes   int        `json:"overlap_minutes,omitempty"`
	Reason           string     `json:"reason"`
}

// IntakeKeyState is the current projected state of one intake key.
type IntakeKeyState struct {
	Found            bool
	KeyName          string
	AccountableHuman string
	Status           string
	ExpiresAt        time.Time
	IssuedAt         time.Time
	RotatedAt        *time.Time
}

// IntakeKeyUsable is the key half of the intake rule: the key exists, is not
// revoked and has not expired. The accountable human must also be usable.
func IntakeKeyUsable(state IntakeKeyState, now time.Time) bool {
	return state.Found && state.Status == IntakeKeyActive && state.ExpiresAt.After(now)
}

// ValidateIntakeKeyChange checks a proposed change against the key's current
// state. It is called when proposing and again when confirming.
func ValidateIntakeKeyChange(change IntakeKeyChange, current IntakeKeyState, now time.Time) error {
	if err := validateIntakeKeyPayload(change); err != nil {
		return err
	}
	switch change.Action {
	case IntakeKeyIssue:
		if current.Found {
			return ErrIntakeKeyExists
		}
	case IntakeKeyRotate, IntakeKeyRevoke:
		if !IntakeKeyUsable(current, now) {
			return ErrIntakeKeyNotActive
		}
	}
	if change.ExpiresAt != nil {
		if !change.ExpiresAt.After(now) {
			return ErrIntakeKeyNeedsExpiry
		}
		if change.ExpiresAt.After(now.Add(MaxIntakeKeyLifetime)) {
			return ErrIntakeKeyExpiryTooFar
		}
	}
	return nil
}

func validateIntakeKeyPayload(change IntakeKeyChange) error {
	if !intakeKeyName.MatchString(change.KeyName) {
		return ErrIntakeKeyName
	}
	if strings.TrimSpace(change.Reason) == "" {
		return ErrIntakeKeyReason
	}
	switch change.Action {
	case IntakeKeyIssue:
		if strings.TrimSpace(change.AccountableHuman) == "" {
			return errors.New("an intake key needs an accountable person")
		}
		if change.ExpiresAt == nil {
			return ErrIntakeKeyNeedsExpiry
		}
		if change.OverlapMinutes != 0 {
			return ErrIntakeKeyOverlap
		}
	case IntakeKeyRotate:
		if change.AccountableHuman != "" {
			return errors.New("rotation keeps the accountable person; revoke and issue a new key to change it")
		}
		if change.OverlapMinutes < 0 || time.Duration(change.OverlapMinutes)*time.Minute > MaxIntakeKeyOverlap {
			return ErrIntakeKeyOverlap
		}
	case IntakeKeyRevoke:
		if change.AccountableHuman != "" || change.ExpiresAt != nil || change.OverlapMinutes != 0 {
			return errors.New("a revocation carries only the key name and a reason")
		}
	default:
		return errors.New("intake key action must be issue, rotate or revoke")
	}
	return nil
}
