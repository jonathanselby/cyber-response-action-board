package app

import (
	"context"
	"errors"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

type RelationshipBackend interface {
	AppendRelationship(context.Context, ledger.Record) error
	PendingRelationship(context.Context, string, string) (ledger.Record, bool, error)
	Relationships(context.Context, string) ([]domain.RelationshipView, error)
}

// RelationshipService governs "incident A is related to incident B" (AC-84).
// Proposing, confirming and seeing a link all require authority over both
// incidents, so a link never reveals an incident to someone who cannot
// already see it.
type RelationshipService struct {
	Backend RelationshipBackend
	// Exists reports whether an incident has been declared.
	Exists func(context.Context, string) (bool, error)
}

// canSeeBoth is the one rule for every relationship operation.
func canSeeBoth(principal authz.Principal, a, b string) bool {
	for _, id := range []string{a, b} {
		if d := authz.Authorise(principal, authz.ActionRead, authz.ObjectScope{IncidentID: id, Compartment: "GENERAL"}); !d.Allowed {
			return false
		}
	}
	return true
}

func (s RelationshipService) Propose(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, value domain.IncidentRelationship, now time.Time) (ledger.Record, error) {
	if s.Backend == nil {
		return ledger.Record{}, errors.New("relationship backend is required")
	}
	if d := authz.Authorise(principal, authz.ActionPropose, authz.ObjectScope{IncidentID: scope.IncidentID, Compartment: "GENERAL"}); !d.Allowed {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	if !canSeeBoth(principal, scope.IncidentID, value.RelatedIncidentID) {
		// The same refusal as an unknown incident: nothing is revealed.
		return ledger.Record{}, ErrAuthorisationDenied
	}
	if s.Exists != nil {
		if ok, err := s.Exists(ctx, value.RelatedIncidentID); err != nil || !ok {
			if err != nil {
				return ledger.Record{}, err
			}
			return ledger.Record{}, ErrAuthorisationDenied
		}
	}
	scope.Compartment = "GENERAL"
	record, err := domain.ProposeIncidentRelationship(scope, recordID, principal.HumanID, now, value)
	if err != nil {
		return ledger.Record{}, err
	}
	return record, s.Backend.AppendRelationship(ctx, record)
}

func (s RelationshipService) Decide(ctx context.Context, principal authz.Principal, incidentID, recordID, version, outcome string, now time.Time) (ledger.Record, error) {
	if s.Backend == nil {
		return ledger.Record{}, errors.New("relationship backend is required")
	}
	if d := authz.Authorise(principal, authz.ActionConfirm, authz.ObjectScope{IncidentID: incidentID, Compartment: "GENERAL"}); !d.Allowed {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	proposal, found, err := s.Backend.PendingRelationship(ctx, incidentID, recordID)
	if err != nil {
		return ledger.Record{}, err
	}
	if !found {
		return ledger.Record{}, ErrProposalNotFound
	}
	value, err := domain.DecodePayload[domain.IncidentRelationship](proposal)
	if err != nil {
		return ledger.Record{}, err
	}
	if !canSeeBoth(principal, incidentID, value.RelatedIncidentID) {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	decided, err := domain.Decide(proposal, version, principal.HumanID, outcome, now)
	if err != nil {
		return ledger.Record{}, err
	}
	return decided, s.Backend.AppendRelationship(ctx, decided)
}

// Visible returns the confirmed links of one incident that the viewer may
// see, which means the viewer can read both incidents (T-205).
func (s RelationshipService) Visible(ctx context.Context, principal authz.Principal, incidentID string) ([]domain.RelationshipView, error) {
	if s.Backend == nil {
		return nil, nil
	}
	if d := authz.Authorise(principal, authz.ActionRead, authz.ObjectScope{IncidentID: incidentID, Compartment: "GENERAL"}); !d.Allowed {
		return nil, ErrAuthorisationDenied
	}
	all, err := s.Backend.Relationships(ctx, incidentID)
	if err != nil {
		return nil, err
	}
	var out []domain.RelationshipView
	for _, v := range all {
		if canSeeBoth(principal, incidentID, v.RelatedIncidentID) {
			out = append(out, v)
		}
	}
	return out, nil
}
