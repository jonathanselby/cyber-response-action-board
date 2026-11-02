package app

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/access"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authn"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

// EnrolmentCodeLifetime bounds how long a one-time enrolment code works.
const EnrolmentCodeLifetime = 24 * time.Hour

// ErrUseCredentialResetPath is returned when a credential reset is confirmed
// through the generic decision path instead of ConfirmCredentialReset.
var ErrUseCredentialResetPath = errors.New("confirm a credential reset from the credential reset action")

// CredentialBackend stores credential lifecycle state and sessions.
type CredentialBackend interface {
	GetCredential(context.Context, string) (authn.Credential, bool, error)
	PutPassword(context.Context, authn.PasswordRecord) error
	PutEnrolmentCode(context.Context, authn.PasswordRecord, time.Time) error
	GetTOTP(context.Context, string) (authn.TOTPRecord, bool, bool, error)
	PutPendingTOTP(context.Context, authn.TOTPRecord) error
	ConfirmTOTP(context.Context, string) error
	DeleteTOTP(context.Context, string) error
	DeleteSessionsFor(context.Context, string) error
	SweepExpiredSessions(context.Context, time.Time, int) error
}

// usable applies the one account rule to a person at time now.
func (s AccessService) usable(ctx context.Context, humanID string, now time.Time) (bool, error) {
	state, legacy, err := s.Backend.IdentityState(ctx, humanID)
	if err != nil {
		return false, err
	}
	return access.Usable(state, legacy, now), nil
}

func (s AccessService) requireUsableCustodian(ctx context.Context, principal authz.Principal, now time.Time) error {
	if err := s.ready(); err != nil {
		return err
	}
	if d := authz.Authorise(principal, authz.ActionAdminIdentityMetadata, authz.ObjectScope{SystemMetadata: true}); !d.Allowed {
		return ErrAuthorisationDenied
	}
	ok, err := s.usable(ctx, principal.HumanID, now)
	if err != nil {
		return err
	}
	if !ok {
		return ErrAuthorisationDenied
	}
	return nil
}

// ProposeIdentity lets a Custodian propose creating or changing a person's
// identity and account state. A different Custodian confirms it.
func (s AccessService) ProposeIdentity(ctx context.Context, principal authz.Principal, recordID string, change access.IdentityChange, now time.Time) (ledger.Record, error) {
	if err := s.requireUsableCustodian(ctx, principal, now); err != nil {
		return ledger.Record{}, err
	}
	change.HumanID = strings.TrimSpace(change.HumanID)
	change.DisplayName = strings.TrimSpace(change.DisplayName)
	change.Organisation = strings.TrimSpace(change.Organisation)
	change.Reason = strings.TrimSpace(change.Reason)
	if change.AccountExpiresAt != nil {
		expiry := change.AccountExpiresAt.UTC()
		change.AccountExpiresAt = &expiry
	}
	current, _, err := s.Backend.IdentityState(ctx, change.HumanID)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := access.ValidateIdentityTransition(change, current, now); err != nil {
		return ledger.Record{}, err
	}
	return s.proposeSystem(ctx, principal.HumanID, recordID, access.RecordIdentity, change, now)
}

// ProposeRoleEligibility lets a Custodian propose granting, changing or
// withdrawing a person's Major/Minor/Observer eligibility.
func (s AccessService) ProposeRoleEligibility(ctx context.Context, principal authz.Principal, recordID, humanID string, role authz.Role, active bool, now time.Time) (ledger.Record, error) {
	if err := s.requireUsableCustodian(ctx, principal, now); err != nil {
		return ledger.Record{}, err
	}
	humanID = strings.TrimSpace(humanID)
	if active {
		// The identity may be proposed in the same step (a new person); it must
		// be confirmed before this eligibility can be confirmed.
		if err := s.requireIdentityForGrant(ctx, humanID, now); err != nil {
			pending, perr := s.Backend.HasPendingIdentity(ctx, humanID)
			if perr != nil {
				return ledger.Record{}, perr
			}
			if !pending {
				return ledger.Record{}, err
			}
		}
	}
	payload := map[string]any{"human_id": humanID, "role": role, "active": active}
	return s.proposeSystem(ctx, principal.HumanID, recordID, access.RecordRoleEligibility, payload, now)
}

// ProposeCredentialReset lets a Custodian ask for a one-time enrolment code to
// be issued to a person. Only the confirming Custodian is shown the code.
func (s AccessService) ProposeCredentialReset(ctx context.Context, principal authz.Principal, recordID, humanID, reason string, now time.Time) (ledger.Record, error) {
	if err := s.requireUsableCustodian(ctx, principal, now); err != nil {
		return ledger.Record{}, err
	}
	humanID = strings.TrimSpace(humanID)
	ok, err := s.usable(ctx, humanID, now)
	if err != nil {
		return ledger.Record{}, err
	}
	if !ok {
		return ledger.Record{}, access.ErrIdentityNotUsable
	}
	payload := access.CredentialReset{HumanID: humanID, Reason: strings.TrimSpace(reason)}
	return s.proposeSystem(ctx, principal.HumanID, recordID, access.RecordCredentialReset, payload, now)
}

// ConfirmCredentialReset is the only way to confirm a credential reset. The
// decision is committed first; then the person's TOTP enrolment and sessions
// are removed and a fresh enrolment code is stored. The code is returned once,
// to the confirmer only. If the process stops after the decision commits, the
// reset is visible in history and a new reset can be proposed.
// IssuedCode is a one-time enrolment code and who it is for.
type IssuedCode struct {
	HumanID   string
	Code      string
	ExpiresAt time.Time
}

func (s AccessService) ConfirmCredentialReset(ctx context.Context, principal authz.Principal, credentials CredentialBackend, recordID, expectedVersion string, now time.Time) (IssuedCode, error) {
	if credentials == nil {
		return IssuedCode{}, errors.New("credential backend is required")
	}
	_, change, err := s.decide(ctx, principal, "", access.RecordCredentialReset, recordID, expectedVersion, "confirmed", now, true)
	if err != nil {
		return IssuedCode{}, err
	}
	code, err := newEnrolmentCode()
	if err != nil {
		return IssuedCode{}, err
	}
	record, err := authn.NewPasswordRecord(change.HumanID, normaliseCode(code), authn.MinPBKDF2Iterations)
	if err != nil {
		return IssuedCode{}, err
	}
	expires := now.UTC().Add(EnrolmentCodeLifetime)
	if err := credentials.DeleteTOTP(ctx, change.HumanID); err != nil {
		return IssuedCode{}, err
	}
	if err := credentials.DeleteSessionsFor(ctx, change.HumanID); err != nil {
		return IssuedCode{}, err
	}
	if err := credentials.PutEnrolmentCode(ctx, record, expires); err != nil {
		return IssuedCode{}, err
	}
	return IssuedCode{HumanID: change.HumanID, Code: code, ExpiresAt: expires}, nil
}

func (s AccessService) requireIdentityForGrant(ctx context.Context, humanID string, now time.Time) error {
	state, _, err := s.Backend.IdentityState(ctx, humanID)
	if err != nil {
		return err
	}
	if !state.Found {
		return access.ErrIdentityRecordMissing
	}
	if !access.Usable(state, false, now) {
		return access.ErrIdentityNotUsable
	}
	return nil
}

func (s AccessService) proposeSystem(ctx context.Context, maker, recordID, recordType string, payload any, now time.Time) (ledger.Record, error) {
	if err := s.ready(); err != nil {
		return ledger.Record{}, err
	}
	record, err := domain.ProposeSystemGoverned(recordType, recordID, maker, now, payload)
	if err != nil {
		return ledger.Record{}, err
	}
	data, err := domain.DecodeGovernedData(record.Data)
	if err != nil {
		return ledger.Record{}, err
	}
	if _, recognised, err := access.Decode(recordType, data.Payload); err != nil || !recognised {
		if err != nil {
			return ledger.Record{}, err
		}
		return ledger.Record{}, errors.New("unsupported access change")
	}
	if err := s.Backend.AppendAccess(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

// newEnrolmentCode returns a 20-character code from a reduced alphabet that is
// easy to read aloud: four groups of five.
func newEnrolmentCode() (string, error) {
	raw := make([]byte, 15)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	code := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)[:20]
	return code[0:5] + "-" + code[5:10] + "-" + code[10:15] + "-" + code[15:20], nil
}
