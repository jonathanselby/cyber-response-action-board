package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/data/postgres"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/presentation/web"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

func (r *operatorRuntime) Intake(ctx context.Context, token string) (web.IntakeView, bool, error) {
	principal, ok, err := r.principal(ctx, token)
	if err != nil || !ok {
		return web.IntakeView{}, ok, err
	}
	service, err := r.continuityService()
	if err != nil {
		return web.IntakeView{}, false, err
	}
	reports, err := service.FirstReports(ctx, principal)
	if err != nil {
		return web.IntakeView{}, false, err
	}
	out := web.IntakeView{HumanID: principal.HumanID, Role: string(principal.Role), CanTriage: principal.Role == authz.RoleMajor}
	for _, value := range reports {
		row := web.IntakeReportRow{
			ReportID:        value.ReportID,
			Reporter:        value.Report.Reporter,
			ContactRoute:    value.Report.ContactRoute,
			ReceivedAt:      value.Report.ReceivedAt.UTC().Format(displayTime),
			Description:     value.Report.Description,
			Affected:        value.Report.Affected,
			ImmediateImpact: value.Report.ImmediateImpact,
			SourceChannel:   value.Report.SourceChannel,
			TriageStatus:    value.Report.TriageStatus,
			AttachmentCount: len(value.Report.Attachments),
			Maker:           value.Maker,
			RecordedAt:      value.RecordedAt.UTC().Format(displayTime),
		}
		if value.Disposition != nil {
			row.Decision = value.Disposition.Decision
			row.IncidentID = value.Disposition.IncidentID
			row.Reason = value.Disposition.Reason
		}
		out.Reports = append(out.Reports, row)
	}
	return out, true, nil
}

func (r *operatorRuntime) CaptureFirstReport(ctx context.Context, token string, input web.FirstReportCapture) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	now := time.Now().UTC()
	receivedAt := input.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = now
	}
	confidence := strings.TrimSpace(input.TimeConfidence)
	if confidence == "" {
		// No stated confidence: CRAB asserts only what its own clock supports.
		confidence = r.kernel.Clock().Read().Status
	}
	attachments := make([]app.FirstReportAttachmentInput, 0, len(input.Attachments))
	for _, attachment := range input.Attachments {
		attachments = append(attachments, app.FirstReportAttachmentInput{Filename: attachment.Filename, DeclaredMediaType: attachment.MediaType, Reader: attachment.Reader})
	}
	service, err := r.continuityService()
	if err != nil {
		return err
	}
	_, err = service.CaptureFirstReport(ctx, principal, newOperatorID("REPORT"), domain.FirstReport{
		Reporter:        strings.TrimSpace(input.Reporter),
		ContactRoute:    strings.TrimSpace(input.ContactRoute),
		ReceivedAt:      receivedAt,
		TimeConfidence:  confidence,
		Description:     strings.TrimSpace(input.Description),
		Affected:        strings.TrimSpace(input.Affected),
		ImmediateImpact: strings.TrimSpace(input.ImmediateImpact),
		SourceChannel:   strings.TrimSpace(input.SourceChannel),
		TriageStatus:    "new",
	}, attachments, now)
	return err
}

func (r *operatorRuntime) TriageFirstReport(ctx context.Context, token, reportID, decision, incidentID, reason string) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok || principal.Role != authz.RoleMajor {
		return app.ErrAuthorisationDenied
	}
	service, err := r.continuityService()
	if err != nil {
		return err
	}
	_, err = service.TriageFirstReport(ctx, principal, newOperatorID("TRIAGE"), domain.FirstReportDisposition{
		ReportID:   strings.TrimSpace(reportID),
		Decision:   strings.TrimSpace(decision),
		IncidentID: strings.TrimSpace(incidentID),
		Reason:     strings.TrimSpace(reason),
	}, time.Now().UTC())
	return err
}

func (r *operatorRuntime) Operations(ctx context.Context, token, incidentID string) (web.OperationsView, bool, error) {
	principal, _, err := r.scope(ctx, token, incidentID)
	if err != nil {
		if errors.Is(err, app.ErrAuthorisationDenied) {
			return web.OperationsView{}, false, nil
		}
		return web.OperationsView{}, false, err
	}
	const compartment = "GENERAL"
	service, err := r.continuityService()
	if err != nil {
		return web.OperationsView{}, false, err
	}
	out := web.OperationsView{}

	scopes, err := service.ScopeAssessments(ctx, principal, incidentID, compartment)
	if err != nil {
		return web.OperationsView{}, false, err
	}
	for _, value := range scopes {
		out.Scopes = append(out.Scopes, web.ScopeAssessmentRow{
			AffectedAssets:        strings.Join(value.Assessment.AffectedAssets, " · "),
			AffectedServices:      strings.Join(value.Assessment.AffectedServices, " · "),
			PeopleDataPopulations: strings.Join(value.Assessment.PeopleDataPopulations, " · "),
			Locations:             strings.Join(value.Assessment.Locations, " · "),
			Suppliers:             strings.Join(value.Assessment.Suppliers, " · "),
			Impact:                value.Assessment.Impact,
			Confidence:            value.Assessment.Confidence,
			ReasonEvidence:        value.Assessment.ReasonEvidence,
			Maker:                 value.Maker,
			RecordedAt:            value.RecordedAt.UTC().Format(displayTime),
		})
	}

	severity, err := service.SeverityChanges(ctx, principal, incidentID, compartment)
	if err != nil {
		return web.OperationsView{}, false, err
	}
	for _, value := range severity {
		out.SeverityHistory = append(out.SeverityHistory, web.SeverityChangeRow{Severity: value.Change.Severity, ReasonEvidence: value.Change.ReasonEvidence, Maker: value.Maker, Confirmer: value.Confirmer, RecordedAt: value.ConfirmedAt.UTC().Format(displayTime)})
	}

	pending, err := service.PendingSeverityChanges(ctx, principal, incidentID, compartment)
	if err != nil {
		return web.OperationsView{}, false, err
	}
	for _, proposal := range pending {
		change, err := domain.DecodePayload[domain.SeverityChange](proposal)
		if err != nil {
			return web.OperationsView{}, false, err
		}
		version, err := domain.ProposalVersion(proposal)
		if err != nil {
			return web.OperationsView{}, false, err
		}
		out.PendingSeverity = append(out.PendingSeverity, web.PendingSeverityRow{RecordID: proposal.RecordID, Version: version, Maker: proposal.Maker, Severity: change.Severity, ReasonEvidence: change.ReasonEvidence, ProposedAt: proposal.Timestamp.UTC().Format(displayTime)})
	}

	handovers, err := service.Handovers(ctx, principal, incidentID, compartment)
	if err != nil {
		return web.OperationsView{}, false, err
	}
	for _, value := range handovers {
		out.Handovers = append(out.Handovers, web.HandoverRow{
			Situation:                value.Handover.Situation,
			OperationalRoles:         strings.Join(value.Handover.OperationalRoles, " · "),
			CriticalActions:          strings.Join(value.Handover.CriticalActions, " · "),
			PendingDecisions:         strings.Join(value.Handover.PendingDecisions, " · "),
			Obligations:              strings.Join(value.Handover.Obligations, " · "),
			Unknowns:                 strings.Join(value.Handover.Unknowns, " · "),
			NextObjectives:           strings.Join(value.Handover.NextObjectives, " · "),
			CommunicationCommitments: strings.Join(value.Handover.CommunicationCommitments, " · "),
			Maker:                    value.Maker,
			RecordedAt:               value.RecordedAt.UTC().Format(displayTime),
		})
	}

	resources, err := service.ResourceRequests(ctx, principal, incidentID, compartment)
	if err != nil {
		return web.OperationsView{}, false, err
	}
	for _, value := range resources {
		out.Resources = append(out.Resources, web.ResourceRequestRow{
			RecordID:        value.RecordID,
			Category:        value.Request.Category,
			Request:         value.Request.Request,
			OwnerHumanID:    value.Request.OwnerHumanID,
			Priority:        value.Request.Priority,
			Status:          value.Request.Status,
			AuthoritySource: value.Request.AuthoritySource,
			Notes:           value.Request.Notes,
			Maker:           value.Maker,
			RecordedAt:      value.RecordedAt.UTC().Format(displayTime),
		})
	}
	return out, true, nil
}

func (r *operatorRuntime) RecordScopeAssessment(ctx context.Context, token, incidentID string, input web.ScopeAssessmentInput) error {
	principal, picture, err := r.scope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	service, err := r.continuityService()
	if err != nil {
		return err
	}
	_, err = service.RecordScopeAssessment(ctx, principal, domain.Scope{IncidentID: incidentID, Mode: picture.Incident.Mode, Compartment: "GENERAL"}, newOperatorID("SCOPE"), domain.ScopeAssessment{
		AffectedAssets:        splitOperatorList(input.AffectedAssets),
		AffectedServices:      splitOperatorList(input.AffectedServices),
		PeopleDataPopulations: splitOperatorList(input.PeopleDataPopulations),
		Locations:             splitOperatorList(input.Locations),
		Suppliers:             splitOperatorList(input.Suppliers),
		Impact:                strings.TrimSpace(input.Impact),
		Confidence:            strings.TrimSpace(input.Confidence),
		ReasonEvidence:        strings.TrimSpace(input.ReasonEvidence),
	}, time.Now().UTC())
	return err
}

func (r *operatorRuntime) ProposeSeverityChange(ctx context.Context, token, incidentID, severity, reasonEvidence string) error {
	principal, picture, err := r.scope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	service, err := r.continuityService()
	if err != nil {
		return err
	}
	_, err = service.ProposeSeverityChange(ctx, principal, domain.Scope{IncidentID: incidentID, Mode: picture.Incident.Mode, Compartment: "GENERAL"}, newOperatorID("SEVERITY"), domain.SeverityChange{Severity: strings.TrimSpace(severity), ReasonEvidence: strings.TrimSpace(reasonEvidence)}, time.Now().UTC())
	return err
}

func (r *operatorRuntime) DecideSeverity(ctx context.Context, token, incidentID, recordID, version, outcome string) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	service, err := r.continuityService()
	if err != nil {
		return err
	}
	_, err = service.DecideSeverity(ctx, principal, incidentID, strings.TrimSpace(recordID), strings.TrimSpace(version), strings.TrimSpace(outcome), time.Now().UTC())
	return err
}

func (r *operatorRuntime) GenerateHandover(ctx context.Context, token, incidentID string, nextObjectives, communicationCommitments []string) error {
	principal, picture, err := r.scope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	service, err := r.continuityService()
	if err != nil {
		return err
	}
	_, err = service.GenerateHandover(ctx, principal, domain.Scope{IncidentID: incidentID, Mode: picture.Incident.Mode, Compartment: "GENERAL"}, newOperatorID("HANDOVER"), nextObjectives, communicationCommitments, time.Now().UTC())
	return err
}

func (r *operatorRuntime) RecordResourceRequest(ctx context.Context, token, incidentID string, input web.ResourceRequestInput) error {
	principal, picture, err := r.scope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	service, err := r.continuityService()
	if err != nil {
		return err
	}
	recordID := strings.TrimSpace(input.RecordID)
	if recordID == "" {
		recordID = newOperatorID("RESOURCE")
	}
	_, err = service.RecordResourceRequest(ctx, principal, domain.Scope{IncidentID: incidentID, Mode: picture.Incident.Mode, Compartment: "GENERAL"}, recordID, domain.ResourceRequest{
		Category:        strings.TrimSpace(input.Category),
		Request:         strings.TrimSpace(input.Request),
		OwnerHumanID:    strings.TrimSpace(input.OwnerHumanID),
		Priority:        strings.TrimSpace(input.Priority),
		Status:          strings.TrimSpace(input.Status),
		AuthoritySource: strings.TrimSpace(input.AuthoritySource),
		Notes:           strings.TrimSpace(input.Notes),
	}, time.Now().UTC())
	return err
}

func (r *operatorRuntime) continuityService() (app.ContinuityService, error) {
	evidenceBackend, ok := r.evidence.Backend.(*postgres.EvidenceRuntime)
	if !ok || evidenceBackend.Blobs == nil {
		return app.ContinuityService{}, errors.New("operator evidence boundary is unavailable")
	}
	backend, err := postgres.NewContinuityRuntimeFromKernel(r.kernel, r.signer, evidenceBackend.Blobs)
	if err != nil {
		return app.ContinuityService{}, err
	}
	return app.ContinuityService{Backend: backend, Incident: r.incident, Severity: r.organisation, Window: r.organisation}, nil
}

func splitOperatorList(raw string) []string {
	values := strings.FieldsFunc(raw, func(r rune) bool { return r == '\n' || r == '\r' || r == ',' })
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}
