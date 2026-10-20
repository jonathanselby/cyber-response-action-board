package app

import (
	"context"
	"errors"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

type ObligationBackend interface {
	AppendObligation(context.Context, ledger.Record) error
	PendingObligation(context.Context, string, string, string) (ledger.Record, bool, error)
	ObligationClocks(context.Context, string, string, time.Time) ([]domain.ObligationClockView, error)
}

type ObligationService struct{ Backend ObligationBackend }

func (s ObligationService) ProposeDefinition(ctx context.Context, principal authz.Principal, recordID string, def domain.ObligationDefinition, now time.Time) (ledger.Record, error) {
	if err := s.ready(); err != nil {
		return ledger.Record{}, err
	}
	if !principal.Authenticated || principal.HumanID == "" || principal.Role != authz.RoleMajor {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	record, err := domain.ProposeObligationDefinition(recordID, principal.HumanID, now, def)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendObligation(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s ObligationService) ProposeTrigger(ctx context.Context, principal authz.Principal, scope domain.Scope, clockID string, trigger domain.ObligationTrigger, now time.Time) (ledger.Record, error) {
	if d := authz.Authorise(principal, authz.ActionPropose, authz.ObjectScope{IncidentID: scope.IncidentID, Compartment: scope.Compartment}); !d.Allowed {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	record, err := domain.ProposeObligationTrigger(scope, clockID, principal.HumanID, now, trigger)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendObligation(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s ObligationService) ProposeChange(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, change domain.ObligationChange, now time.Time) (ledger.Record, error) {
	if d := authz.Authorise(principal, authz.ActionPropose, authz.ObjectScope{IncidentID: scope.IncidentID, Compartment: scope.Compartment}); !d.Allowed {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	record, err := domain.ProposeObligationChange(scope, recordID, principal.HumanID, now, change)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendObligation(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s ObligationService) Decide(ctx context.Context, principal authz.Principal, scopeID, recordType, recordID, expectedVersion, outcome string, now time.Time) (ledger.Record, error) {
	if err := s.ready(); err != nil {
		return ledger.Record{}, err
	}
	proposal, found, err := s.Backend.PendingObligation(ctx, scopeID, recordType, recordID)
	if err != nil {
		return ledger.Record{}, err
	}
	if !found {
		return ledger.Record{}, ErrProposalNotFound
	}
	if proposal.IncidentMode == "system" {
		if !principal.Authenticated || principal.HumanID == "" || principal.Role != authz.RoleMajor {
			return ledger.Record{}, ErrAuthorisationDenied
		}
	} else if d := authz.Authorise(principal, authz.ActionConfirm, authz.ObjectScope{IncidentID: proposal.IncidentID, Compartment: proposal.Compartment}); !d.Allowed {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	decided, err := domain.Decide(proposal, expectedVersion, principal.HumanID, outcome, now)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendObligation(ctx, decided); err != nil {
		return ledger.Record{}, err
	}
	return decided, nil
}

func (s ObligationService) Clocks(ctx context.Context, principal authz.Principal, incidentID, compartment string, now time.Time) ([]domain.ObligationClockView, error) {
	if d := authz.Authorise(principal, authz.ActionRead, authz.ObjectScope{IncidentID: incidentID, Compartment: compartment}); !d.Allowed {
		return nil, ErrAuthorisationDenied
	}
	return s.Backend.ObligationClocks(ctx, incidentID, compartment, now)
}

func (s ObligationService) ready() error {
	if s.Backend == nil {
		return errors.New("obligation backend is required")
	}
	return nil
}
