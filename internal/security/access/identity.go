package access

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// RecordIdentity is the governed record that a person exists, who they are,
// whether they are external, their account state and their account expiry.
// It carries no contact address: personal routes stay out of the ledger.
const RecordIdentity = "security.identity"

// RecordCredentialReset is the governed request to issue a one-time enrolment
// code to a person. A different Custodian confirms it and only the confirmer
// is shown the code, so no single Custodian can take over an account.
const RecordCredentialReset = "security.credential_reset"

// CredentialReset is the payload of a security.credential_reset proposal.
type CredentialReset struct {
	HumanID string `json:"human_id"`
	Reason  string `json:"reason"`
}

// Account states. Suspended is reversible; disabled is final so a human ID is
// never reused for a different person.
const (
	IdentityActive    = "active"
	IdentitySuspended = "suspended"
	IdentityDisabled  = "disabled"
)

// MaxExternalAccountLifetime bounds how far ahead an external person's account
// expiry may be set (U-18). Renewal is a new governed change.
const MaxExternalAccountLifetime = 30 * 24 * time.Hour

const maxIdentityText = 120

var (
	ErrIdentityDisabled      = errors.New("disabled identities are final; record a new identity instead")
	ErrExternalNeedsExpiry   = errors.New("an external person needs an account expiry")
	ErrAccountExpiryTooLong  = errors.New("an external account expiry may be at most 30 days ahead")
	ErrAccountExpiryInPast   = errors.New("account expiry must be in the future")
	ErrIdentityNotUsable     = errors.New("that person's account is not active")
	ErrIdentityRecordMissing = errors.New("record the person's identity before granting a role")
)

// IdentityChange is the payload of a security.identity proposal.
type IdentityChange struct {
	HumanID          string     `json:"human_id"`
	DisplayName      string     `json:"display_name"`
	Organisation     string     `json:"organisation,omitempty"`
	External         bool       `json:"external"`
	Status           string     `json:"status"`
	AccountExpiresAt *time.Time `json:"account_expires_at,omitempty"`
	Reason           string     `json:"reason,omitempty"`
}

// IdentityState is the current projected identity of one person. Found=false
// means no identity record exists.
type IdentityState struct {
	Found            bool
	DisplayName      string
	Organisation     string
	External         bool
	Status           string
	AccountExpiresAt *time.Time
}

// Usable is the single account rule (account_usable). It is evaluated where
// a person authenticates and where a session becomes a principal.
//
// A person with no identity record is usable only as a legacy account created
// before identity records existed (legacy=true: they already hold role
// eligibility or a Custodian designation). New eligibility always requires an
// active identity record, so the legacy population cannot grow.
func Usable(state IdentityState, legacy bool, now time.Time) bool {
	if !state.Found {
		return legacy
	}
	if state.Status != IdentityActive {
		return false
	}
	return state.AccountExpiresAt == nil || state.AccountExpiresAt.After(now)
}

// ValidateIdentityTransition checks a proposed identity change against the
// person's current identity at time now. It is called when proposing and
// again when confirming, because state may change in between.
func ValidateIdentityTransition(change IdentityChange, current IdentityState, now time.Time) error {
	if err := validateIdentityPayload(change); err != nil {
		return err
	}
	if current.Found && current.Status == IdentityDisabled {
		return ErrIdentityDisabled
	}
	if change.Status == IdentityDisabled {
		return nil
	}
	if change.AccountExpiresAt != nil && !change.AccountExpiresAt.After(now) {
		return ErrAccountExpiryInPast
	}
	if change.External {
		if change.AccountExpiresAt == nil {
			return ErrExternalNeedsExpiry
		}
		if change.AccountExpiresAt.After(now.Add(MaxExternalAccountLifetime)) {
			return ErrAccountExpiryTooLong
		}
	}
	return nil
}

func validateIdentityPayload(change IdentityChange) error {
	if strings.TrimSpace(change.HumanID) == "" {
		return errors.New("person ID is required")
	}
	name := strings.TrimSpace(change.DisplayName)
	if name == "" || utf8.RuneCountInString(name) > maxIdentityText {
		return errors.New("a display name of up to 120 characters is required")
	}
	if utf8.RuneCountInString(change.Organisation) > maxIdentityText {
		return errors.New("organisation may be at most 120 characters")
	}
	switch change.Status {
	case IdentityActive, IdentitySuspended, IdentityDisabled:
	default:
		return errors.New("status must be active, suspended or disabled")
	}
	if change.AccountExpiresAt != nil && change.AccountExpiresAt.IsZero() {
		return errors.New("account expiry cannot be zero")
	}
	if change.Status != IdentityActive && strings.TrimSpace(change.Reason) == "" {
		return errors.New("a reason is required to suspend or disable a person")
	}
	return nil
}
