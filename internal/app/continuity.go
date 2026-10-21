package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

type FirstReportAttachmentInput struct {
	Filename          string
	DeclaredMediaType string
	Reader            io.Reader
}

type ContinuityBackend interface {
	PutFirstReportBlob(context.Context, string, string, io.Reader) (domain.BlobReceipt, error)
	AppendContinuity(context.Context, ledger.Record) error
	PendingContinuity(context.Context, string, string, string) (ledger.Record, bool, error)
	FirstReports(context.Context) ([]domain.FirstReportView, error)
	ScopeAssessments(context.Context, string, string) ([]domain.ScopeAssessmentView, error)
	SeverityChanges(context.Context, string, string) ([]domain.SeverityChangeView, error)
	Handovers(context.Context, string, string) ([]domain.HandoverView, error)
	ResourceRequests(context.Context, string, string) ([]domain.ResourceRequestView, error)
}

type pendingSeverityReader interface {
	PendingSeverityChanges(context.Context, string, string) ([]ledger.Record, error)
}

type ContinuityService struct {
	Backend  ContinuityBackend
	Incident IncidentPictureReader
}

func (s ContinuityService) CaptureFirstReport(ctx context.Context, principal authz.Principal, reportID string, report domain.FirstReport, attachments []FirstReportAttachmentInput, now time.Time) (ledger.Record, error) {
	if err := s.ready(); err != nil {
		return ledger.Record{}, err
	}
	if !principal.Authenticated || principal.HumanID == "" || (principal.Role != authz.RoleMajor && principal.Role != authz.RoleMinor) {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	for _, input := range attachments {
		if strings.TrimSpace(input.Filename) == "" || input.Reader == nil {
			return ledger.Record{}, errors.New("first-report attachment requires filename and reader")
		}
		receipt, err := s.Backend.PutFirstReportBlob(ctx, input.Filename, input.DeclaredMediaType, input.Reader)
		if err != nil {
			return ledger.Record{}, err
		}
		report.Attachments = append(report.Attachments, receipt)
	}
	record, err := domain.RecordFirstReport(reportID, principal.HumanID, now, report)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendContinuity(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s ContinuityService) TriageFirstReport(ctx context.Context, principal authz.Principal, recordID string, value domain.FirstReportDisposition, now time.Time) (ledger.Record, error) {
	if err := s.ready(); err != nil {
		return ledger.Record{}, err
	}
	if !principal.Authenticated || principal.HumanID == "" || (principal.Role != authz.RoleMajor && principal.Role != authz.RoleMinor) {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	record, err := domain.RecordFirstReportDisposition(recordID, principal.HumanID, now, value)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendContinuity(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s ContinuityService) RecordScopeAssessment(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, value domain.ScopeAssessment, now time.Time) (ledger.Record, error) {
	if d := authz.Authorise(principal, authz.ActionRecord, authz.ObjectScope{IncidentID: scope.IncidentID, Compartment: scope.Compartment}); !d.Allowed {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	record, err := domain.RecordScopeAssessment(scope, recordID, principal.HumanID, now, value)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendContinuity(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s ContinuityService) ProposeSeverityChange(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, value domain.SeverityChange, now time.Time) (ledger.Record, error) {
	if d := authz.Authorise(principal, authz.ActionPropose, authz.ObjectScope{IncidentID: scope.IncidentID, Compartment: scope.Compartment}); !d.Allowed {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	record, err := domain.ProposeSeverityChange(scope, recordID, principal.HumanID, now, value)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendContinuity(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s ContinuityService) DecideSeverity(ctx context.Context, principal authz.Principal, incidentID, recordID, version, outcome string, now time.Time) (ledger.Record, error) {
	if s.Backend == nil {
		return ledger.Record{}, errors.New("continuity backend is required")
	}
	proposal, found, err := s.Backend.PendingContinuity(ctx, incidentID, domain.TypeSeverityChange, recordID)
	if err != nil {
		return ledger.Record{}, err
	}
	if !found {
		return ledger.Record{}, ErrProposalNotFound
	}
	if d := authz.Authorise(principal, authz.ActionConfirm, authz.ObjectScope{IncidentID: incidentID, Compartment: proposal.Compartment}); !d.Allowed {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	decided, err := domain.Decide(proposal, version, principal.HumanID, outcome, now)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendContinuity(ctx, decided); err != nil {
		return ledger.Record{}, err
	}
	return decided, nil
}

func (s ContinuityService) GenerateHandover(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, nextObjectives, communicationCommitments []string, now time.Time) (ledger.Record, error) {
	if s.Incident == nil {
		return ledger.Record{}, errors.New("incident operating-picture reader is required")
	}
	if d := authz.Authorise(principal, authz.ActionRecord, authz.ObjectScope{IncidentID: scope.IncidentID, Compartment: scope.Compartment}); !d.Allowed {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	picture, found, err := s.Incident.CommonOperatingPicture(ctx, principal, scope.IncidentID, scope.Compartment)
	if err != nil || !found {
		if err != nil {
			return ledger.Record{}, err
		}
		return ledger.Record{}, ErrAuthorisationDenied
	}
	h := domain.Handover{Situation: picture.Incident.Summary, NextObjectives: cleanStrings(nextObjectives), CommunicationCommitments: cleanStrings(communicationCommitments)}
	for _, role := range picture.OperationalRoles {
		label := role.Assignment.RoleName + ": " + role.Assignment.HumanID
		if role.Assignment.Deputy {
			label += " (deputy)"
		}
		h.OperationalRoles = append(h.OperationalRoles, label)
	}
	for _, action := range picture.Actions {
		if action.Action.Status != domain.ActionStatusComplete && action.Action.Status != domain.ActionStatusSkipped {
			h.CriticalActions = append(h.CriticalActions, fmt.Sprintf("%s [%s] — %s", action.Action.Title, action.Action.Priority, action.Action.OwnerHumanID))
		}
	}
	for _, pending := range picture.Pending {
		h.PendingDecisions = append(h.PendingDecisions, pending.RecordType+"/"+pending.RecordID)
	}
	pendingSeverity, err := s.PendingSeverityChanges(ctx, principal, scope.IncidentID, scope.Compartment)
	if err != nil {
		return ledger.Record{}, err
	}
	for _, pending := range pendingSeverity {
		h.PendingDecisions = append(h.PendingDecisions, pending.RecordType+"/"+pending.RecordID)
	}
	for _, obligation := range picture.Obligations {
		h.Obligations = append(h.Obligations, fmt.Sprintf("%s — %s", obligation.Name, obligation.Deadline.UTC().Format(time.RFC3339)))
	}
	for _, unknown := range picture.Unknowns {
		h.Unknowns = append(h.Unknowns, unknown.Uncertainty.Statement)
	}
	record, err := domain.RecordHandover(scope, recordID, principal.HumanID, now, h)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendContinuity(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s ContinuityService) RecordResourceRequest(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, value domain.ResourceRequest, now time.Time) (ledger.Record, error) {
	if d := authz.Authorise(principal, authz.ActionRecord, authz.ObjectScope{IncidentID: scope.IncidentID, Compartment: scope.Compartment}); !d.Allowed {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	record, err := domain.RecordResourceRequest(scope, recordID, principal.HumanID, now, value)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendContinuity(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s ContinuityService) FirstReports(ctx context.Context, principal authz.Principal) ([]domain.FirstReportView, error) {
	if !principal.Authenticated || principal.HumanID == "" || (principal.Role != authz.RoleMajor && principal.Role != authz.RoleMinor) {
		return nil, ErrAuthorisationDenied
	}
	return s.Backend.FirstReports(ctx)
}

func (s ContinuityService) ScopeAssessments(ctx context.Context, principal authz.Principal, incidentID, compartment string) ([]domain.ScopeAssessmentView, error) {
	if d := authz.Authorise(principal, authz.ActionRead, authz.ObjectScope{IncidentID: incidentID, Compartment: compartment}); !d.Allowed {
		return nil, ErrAuthorisationDenied
	}
	return s.Backend.ScopeAssessments(ctx, incidentID, compartment)
}

func (s ContinuityService) SeverityChanges(ctx context.Context, principal authz.Principal, incidentID, compartment string) ([]domain.SeverityChangeView, error) {
	if d := authz.Authorise(principal, authz.ActionRead, authz.ObjectScope{IncidentID: incidentID, Compartment: compartment}); !d.Allowed {
		return nil, ErrAuthorisationDenied
	}
	return s.Backend.SeverityChanges(ctx, incidentID, compartment)
}

func (s ContinuityService) PendingSeverityChanges(ctx context.Context, principal authz.Principal, incidentID, compartment string) ([]ledger.Record, error) {
	if d := authz.Authorise(principal, authz.ActionRead, authz.ObjectScope{IncidentID: incidentID, Compartment: compartment}); !d.Allowed {
		return nil, ErrAuthorisationDenied
	}
	reader, ok := s.Backend.(pendingSeverityReader)
	if !ok {
		return nil, errors.New("continuity backend does not expose pending severity changes")
	}
	return reader.PendingSeverityChanges(ctx, incidentID, compartment)
}

func (s ContinuityService) Handovers(ctx context.Context, principal authz.Principal, incidentID, compartment string) ([]domain.HandoverView, error) {
	if d := authz.Authorise(principal, authz.ActionRead, authz.ObjectScope{IncidentID: incidentID, Compartment: compartment}); !d.Allowed {
		return nil, ErrAuthorisationDenied
	}
	return s.Backend.Handovers(ctx, incidentID, compartment)
}

func (s ContinuityService) ResourceRequests(ctx context.Context, principal authz.Principal, incidentID, compartment string) ([]domain.ResourceRequestView, error) {
	if d := authz.Authorise(principal, authz.ActionRead, authz.ObjectScope{IncidentID: incidentID, Compartment: compartment}); !d.Allowed {
		return nil, ErrAuthorisationDenied
	}
	return s.Backend.ResourceRequests(ctx, incidentID, compartment)
}

func (s ContinuityService) ready() error {
	if s.Backend == nil {
		return errors.New("continuity backend is required")
	}
	return nil
}

func cleanStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}
