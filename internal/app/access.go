package app

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/access"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

type AccessBackend interface {
	AppendAccess(context.Context, ledger.Record) error
	PendingAccess(context.Context, string, string, string) (ledger.Record, bool, error)
	IsSponsor(context.Context, string, string) (bool, error)
	HasSponsor(context.Context, string) (bool, error)
	IsController(context.Context, string, string, string) (bool, error)
	IsMajorEligible(context.Context, string) (bool, error)
	IsCustodian(context.Context, string) (bool, error)
	HasRoleEligibility(context.Context, string) (bool, error)
	IncidentExists(context.Context, string) (bool, error)
	HasIncidentMembershipAt(context.Context, string, string, time.Time) (bool, error)
	AdministrationView(context.Context, time.Time) (access.AdministrationView, error)
	IdentityState(context.Context, string) (access.IdentityState, bool, error)
	People(context.Context, time.Time) ([]access.PersonRecord, error)
	HasPendingIdentity(context.Context, string) (bool, error)
	ExternalPeople(context.Context) (map[string]bool, error)
	IntakeKeyState(context.Context, string) (access.IntakeKeyState, error)
	IntakeKeys(context.Context, time.Time) ([]access.IntakeKeyState, map[string]time.Time, error)
}

type AccessService struct{ Backend AccessBackend }

func (s AccessService) ProposeSponsorDesignation(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, payload access.IncidentSponsorDesignation, now time.Time) (ledger.Record, error) {
	if err := s.ready(); err != nil {
		return ledger.Record{}, err
	}
	if !principal.Authenticated || principal.HumanID == "" || principal.Role != authz.RoleMajor || payload.IncidentID != scope.IncidentID {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	exists, err := s.Backend.IncidentExists(ctx, scope.IncidentID)
	if err != nil || !exists {
		if err != nil {
			return ledger.Record{}, err
		}
		return ledger.Record{}, ErrAuthorisationDenied
	}
	hasSponsor, err := s.Backend.HasSponsor(ctx, scope.IncidentID)
	if err != nil {
		return ledger.Record{}, err
	}
	if hasSponsor {
		allowed, err := s.Backend.IsSponsor(ctx, principal.HumanID, scope.IncidentID)
		if err != nil {
			return ledger.Record{}, err
		}
		if !allowed {
			return ledger.Record{}, ErrAuthorisationDenied
		}
	} else {
		eligible, err := s.Backend.IsMajorEligible(ctx, principal.HumanID)
		if err != nil {
			return ledger.Record{}, err
		}
		if !eligible {
			return ledger.Record{}, ErrAuthorisationDenied
		}
	}
	return s.propose(ctx, principal.HumanID, scope, recordID, access.RecordIncidentSponsor, payload, now)
}

func (s AccessService) ProposeControllerDesignation(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, payload access.CompartmentControllerDesignation, now time.Time) (ledger.Record, error) {
	if payload.IncidentID != scope.IncidentID || payload.Compartment != scope.Compartment || scope.Compartment == "GENERAL" {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	if err := s.requireSponsor(ctx, principal, scope.IncidentID); err != nil {
		return ledger.Record{}, err
	}
	return s.propose(ctx, principal.HumanID, scope, recordID, access.RecordCompartmentController, payload, now)
}

func (s AccessService) ProposeIncidentMembership(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, payload access.IncidentMembershipChange, now time.Time) (ledger.Record, error) {
	if payload.IncidentID != scope.IncidentID || scope.Compartment != "GENERAL" {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	if err := s.requireSponsor(ctx, principal, scope.IncidentID); err != nil {
		return ledger.Record{}, err
	}
	if payload.Active {
		expires, err := s.externalAccessEnd(ctx, payload.HumanID, payload.ExpiresAt)
		if err != nil {
			return ledger.Record{}, err
		}
		payload.ExpiresAt = expires
	}
	return s.propose(ctx, principal.HumanID, scope, recordID, access.RecordIncidentMembership, payload, now)
}

func (s AccessService) ProposeCompartmentMembership(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, payload access.CompartmentMembershipChange, now time.Time) (ledger.Record, error) {
	if payload.IncidentID != scope.IncidentID || payload.Compartment != scope.Compartment || scope.Compartment == "GENERAL" {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	if err := s.ready(); err != nil {
		return ledger.Record{}, err
	}
	if !principal.Authenticated || principal.HumanID == "" {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	allowed, err := s.Backend.IsController(ctx, principal.HumanID, scope.IncidentID, scope.Compartment)
	if err != nil {
		return ledger.Record{}, err
	}
	if !allowed {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	if payload.Active {
		expires, err := s.externalAccessEnd(ctx, payload.HumanID, payload.ExpiresAt)
		if err != nil {
			return ledger.Record{}, err
		}
		payload.ExpiresAt = expires
	}
	return s.propose(ctx, principal.HumanID, scope, recordID, access.RecordCompartmentMembership, payload, now)
}

// ProposeOutputPermission lets a Custodian propose granting or withdrawing a
// separate output permission (print, download) for a named person. Another
// Custodian confirms it. Read access never implies output authority.
func (s AccessService) ProposeOutputPermission(ctx context.Context, principal authz.Principal, recordID, humanID string, action authz.Action, active bool, now time.Time) (ledger.Record, error) {
	if err := s.ready(); err != nil {
		return ledger.Record{}, err
	}
	if d := authz.Authorise(principal, authz.ActionAdminIdentityMetadata, authz.ObjectScope{SystemMetadata: true}); !d.Allowed {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	if action != authz.ActionPrint && action != authz.ActionDownload {
		return ledger.Record{}, errors.New("only print and download permissions are administered here")
	}
	if strings.TrimSpace(humanID) == "" {
		return ledger.Record{}, errors.New("person is required")
	}
	eligible, err := s.Backend.HasRoleEligibility(ctx, humanID)
	if err != nil {
		return ledger.Record{}, err
	}
	if !eligible {
		return ledger.Record{}, errors.New("that person has no CRAB role")
	}
	payload := map[string]any{"human_id": strings.TrimSpace(humanID), "action": action, "active": active}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ledger.Record{}, err
	}
	if _, _, err := access.Decode(access.RecordOutputPermission, raw); err != nil {
		return ledger.Record{}, err
	}
	record, err := domain.ProposeSystemGoverned(access.RecordOutputPermission, recordID, principal.HumanID, now, payload)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendAccess(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s AccessService) Decide(ctx context.Context, principal authz.Principal, scopeID, recordType, recordID, expectedVersion, outcome string, now time.Time) (ledger.Record, error) {
	decided, _, err := s.decide(ctx, principal, scopeID, recordType, recordID, expectedVersion, outcome, now, false)
	return decided, err
}

// decide is the one access decision path. Confirming re-checks, at decision
// time, that the confirmer and the maker are both usable accounts, that the
// maker still holds the authority to have proposed the change, and that the
// change is still valid against current state.
func (s AccessService) decide(ctx context.Context, principal authz.Principal, scopeID, recordType, recordID, expectedVersion, outcome string, now time.Time, credentialPath bool) (ledger.Record, access.Change, error) {
	if err := s.requireUsableCustodian(ctx, principal, now); err != nil {
		return ledger.Record{}, access.Change{}, err
	}
	proposal, found, err := s.Backend.PendingAccess(ctx, scopeID, recordType, recordID)
	if err != nil {
		return ledger.Record{}, access.Change{}, err
	}
	if !found {
		return ledger.Record{}, access.Change{}, ErrProposalNotFound
	}
	data, err := domain.DecodeGovernedData(proposal.Data)
	if err != nil {
		return ledger.Record{}, access.Change{}, err
	}
	change, recognised, err := access.Decode(proposal.RecordType, data.Payload)
	if err != nil || !recognised {
		return ledger.Record{}, access.Change{}, err
	}
	if outcome == "confirmed" {
		if change.RecordType == access.RecordCredentialReset && !credentialPath {
			return ledger.Record{}, access.Change{}, ErrUseCredentialResetPath
		}
		if change.RecordType == access.RecordIntakeKey && change.IntakeKey.Action != access.IntakeKeyRevoke && !credentialPath {
			return ledger.Record{}, access.Change{}, ErrUseIntakeKeyPath
		}
		makerUsable, err := s.usable(ctx, proposal.Maker, now)
		if err != nil {
			return ledger.Record{}, access.Change{}, err
		}
		if !makerUsable {
			return ledger.Record{}, access.Change{}, ErrAuthorisationDenied
		}
		if err := s.authoriseMakerAtDecision(ctx, proposal.Maker, change); err != nil {
			return ledger.Record{}, access.Change{}, err
		}
		if err := s.validateAtConfirmation(ctx, change, now); err != nil {
			return ledger.Record{}, access.Change{}, err
		}
	}
	decided, err := domain.Decide(proposal, expectedVersion, principal.HumanID, outcome, now)
	if err != nil {
		return ledger.Record{}, access.Change{}, err
	}
	if err := s.Backend.AppendAccess(ctx, decided); err != nil {
		return ledger.Record{}, access.Change{}, err
	}
	return decided, change, nil
}

func (s AccessService) validateAtConfirmation(ctx context.Context, change access.Change, now time.Time) error {
	switch change.RecordType {
	case access.RecordIdentity:
		current, _, err := s.Backend.IdentityState(ctx, change.HumanID)
		if err != nil {
			return err
		}
		return access.ValidateIdentityTransition(*change.Identity, current, now)
	case access.RecordRoleEligibility, access.RecordCustodianDesignation:
		if change.Active {
			return s.requireIdentityForGrant(ctx, change.HumanID, now)
		}
	case access.RecordIntakeKey:
		return s.validateIntakeKey(ctx, *change.IntakeKey, now)
	case access.RecordCredentialReset:
		ok, err := s.usable(ctx, change.HumanID, now)
		if err != nil {
			return err
		}
		if !ok {
			return access.ErrIdentityNotUsable
		}
	case access.RecordIncidentMembership:
		if change.Active {
			if _, err := s.externalAccessEnd(ctx, change.HumanID, change.ExpiresAt); err != nil {
				return err
			}
			eligible, err := s.Backend.HasRoleEligibility(ctx, change.HumanID)
			if err != nil {
				return err
			}
			ok, err := s.usable(ctx, change.HumanID, now)
			if err != nil {
				return err
			}
			if !eligible || !ok {
				return ErrAuthorisationDenied
			}
		}
	case access.RecordCompartmentMembership:
		if change.Active {
			if _, err := s.externalAccessEnd(ctx, change.HumanID, change.ExpiresAt); err != nil {
				return err
			}
			member, err := s.Backend.HasIncidentMembershipAt(ctx, change.HumanID, change.IncidentID, now)
			if err != nil {
				return err
			}
			if !member {
				return ErrAuthorisationDenied
			}
		}
	}
	return nil
}

func (s AccessService) Administration(ctx context.Context, principal authz.Principal, now time.Time) (access.AdministrationView, error) {
	if err := s.ready(); err != nil {
		return access.AdministrationView{}, err
	}
	if d := authz.Authorise(principal, authz.ActionAdminIdentityMetadata, authz.ObjectScope{SystemMetadata: true}); !d.Allowed {
		return access.AdministrationView{}, ErrAuthorisationDenied
	}
	return s.Backend.AdministrationView(ctx, now)
}

func SelectiveReleaseCompartments(principal authz.Principal, incidentID string) ([]string, error) {
	if !principal.Authenticated || principal.HumanID == "" || !principal.Incidents[incidentID] {
		return nil, ErrAuthorisationDenied
	}
	byIncident := principal.Compartments[incidentID]
	if byIncident == nil || !byIncident["GENERAL"] {
		return nil, ErrAuthorisationDenied
	}
	result := make([]string, 0, len(byIncident))
	for compartment, allowed := range byIncident {
		if allowed {
			result = append(result, compartment)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i] == "GENERAL" {
			return true
		}
		if result[j] == "GENERAL" {
			return false
		}
		return result[i] < result[j]
	})
	return result, nil
}

func (s AccessService) propose(ctx context.Context, maker string, scope domain.Scope, recordID, recordType string, payload any, now time.Time) (ledger.Record, error) {
	if err := s.ready(); err != nil {
		return ledger.Record{}, err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ledger.Record{}, err
	}
	if _, recognised, err := access.Decode(recordType, raw); err != nil || !recognised {
		if err != nil {
			return ledger.Record{}, err
		}
		return ledger.Record{}, errors.New("unsupported access change")
	}
	record, err := domain.ProposeGoverned(scope, recordType, recordID, maker, now, payload)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendAccess(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s AccessService) requireSponsor(ctx context.Context, principal authz.Principal, incidentID string) error {
	if err := s.ready(); err != nil {
		return err
	}
	if !principal.Authenticated || principal.HumanID == "" {
		return ErrAuthorisationDenied
	}
	allowed, err := s.Backend.IsSponsor(ctx, principal.HumanID, incidentID)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrAuthorisationDenied
	}
	return nil
}

func (s AccessService) authoriseMakerAtDecision(ctx context.Context, maker string, change access.Change) error {
	switch change.RecordType {
	case access.RecordIncidentSponsor:
		hasSponsor, err := s.Backend.HasSponsor(ctx, change.IncidentID)
		if err != nil {
			return err
		}
		if hasSponsor {
			allowed, err := s.Backend.IsSponsor(ctx, maker, change.IncidentID)
			if err != nil {
				return err
			}
			if !allowed {
				return ErrAuthorisationDenied
			}
			return nil
		}
		eligible, err := s.Backend.IsMajorEligible(ctx, maker)
		if err != nil {
			return err
		}
		if !eligible {
			return ErrAuthorisationDenied
		}
	case access.RecordCompartmentController, access.RecordIncidentMembership:
		allowed, err := s.Backend.IsSponsor(ctx, maker, change.IncidentID)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrAuthorisationDenied
		}
	case access.RecordCompartmentMembership:
		allowed, err := s.Backend.IsController(ctx, maker, change.IncidentID, change.Compartment)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrAuthorisationDenied
		}
	case access.RecordOutputPermission, access.RecordIdentity, access.RecordRoleEligibility, access.RecordCustodianDesignation, access.RecordCredentialReset, access.RecordIntakeKey:
		allowed, err := s.Backend.IsCustodian(ctx, maker)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrAuthorisationDenied
		}
	default:
		return ErrAuthorisationDenied
	}
	return nil
}

// ErrExternalAccessOutlivesAccount refuses incident or compartment access for
// an external person that would outlast their account.
var ErrExternalAccessOutlivesAccount = errors.New("an external person's access cannot end after their account ends")

// externalAccessEnd applies the external-access rule (T-222): for an external
// person, incident and compartment access must end no later than their
// account. If no end is given it defaults to the account end. Internal people
// are unchanged.
func (s AccessService) externalAccessEnd(ctx context.Context, humanID string, requested *time.Time) (*time.Time, error) {
	state, _, err := s.Backend.IdentityState(ctx, humanID)
	if err != nil {
		return nil, err
	}
	if !state.Found || !state.External {
		return requested, nil
	}
	if state.AccountExpiresAt == nil {
		return nil, access.ErrExternalNeedsExpiry
	}
	if requested == nil {
		end := state.AccountExpiresAt.UTC()
		return &end, nil
	}
	if requested.After(*state.AccountExpiresAt) {
		return nil, ErrExternalAccessOutlivesAccount
	}
	return requested, nil
}

func (s AccessService) ready() error {
	if s.Backend == nil {
		return errors.New("access backend is required")
	}
	return nil
}
