package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

type RecoveryBackend interface {
	AppendRecovery(context.Context, ledger.Record) error
	PendingRecovery(context.Context, string, string, string) (ledger.Record, bool, error)
	RecoveryState(context.Context, string) (domain.RecoveryState, bool, error)
	CommonOperatingPicture(context.Context, string) (domain.OperatingPicture, bool, error)
	ClosureOutstanding(context.Context, string, time.Time) (domain.ClosureOutstanding, error)
}

type RecoveryService struct {
	Backend RecoveryBackend
	Window  WindowPolicy
}

func (s RecoveryService) RecordRecoveryUpdate(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, value domain.RecoveryUpdate, now time.Time) (ledger.Record, error) {
	if err := s.authorise(ctx, principal, authz.ActionRecord, scope); err != nil {
		return ledger.Record{}, err
	}
	record, err := domain.RecordRecoveryUpdate(scope, recordID, principal.HumanID, now, value)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendRecovery(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s RecoveryService) ProposeStandDown(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, value domain.StandDown, now time.Time) (ledger.Record, error) {
	return s.propose(ctx, principal, scope, func() (ledger.Record, error) {
		return domain.ProposeStandDown(scope, recordID, principal.HumanID, now, value)
	})
}

func (s RecoveryService) ProposePIR(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, value domain.PIR, now time.Time) (ledger.Record, error) {
	return s.propose(ctx, principal, scope, func() (ledger.Record, error) {
		return domain.ProposePIR(scope, recordID, principal.HumanID, now, value)
	})
}

func (s RecoveryService) ProposeRetention(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, value domain.RetentionDisposition, now time.Time) (ledger.Record, error) {
	return s.propose(ctx, principal, scope, func() (ledger.Record, error) {
		return domain.ProposeRetention(scope, recordID, principal.HumanID, now, value)
	})
}

func (s RecoveryService) ProposeClosure(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, value domain.Closure, now time.Time) (ledger.Record, error) {
	if err := s.authorise(ctx, principal, authz.ActionPropose, scope); err != nil {
		return ledger.Record{}, err
	}
	state, found, err := s.Backend.RecoveryState(ctx, scope.IncidentID)
	if err != nil {
		return ledger.Record{}, err
	}
	if !found {
		return ledger.Record{}, errors.New("incident not found")
	}
	if err := s.closureAllowed(ctx, state, now); err != nil {
		return ledger.Record{}, err
	}
	record, err := domain.ProposeClosure(scope, recordID, principal.HumanID, now, value)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendRecovery(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s RecoveryService) Decide(ctx context.Context, principal authz.Principal, incidentID, recordType, recordID, expectedVersion, outcome string, now time.Time) (ledger.Record, error) {
	if s.Backend == nil {
		return ledger.Record{}, errors.New("recovery backend is required")
	}
	if !domain.IsRecoveryType(recordType) || recordType == domain.TypeRecoveryUpdate {
		return ledger.Record{}, ErrProposalNotFound
	}
	proposal, found, err := s.Backend.PendingRecovery(ctx, incidentID, recordType, recordID)
	if err != nil {
		return ledger.Record{}, err
	}
	if !found {
		return ledger.Record{}, ErrProposalNotFound
	}
	scope := domain.Scope{IncidentID: proposal.IncidentID, Mode: proposal.IncidentMode, Compartment: proposal.Compartment}
	if recordType == domain.TypeReopening {
		if err := s.authoriseReopening(ctx, principal, authz.ActionConfirm, scope); err != nil {
			return ledger.Record{}, err
		}
	} else if err := s.authorise(ctx, principal, authz.ActionConfirm, scope); err != nil {
		return ledger.Record{}, err
	}
	if recordType == domain.TypeClosure && outcome == "confirmed" {
		state, found, err := s.Backend.RecoveryState(ctx, incidentID)
		if err != nil {
			return ledger.Record{}, err
		}
		if !found {
			return ledger.Record{}, errors.New("incident not found")
		}
		if err := s.closureAllowed(ctx, state, now); err != nil {
			return ledger.Record{}, err
		}
	}
	decided, err := decideGoverned(ctx, s.Window, proposal, expectedVersion, principal, outcome, now)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendRecovery(ctx, decided); err != nil {
		return ledger.Record{}, err
	}
	return decided, nil
}

func (s RecoveryService) State(ctx context.Context, principal authz.Principal, incidentID, compartment string) (domain.RecoveryState, bool, error) {
	if s.Backend == nil {
		return domain.RecoveryState{}, false, errors.New("recovery backend is required")
	}
	if d := authz.Authorise(principal, authz.ActionRead, authz.ObjectScope{IncidentID: incidentID, Compartment: compartment}); !d.Allowed {
		return domain.RecoveryState{}, false, ErrAuthorisationDenied
	}
	state, found, err := s.Backend.RecoveryState(ctx, incidentID)
	if err != nil || !found {
		return state, found, err
	}
	updates := state.Updates[:0]
	for _, update := range state.Updates {
		if update.Compartment == "GENERAL" || update.Compartment == compartment {
			updates = append(updates, update)
		}
	}
	state.Updates = updates
	if state.Closure != nil && state.Closure.PrivilegedCompartment != "" && state.Closure.PrivilegedCompartment != compartment {
		closure := *state.Closure
		closure.PrivilegedCompartment = ""
		closure.PrivilegedSummary = ""
		state.Closure = &closure
	}
	return domain.FinaliseRecoveryState(state), true, nil
}

func (s RecoveryService) GeneratePIR(ctx context.Context, principal authz.Principal, incidentID, compartment string) (domain.PIRDraft, error) {
	if s.Backend == nil {
		return domain.PIRDraft{}, errors.New("recovery backend is required")
	}
	if d := authz.Authorise(principal, authz.ActionRead, authz.ObjectScope{IncidentID: incidentID, Compartment: compartment}); !d.Allowed {
		return domain.PIRDraft{}, ErrAuthorisationDenied
	}
	state, found, err := s.Backend.RecoveryState(ctx, incidentID)
	if err != nil {
		return domain.PIRDraft{}, err
	}
	if !found {
		return domain.PIRDraft{}, errors.New("incident not found")
	}
	if state.StandDownAt == nil {
		return domain.PIRDraft{}, errors.New("PIR generation requires confirmed stand-down")
	}
	picture, found, err := s.Backend.CommonOperatingPicture(ctx, incidentID)
	if err != nil {
		return domain.PIRDraft{}, err
	}
	if !found {
		return domain.PIRDraft{}, errors.New("incident not found")
	}
	picture = filterOperatingPicture(picture, compartment)
	return domain.GeneratePIRDraft(picture), nil
}

func (s RecoveryService) propose(ctx context.Context, principal authz.Principal, scope domain.Scope, build func() (ledger.Record, error)) (ledger.Record, error) {
	if err := s.authorise(ctx, principal, authz.ActionPropose, scope); err != nil {
		return ledger.Record{}, err
	}
	record, err := build()
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendRecovery(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

// ProposeReopening proposes reopening a closed incident (addendum A1). It is
// the one proposal a closed incident accepts. A different Major on the
// incident confirms it; no single-person window can cover it.
func (s RecoveryService) ProposeReopening(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, value domain.Reopening, now time.Time) (ledger.Record, error) {
	if err := s.authoriseReopening(ctx, principal, authz.ActionPropose, scope); err != nil {
		return ledger.Record{}, err
	}
	record, err := domain.ProposeReopening(scope, recordID, principal.HumanID, now, value)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendRecovery(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

// CanReopen reports whether this person may propose reopening the incident
// now: it is closed and they may propose in it.
func (s RecoveryService) CanReopen(ctx context.Context, principal authz.Principal, incidentID string) bool {
	return s.authoriseReopening(ctx, principal, authz.ActionPropose, domain.Scope{IncidentID: incidentID, Compartment: "GENERAL"}) == nil
}

// RecoveryApprovalSummary words a reopening proposal for Approvals.
func RecoveryApprovalSummary(recordType string, payload map[string]any) (summary, consequence string, ok bool) {
	if recordType != domain.TypeReopening {
		return "", "", false
	}
	reason, _ := payload["reason"].(string)
	return "Reopen the incident: " + reason, "The incident becomes writable again. Its closure and stand-down are superseded and kept in history; closing again needs a new stand-down and closure.", true
}

// ClosureBlockers names everything that stops the incident closing now: the
// recovery milestones, pending proposals and running obligation clocks.
func (s RecoveryService) ClosureBlockers(ctx context.Context, principal authz.Principal, incidentID string, now time.Time) ([]string, error) {
	if d := authz.Authorise(principal, authz.ActionRead, authz.ObjectScope{IncidentID: incidentID, Compartment: "GENERAL"}); !d.Allowed {
		return nil, ErrAuthorisationDenied
	}
	state, found, err := s.Backend.RecoveryState(ctx, incidentID)
	if err != nil {
		return nil, err
	}
	if !found || state.ClosedAt != nil {
		return nil, nil
	}
	return s.blockers(ctx, state, now)
}

func (s RecoveryService) blockers(ctx context.Context, state domain.RecoveryState, now time.Time) ([]string, error) {
	out := domain.ClosureBlockers(state)
	outstanding, err := s.Backend.ClosureOutstanding(ctx, state.IncidentID, now)
	if err != nil {
		return nil, err
	}
	types := make([]string, 0, len(outstanding.PendingByType))
	for t := range outstanding.PendingByType {
		types = append(types, t)
	}
	sort.Strings(types)
	for _, t := range types {
		n := outstanding.PendingByType[t]
		label := strings.ReplaceAll(strings.TrimPrefix(strings.TrimPrefix(t, "incident."), "security."), "_", " ")
		if n == 1 {
			out = append(out, "1 pending "+label+" proposal")
		} else {
			out = append(out, fmt.Sprintf("%d pending %s proposals", n, label))
		}
	}
	for _, c := range outstanding.RunningClocks {
		out = append(out, "obligation clock "+c.Name+" still running until "+c.Deadline.UTC().Format("2 Jan 2006 15:04 UTC"))
	}
	return out, nil
}

// closureAllowed refuses closure while anything is outstanding, naming it.
func (s RecoveryService) closureAllowed(ctx context.Context, state domain.RecoveryState, now time.Time) error {
	if state.ClosedAt != nil {
		return domain.ErrIncidentClosed
	}
	blockers, err := s.blockers(ctx, state, now)
	if err != nil {
		return err
	}
	if len(blockers) > 0 {
		return domain.ClosureBlocked{Blockers: blockers}
	}
	return nil
}

// authoriseReopening is the closed-incident counterpart of authorise: the
// incident must be closed.
func (s RecoveryService) authoriseReopening(ctx context.Context, principal authz.Principal, action authz.Action, scope domain.Scope) error {
	if s.Backend == nil {
		return errors.New("recovery backend is required")
	}
	if d := authz.Authorise(principal, action, authz.ObjectScope{IncidentID: scope.IncidentID, Compartment: "GENERAL"}); !d.Allowed {
		return ErrAuthorisationDenied
	}
	state, found, err := s.Backend.RecoveryState(ctx, scope.IncidentID)
	if err != nil {
		return err
	}
	if !found || state.ClosedAt == nil {
		return errors.New("only a closed incident can be reopened")
	}
	return nil
}

func (s RecoveryService) authorise(ctx context.Context, principal authz.Principal, action authz.Action, scope domain.Scope) error {
	if s.Backend == nil {
		return errors.New("recovery backend is required")
	}
	if d := authz.Authorise(principal, action, authz.ObjectScope{IncidentID: scope.IncidentID, Compartment: scope.Compartment}); !d.Allowed {
		return ErrAuthorisationDenied
	}
	state, found, err := s.Backend.RecoveryState(ctx, scope.IncidentID)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("incident not found")
	}
	if state.ClosedAt != nil {
		return domain.ErrIncidentClosed
	}
	return nil
}
