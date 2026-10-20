package app

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
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
	HasRoleEligibility(context.Context, string) (bool, error)
	IncidentExists(context.Context, string) (bool, error)
	HasIncidentMembershipAt(context.Context, string, string, time.Time) (bool, error)
	AdministrationView(context.Context, time.Time) (access.AdministrationView, error)
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
	return s.propose(ctx, principal.HumanID, scope, recordID, access.RecordCompartmentMembership, payload, now)
}

func (s AccessService) Decide(ctx context.Context, principal authz.Principal, scopeID, recordType, recordID, expectedVersion, outcome string, now time.Time) (ledger.Record, error) {
	if err := s.ready(); err != nil {
		return ledger.Record{}, err
	}
	if d := authz.Authorise(principal, authz.ActionAdminIdentityMetadata, authz.ObjectScope{SystemMetadata: true}); !d.Allowed {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	proposal, found, err := s.Backend.PendingAccess(ctx, scopeID, recordType, recordID)
	if err != nil {
		return ledger.Record{}, err
	}
	if !found {
		return ledger.Record{}, ErrProposalNotFound
	}
	data, err := domain.DecodeGovernedData(proposal.Data)
	if err != nil {
		return ledger.Record{}, err
	}
	change, recognised, err := access.Decode(proposal.RecordType, data.Payload)
	if err != nil || !recognised {
		return ledger.Record{}, err
	}
	if err := s.authoriseMakerAtDecision(ctx, proposal.Maker, change); err != nil {
		return ledger.Record{}, err
	}
	if outcome == "confirmed" && change.Active {
		switch change.RecordType {
		case access.RecordIncidentMembership:
			eligible, err := s.Backend.HasRoleEligibility(ctx, change.HumanID)
			if err != nil {
				return ledger.Record{}, err
			}
			if !eligible {
				return ledger.Record{}, ErrAuthorisationDenied
			}
		case access.RecordCompartmentMembership:
			member, err := s.Backend.HasIncidentMembershipAt(ctx, change.HumanID, change.IncidentID, now)
			if err != nil {
				return ledger.Record{}, err
			}
			if !member {
				return ledger.Record{}, ErrAuthorisationDenied
			}
		}
	}
	decided, err := domain.Decide(proposal, expectedVersion, principal.HumanID, outcome, now)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendAccess(ctx, decided); err != nil {
		return ledger.Record{}, err
	}
	return decided, nil
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
	default:
		return ErrAuthorisationDenied
	}
	return nil
}

func (s AccessService) ready() error {
	if s.Backend == nil {
		return errors.New("access backend is required")
	}
	return nil
}
