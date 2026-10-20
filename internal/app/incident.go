package app

import (
	"context"
	"errors"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

var (
	ErrAuthorisationDenied = errors.New("authorisation denied")
	ErrProposalNotFound    = errors.New("proposal not found")
	ErrIncidentExists      = errors.New("incident declaration already exists or is pending")
)

type IncidentBackend interface {
	AppendIncident(context.Context, ledger.Record) error
	PendingProposalRecord(context.Context, string, string, string) (ledger.Record, bool, error)
	CommonOperatingPicture(context.Context, string) (domain.OperatingPicture, bool, error)
	DeclarationExists(context.Context, string) (bool, error)
}

type incidentObligationBackend interface {
	ObligationClocks(context.Context, string, string, time.Time) ([]domain.ObligationClockView, error)
}

type IncidentService struct {
	Backend IncidentBackend
}

func (s IncidentService) ProposeIncidentDeclaration(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, declaration domain.IncidentDeclaration, now time.Time) (ledger.Record, error) {
	if err := s.ready(); err != nil {
		return ledger.Record{}, err
	}
	if !principal.Authenticated || principal.HumanID == "" || principal.Role != authz.RoleMajor {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	exists, err := s.Backend.DeclarationExists(ctx, scope.IncidentID)
	if err != nil {
		return ledger.Record{}, err
	}
	if exists {
		return ledger.Record{}, ErrIncidentExists
	}
	record, err := domain.ProposeIncidentDeclaration(scope, recordID, principal.HumanID, now, declaration)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendIncident(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s IncidentService) RecordFact(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, fact domain.Fact, now time.Time) (ledger.Record, error) {
	if err := s.authorise(principal, authz.ActionRecord, scope); err != nil {
		return ledger.Record{}, err
	}
	record, err := domain.RecordFact(scope, recordID, principal.HumanID, now, fact)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendIncident(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s IncidentService) RecordUncertainty(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, item domain.Uncertainty, now time.Time) (ledger.Record, error) {
	if err := s.authorise(principal, authz.ActionRecord, scope); err != nil {
		return ledger.Record{}, err
	}
	record, err := domain.RecordUncertaintyItem(scope, recordID, principal.HumanID, now, item)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendIncident(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s IncidentService) ProposeAction(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, action domain.Action, now time.Time) (ledger.Record, error) {
	if err := s.authorise(principal, authz.ActionPropose, scope); err != nil {
		return ledger.Record{}, err
	}
	record, err := domain.ProposeAction(scope, recordID, principal.HumanID, now, action)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendIncident(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s IncidentService) ActivateChecklist(ctx context.Context, principal authz.Principal, scope domain.Scope, templateID, activationID string, template domain.ChecklistTemplate, ownerOverrides map[string]string, now time.Time) ([]ledger.Record, error) {
	if err := s.authorise(principal, authz.ActionPropose, scope); err != nil {
		return nil, err
	}
	actions, err := domain.InstantiateChecklist(templateID, activationID, template, ownerOverrides, now)
	if err != nil {
		return nil, err
	}
	proposals := make([]ledger.Record, 0, len(actions))
	for _, item := range actions {
		record, err := domain.ProposeAction(scope, item.RecordID, principal.HumanID, now, item.Action)
		if err != nil {
			return nil, err
		}
		proposals = append(proposals, record)
	}
	for _, record := range proposals {
		if err := s.Backend.AppendIncident(ctx, record); err != nil {
			return nil, err
		}
	}
	return proposals, nil
}

func (s IncidentService) ProposeDecision(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, decision domain.Decision, now time.Time) (ledger.Record, error) {
	if err := s.authorise(principal, authz.ActionPropose, scope); err != nil {
		return ledger.Record{}, err
	}
	record, err := domain.ProposeDecision(scope, recordID, principal.HumanID, now, decision)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendIncident(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s IncidentService) ProposeOperationalRoleAssignment(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, assignment domain.OperationalRoleAssignment, now time.Time) (ledger.Record, error) {
	if err := s.authorise(principal, authz.ActionPropose, scope); err != nil {
		return ledger.Record{}, err
	}
	record, err := domain.ProposeOperationalRoleAssignment(scope, recordID, principal.HumanID, now, assignment)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendIncident(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

// Decide reloads the proposal from CRAB's projection. Presentation callers
// provide only identifiers and the version they reviewed; they cannot supply a
// forged proposal body or swap its incident/compartment scope.
func (s IncidentService) Decide(ctx context.Context, principal authz.Principal, incidentID, recordType, recordID, expectedVersion, outcome string, now time.Time) (ledger.Record, error) {
	if err := s.ready(); err != nil {
		return ledger.Record{}, err
	}
	proposal, found, err := s.Backend.PendingProposalRecord(ctx, incidentID, recordType, recordID)
	if err != nil {
		return ledger.Record{}, err
	}
	if !found {
		return ledger.Record{}, ErrProposalNotFound
	}
	if proposal.RecordType == domain.TypeIncidentDeclaration {
		if !principal.Authenticated || principal.HumanID == "" || principal.Role != authz.RoleMajor {
			return ledger.Record{}, ErrAuthorisationDenied
		}
	} else {
		scope := domain.Scope{IncidentID: proposal.IncidentID, Mode: proposal.IncidentMode, Compartment: proposal.Compartment}
		if err := s.authorise(principal, authz.ActionConfirm, scope); err != nil {
			return ledger.Record{}, err
		}
	}
	decided, err := domain.Decide(proposal, expectedVersion, principal.HumanID, outcome, now)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendIncident(ctx, decided); err != nil {
		return ledger.Record{}, err
	}
	return decided, nil
}

func (s IncidentService) CommonOperatingPicture(ctx context.Context, principal authz.Principal, incidentID, compartment string) (domain.OperatingPicture, bool, error) {
	if err := s.ready(); err != nil {
		return domain.OperatingPicture{}, false, err
	}
	if d := authz.Authorise(principal, authz.ActionRead, authz.ObjectScope{IncidentID: incidentID, Compartment: compartment}); !d.Allowed {
		return domain.OperatingPicture{}, false, ErrAuthorisationDenied
	}
	picture, found, err := s.Backend.CommonOperatingPicture(ctx, incidentID)
	if err != nil || !found {
		return picture, found, err
	}
	picture = filterOperatingPicture(picture, compartment)
	if backend, ok := s.Backend.(incidentObligationBackend); ok {
		obligations, err := backend.ObligationClocks(ctx, incidentID, compartment, time.Now().UTC())
		if err != nil {
			return domain.OperatingPicture{}, false, err
		}
		picture.Obligations = obligations
	}
	return picture, true, nil
}

func filterOperatingPicture(picture domain.OperatingPicture, compartment string) domain.OperatingPicture {
	allowed := func(value string) bool { return value == "GENERAL" || value == compartment }
	facts := picture.Facts[:0]
	for _, value := range picture.Facts {
		if allowed(value.Compartment) {
			facts = append(facts, value)
		}
	}
	picture.Facts = facts
	unknowns := picture.Unknowns[:0]
	for _, value := range picture.Unknowns {
		if allowed(value.Compartment) {
			unknowns = append(unknowns, value)
		}
	}
	picture.Unknowns = unknowns
	hypotheses := picture.Hypotheses[:0]
	for _, value := range picture.Hypotheses {
		if allowed(value.Compartment) {
			hypotheses = append(hypotheses, value)
		}
	}
	picture.Hypotheses = hypotheses
	decisions := picture.Decisions[:0]
	for _, value := range picture.Decisions {
		if allowed(value.Compartment) {
			decisions = append(decisions, value)
		}
	}
	picture.Decisions = decisions
	actions := picture.Actions[:0]
	for _, value := range picture.Actions {
		if allowed(value.Compartment) {
			actions = append(actions, value)
		}
	}
	picture.Actions = actions
	roles := picture.OperationalRoles[:0]
	for _, value := range picture.OperationalRoles {
		if allowed(value.Compartment) {
			roles = append(roles, value)
		}
	}
	picture.OperationalRoles = roles
	pending := picture.Pending[:0]
	for _, value := range picture.Pending {
		if allowed(value.Compartment) {
			pending = append(pending, value)
		}
	}
	picture.Pending = pending
	timeline := picture.Timeline[:0]
	for _, value := range picture.Timeline {
		if allowed(value.Compartment) {
			timeline = append(timeline, value)
		}
	}
	picture.Timeline = timeline
	return picture
}

func (s IncidentService) authorise(principal authz.Principal, action authz.Action, scope domain.Scope) error {
	if err := s.ready(); err != nil {
		return err
	}
	decision := authz.Authorise(principal, action, authz.ObjectScope{IncidentID: scope.IncidentID, Compartment: scope.Compartment})
	if !decision.Allowed {
		return ErrAuthorisationDenied
	}
	return nil
}

func (s IncidentService) ready() error {
	if s.Backend == nil {
		return errors.New("incident backend is required")
	}
	return nil
}
