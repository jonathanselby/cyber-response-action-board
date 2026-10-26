package main

// Slice 15 browser runtime: composition of existing application services for
// the go-app-shell presentation tier. Every method authorises through the
// application tier; nothing here introduces a new mechanism or state store.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/config"
	"github.com/jonathanselby/cyber-response-action-board/internal/data/postgres"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/presentation/web"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

// displayTime is the operator-facing time format for browser views. The JSON
// API keeps RFC 3339.
const displayTime = "2 Jan 2006 15:04 UTC"

// ---- frame -------------------------------------------------------------------

func (r *operatorRuntime) Frame(ctx context.Context, token, incidentID string) (web.FrameView, bool, error) {
	session, ok, err := r.sessions.Resolve(ctx, token)
	if err != nil || !ok {
		return web.FrameView{}, false, err
	}
	principal, err := r.access.Principal(ctx, session.HumanID, true)
	if err != nil {
		return web.FrameView{}, false, err
	}
	frame := web.FrameView{HumanID: principal.HumanID, Role: string(principal.Role), Custodian: principal.Custodian}
	frame.Status.BreakGlass = session.BreakGlass
	frame.Status.TimeStatus = r.kernel.Clock().Read().Status
	frame.Status.IntegrityFailed = r.kernel.IntegrityFailed()
	if isolate, err := r.access.IsolateMode(ctx); err == nil {
		frame.Status.Isolate = isolate
	}
	if cfg, err := config.Load(); err == nil {
		frame.Status.PackStale = packFreshnessCheck(cfg, r.emergencyChangedAt(ctx)).Status != "pass"
	}
	if approvals, err := r.approvals(ctx, principal, ""); err == nil {
		frame.Status.PendingApprovals = len(approvals.Waiting)
	}
	if incidentID = strings.TrimSpace(incidentID); incidentID != "" {
		frame.Incident = r.incidentContext(ctx, principal, incidentID)
	}
	return frame, true, nil
}

// incidentContext returns nil when the incident is not authorised, so the
// frame never names an incident the user cannot see.
func (r *operatorRuntime) incidentContext(ctx context.Context, principal authz.Principal, incidentID string) *web.IncidentContext {
	picture, found, err := r.incident.CommonOperatingPicture(ctx, principal, incidentID, "GENERAL")
	if err != nil || !found {
		return nil
	}
	out := &web.IncidentContext{ID: incidentID, Title: picture.Incident.Title, Mode: picture.Incident.Mode, Phase: humanPhase(domain.PhaseActiveResponse), Compartments: readableCompartments(principal, incidentID)}
	if state, found, err := r.recovery.State(ctx, principal, incidentID, "GENERAL"); err == nil && found && state.Phase != "" {
		out.Phase = humanPhase(state.Phase)
	}
	return out
}

func humanPhase(phase string) string {
	phase = strings.ToLower(strings.TrimSpace(phase))
	if phase == "" {
		return ""
	}
	return strings.ToUpper(phase[:1]) + phase[1:]
}

func readableCompartments(principal authz.Principal, incidentID string) []string {
	out := []string{"GENERAL"}
	var restricted []string
	for name, allowed := range principal.Compartments[incidentID] {
		if allowed && name != "GENERAL" {
			restricted = append(restricted, name)
		}
	}
	sort.Strings(restricted)
	return append(out, restricted...)
}

// ---- board -------------------------------------------------------------------

func (r *operatorRuntime) Board(ctx context.Context, token, incidentID, compartment string) (web.BoardView, error) {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return web.BoardView{}, err
	}
	if !ok {
		return web.BoardView{}, app.ErrAuthorisationDenied
	}
	if compartment = strings.TrimSpace(compartment); compartment == "" {
		compartment = "GENERAL"
	}
	picture, found, err := r.incident.CommonOperatingPicture(ctx, principal, incidentID, compartment)
	if err != nil {
		return web.BoardView{}, err
	}
	if !found {
		return web.BoardView{}, app.ErrAuthorisationDenied
	}
	out := web.BoardView{Picture: picture, Compartment: compartment, Clocks: picture.Obligations, Phase: humanPhase(domain.PhaseActiveResponse)}
	sort.SliceStable(out.Clocks, func(i, j int) bool { return out.Clocks[i].Deadline.Before(out.Clocks[j].Deadline) })
	sort.SliceStable(out.Picture.Timeline, func(i, j int) bool {
		return out.Picture.Timeline[i].RecordedAt.After(out.Picture.Timeline[j].RecordedAt)
	})
	sort.SliceStable(out.Picture.Decisions, func(i, j int) bool {
		return out.Picture.Decisions[i].ConfirmedAt.After(out.Picture.Decisions[j].ConfirmedAt)
	})
	if state, found, err := r.recovery.State(ctx, principal, incidentID, compartment); err == nil && found && state.Phase != "" {
		out.Phase = humanPhase(state.Phase)
	}

	if service, err := r.continuityService(); err == nil {
		if scopes, err := service.ScopeAssessments(ctx, principal, incidentID, compartment); err == nil && len(scopes) > 0 {
			latest := scopes[0]
			for _, v := range scopes {
				if v.RecordedAt.After(latest.RecordedAt) {
					latest = v
				}
			}
			out.LatestScope = &latest
		}
		if history, err := service.SeverityChanges(ctx, principal, incidentID, compartment); err == nil {
			sort.SliceStable(history, func(i, j int) bool { return history[i].ConfirmedAt.After(history[j].ConfirmedAt) })
			out.SeverityHistory = history
		}
		if handovers, err := service.Handovers(ctx, principal, incidentID, compartment); err == nil && len(handovers) > 0 {
			latest := handovers[0]
			for _, v := range handovers {
				if v.RecordedAt.After(latest.RecordedAt) {
					latest = v
				}
			}
			out.LatestHandover = &latest
		}
		if resources, err := service.ResourceRequests(ctx, principal, incidentID, compartment); err == nil {
			out.Resources = resources
		}
	}
	out.SeverityValues = r.severityLabels(ctx)
	if related, err := r.relationships.Visible(ctx, principal, incidentID); err == nil {
		for _, v := range related {
			view := web.RelatedIncidentView{IncidentID: v.RelatedIncidentID, Reason: v.Reason, Maker: v.Maker, Confirmer: v.Confirmer, ConfirmedAt: v.ConfirmedAt.UTC().Format(displayTime)}
			if picture, found, err := r.incident.CommonOperatingPicture(ctx, principal, v.RelatedIncidentID, "GENERAL"); err == nil && found {
				view.Title = picture.Incident.Title
			}
			out.Related = append(out.Related, view)
		}
	}
	for id := range principal.Incidents {
		if id == incidentID || !principal.Compartments[id]["GENERAL"] {
			continue
		}
		if picture, found, err := r.incident.CommonOperatingPicture(ctx, principal, id, "GENERAL"); err == nil && found {
			out.RelateCandidates = append(out.RelateCandidates, web.IncidentLink{IncidentID: id, Title: picture.Incident.Title})
		}
	}
	sort.Slice(out.RelateCandidates, func(i, j int) bool { return out.RelateCandidates[i].IncidentID < out.RelateCandidates[j].IncidentID })
	if templates, err := r.organisation.ChecklistTemplates(ctx, principal); err == nil {
		for _, t := range templates {
			if t.Active {
				out.Checklists = append(out.Checklists, web.ChecklistOption{TemplateID: t.TemplateID, Name: t.Name, SourceRef: t.SourceRef, Steps: len(t.Steps)})
			}
		}
	}

	if store, err := postgres.NewCommunicationStoreFromKernel(r.kernel); err == nil {
		if cadences, err := store.Cadences(ctx, incidentID, compartment); err == nil {
			var next time.Time
			for _, c := range cadences {
				if next.IsZero() || c.NextDueAt.Before(next) {
					next = c.NextDueAt
				}
			}
			if !next.IsZero() {
				out.NextReportDue = next.UTC().Format(displayTime)
			}
		}
		if enquiries, err := store.IncomingEnquiries(ctx, incidentID, compartment); err == nil {
			out.OpenEnquiries = len(enquiries)
		}
		if issued, err := store.Issued(ctx, incidentID, compartment); err == nil {
			cutoff := time.Now().UTC().Add(-24 * time.Hour)
			for _, v := range issued {
				if v.IssuedAt.After(cutoff) {
					out.IssuedRecently = append(out.IssuedRecently, v)
				}
			}
		}
	}

	if view, err := r.access.AdministrationView(ctx, time.Now().UTC()); err == nil {
		seen := map[string]bool{}
		for _, m := range view.IncidentMemberships {
			if m.IncidentID == incidentID && m.Status == "active" && !seen[m.HumanID] {
				seen[m.HumanID] = true
				out.Members = append(out.Members, m.HumanID)
			}
		}
		sort.Strings(out.Members)
	}

	if cfg, err := config.Load(); err == nil {
		for _, check := range []app.Check{timeCheck(r.kernel.Clock()), packFreshnessCheck(cfg, r.emergencyChangedAt(ctx)), exportPathCheck(cfg), storageCheck(cfg)} {
			if check.Status != "pass" {
				out.Readiness = append(out.Readiness, web.ReadinessCheckView{Name: humanCheckName(check.Name), Status: check.Status, Message: check.Message})
			}
		}
	}
	if r.kernel.IntegrityFailed() {
		out.Readiness = append(out.Readiness, web.ReadinessCheckView{Name: "Ledger integrity", Status: "fail", Message: "ledger verification failed; CRAB is read-only until recovered"})
	}
	return out, nil
}

func humanCheckName(name string) string {
	switch name {
	case "emergency_pack":
		return "Paper pack"
	case "export_paths":
		return "Export locations"
	case "storage_headroom":
		return "Storage"
	case "time_status":
		return "Time"
	}
	name = strings.ReplaceAll(name, "_", " ")
	return strings.ToUpper(name[:1]) + name[1:]
}

// severityLabels returns the organisation's confirmed scale, least severe
// first. Responders choose only from it (T-202).
func (r *operatorRuntime) severityLabels(ctx context.Context) []string {
	scale, found, err := r.organisation.SeverityScale(ctx)
	if err != nil || !found {
		return nil
	}
	return scale.Labels()
}

// ---- incident writes ----------------------------------------------------------

func (r *operatorRuntime) generalScope(ctx context.Context, token, incidentID string) (authz.Principal, domain.Scope, error) {
	principal, picture, err := r.scope(ctx, token, incidentID)
	if err != nil {
		return authz.Principal{}, domain.Scope{}, err
	}
	return principal, domain.Scope{IncidentID: incidentID, Mode: picture.Incident.Mode, Compartment: "GENERAL"}, nil
}

func (r *operatorRuntime) RecordUncertainty(ctx context.Context, token, incidentID, kind, statement string) error {
	principal, scope, err := r.generalScope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	_, err = r.incident.RecordUncertainty(ctx, principal, scope, newOperatorID("UNC"), domain.Uncertainty{Kind: kind, Statement: statement}, time.Now().UTC())
	return err
}

func (r *operatorRuntime) ProposeDecision(ctx context.Context, token, incidentID, decision, rationale string) error {
	principal, scope, err := r.generalScope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	_, err = r.incident.ProposeDecision(ctx, principal, scope, newOperatorID("DEC"), domain.Decision{Decision: decision, Rationale: rationale}, time.Now().UTC())
	return err
}

func (r *operatorRuntime) ProposeRoleAssignment(ctx context.Context, token, incidentID, roleName, humanID string, deputy bool) error {
	principal, scope, err := r.generalScope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	_, err = r.incident.ProposeOperationalRoleAssignment(ctx, principal, scope, newOperatorID("ROLE"), domain.OperationalRoleAssignment{RoleName: roleName, HumanID: humanID, Deputy: deputy}, time.Now().UTC())
	return err
}

// ProposeActionStatus proposes a new governed version of an existing action.
func (r *operatorRuntime) ProposeActionStatus(ctx context.Context, token, incidentID, actionID, status, skipReason string) error {
	principal, picture, err := r.scope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	for _, current := range picture.Actions {
		if current.RecordID != actionID {
			continue
		}
		next := current.Action
		next.Status = status
		next.SkipReason = ""
		if status == domain.ActionStatusSkipped {
			next.SkipReason = skipReason
		}
		scope := domain.Scope{IncidentID: incidentID, Mode: picture.Incident.Mode, Compartment: current.Compartment}
		_, err = r.incident.ProposeAction(ctx, principal, scope, actionID, next, time.Now().UTC())
		return err
	}
	return errors.New("action not found")
}

func (r *operatorRuntime) Retrieve(ctx context.Context, token, incidentID, question string) (web.GuidanceView, error) {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return web.GuidanceView{}, err
	}
	if !ok {
		return web.GuidanceView{}, app.ErrAuthorisationDenied
	}
	if r.retrieval.Backend == nil {
		return web.GuidanceView{Question: question, Refused: true, Reason: "Runbook search is not available in this deployment."}, nil
	}
	result, err := r.retrieval.Retrieve(ctx, principal, incidentID, "GENERAL", question, 5)
	if err != nil {
		return web.GuidanceView{}, err
	}
	return web.GuidanceView{Question: question, Refused: result.Refused, Reason: result.Reason, Citations: result.Citations}, nil
}

// ---- close-out ----------------------------------------------------------------

func (r *operatorRuntime) CloseOut(ctx context.Context, token, incidentID string) (web.CloseOutView, error) {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return web.CloseOutView{}, err
	}
	if !ok {
		return web.CloseOutView{}, app.ErrAuthorisationDenied
	}
	state, found, err := r.recovery.State(ctx, principal, incidentID, "GENERAL")
	if err != nil {
		return web.CloseOutView{}, err
	}
	if !found {
		state = domain.RecoveryState{IncidentID: incidentID, Phase: domain.PhaseActiveResponse}
	}
	out := web.CloseOutView{IncidentID: incidentID, State: state}
	if state.StandDown != nil && state.PIR == nil {
		if draft, err := r.recovery.GeneratePIR(ctx, principal, incidentID, "GENERAL"); err == nil {
			out.PIRDraft = &draft
		}
	}
	if pending, err := r.kernel.PendingRecoveryProposals(ctx, incidentID, "GENERAL"); err == nil {
		out.Pending = len(pending)
	}
	return out, nil
}

func (r *operatorRuntime) RecordRecoveryUpdate(ctx context.Context, token, incidentID string, input web.RecoveryUpdateInput) error {
	principal, scope, err := r.generalScope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	value := domain.RecoveryUpdate{ItemID: newOperatorID("ITEM"), Summary: input.Summary, OwnerHumanID: input.OwnerHumanID, RestorationPriority: input.RestorationPriority, Verified: input.Verified, ResidualRisk: input.ResidualRisk, HandoffRef: input.HandoffRef}
	_, err = r.recovery.RecordRecoveryUpdate(ctx, principal, scope, newOperatorID("RECOV"), value, time.Now().UTC())
	return err
}

func (r *operatorRuntime) ProposeStandDown(ctx context.Context, token, incidentID, reason string) error {
	principal, scope, err := r.generalScope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	_, err = r.recovery.ProposeStandDown(ctx, principal, scope, newOperatorID("STANDDOWN"), domain.StandDown{Reason: reason}, time.Now().UTC())
	return err
}

func (r *operatorRuntime) ProposePIR(ctx context.Context, token, incidentID, summary string, lessons, longTermWork []string) error {
	principal, scope, err := r.generalScope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	_, err = r.recovery.ProposePIR(ctx, principal, scope, newOperatorID("PIR"), domain.PIR{ExecutiveSummary: summary, HumanLessons: lessons, LongTermWork: longTermWork}, time.Now().UTC())
	return err
}

func (r *operatorRuntime) ProposeRetention(ctx context.Context, token, incidentID string, input web.RetentionInput) error {
	principal, scope, err := r.generalScope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	value := domain.RetentionDisposition{RetentionClass: input.RetentionClass, Hold: input.Hold, HoldReason: input.HoldReason, DispositionEligibleAfter: input.DispositionEligibleAfter, HandoffRef: input.HandoffRef}
	_, err = r.recovery.ProposeRetention(ctx, principal, scope, newOperatorID("RETAIN"), value, time.Now().UTC())
	return err
}

func (r *operatorRuntime) ProposeClosure(ctx context.Context, token, incidentID string, input web.ClosureInput) error {
	principal, scope, err := r.generalScope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	value := domain.Closure{Reason: input.Reason, PrivilegedCompartment: input.PrivilegedCompartment, PrivilegedSummary: input.PrivilegedSummary, ShareableSummary: input.ShareableSummary}
	_, err = r.recovery.ProposeClosure(ctx, principal, scope, newOperatorID("CLOSE"), value, time.Now().UTC())
	return err
}

func (r *operatorRuntime) TranscribePaper(ctx context.Context, token, incidentID string, input web.PaperInput) error {
	principal, scope, err := r.generalScope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	value := domain.PaperTranscription{Statement: input.Statement, PaperOccurredAt: input.PaperOccurredAt, SourceReference: input.SourceReference, TimeConfidence: input.TimeConfidence}
	_, err = r.incident.RecordPaperTranscription(ctx, principal, scope, newOperatorID("PAPER"), value, time.Now().UTC())
	return err
}

// ---- paper --------------------------------------------------------------------

// capturedArtifacts keeps the normal atomic last-good file behaviour and also
// returns the rendered bytes so the browser can download the same artefact.
type capturedArtifacts struct {
	inner app.ArtifactStore
	bytes []byte
}

func (c *capturedArtifacts) WriteArtifact(name string, b []byte) (app.ArtifactWriteResult, error) {
	result, err := c.inner.WriteArtifact(name, b)
	if err != nil {
		return result, err
	}
	c.bytes = append([]byte(nil), b...)
	return result, nil
}

func (r *operatorRuntime) printService() (app.PrintService, *capturedArtifacts, string, error) {
	if r.print.Artifacts == nil {
		return app.PrintService{}, nil, "", errors.New("paper output is not configured: CRAB_EXPORT_DIR is required")
	}
	capture := &capturedArtifacts{inner: r.print.Artifacts}
	service := r.print
	service.Artifacts = capture
	return service, capture, r.pageSizeDefault, nil
}

func (r *operatorRuntime) IncidentBriefPDF(ctx context.Context, token, incidentID, pageSize, printMode string) ([]byte, error) {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, app.ErrAuthorisationDenied
	}
	service, capture, defaultSize, err := r.printService()
	if err != nil {
		return nil, err
	}
	if pageSize == "" {
		pageSize = defaultSize
	}
	if _, err := service.GenerateIncidentBrief(ctx, principal, incidentID, "GENERAL", nil, pageSize, printMode, time.Now().UTC()); err != nil {
		return nil, err
	}
	return capture.bytes, nil
}

func (r *operatorRuntime) EmergencyPackPDF(ctx context.Context, token, incidentID, pageSize, printMode string) ([]byte, error) {
	_, picture, err := r.scope(ctx, token, incidentID)
	if err != nil {
		return nil, err
	}
	return r.emergencyPackPDF(ctx, token, picture.Incident.Mode, pageSize, printMode)
}

// OrganisationPackPDF generates the organisation Emergency Response Pack,
// which is not tied to any incident (UAT 2 notes 2.4).
func (r *operatorRuntime) OrganisationPackPDF(ctx context.Context, token, pageSize, printMode string) ([]byte, error) {
	return r.emergencyPackPDF(ctx, token, "live", pageSize, printMode)
}

func (r *operatorRuntime) emergencyPackPDF(ctx context.Context, token, mode, pageSize, printMode string) ([]byte, error) {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, app.ErrAuthorisationDenied
	}
	service, capture, defaultSize, err := r.printService()
	if err != nil {
		return nil, err
	}
	if pageSize == "" {
		pageSize = defaultSize
	}
	pack, err := r.emergencyPack(ctx, mode)
	if err != nil {
		return nil, err
	}
	if _, err := service.GenerateEmergencyResponsePack(ctx, principal, pack, pageSize, printMode, r.kernel.Clock().Read().Now); err != nil {
		return nil, err
	}
	return capture.bytes, nil
}

// emergencyPack composes the pack content. Organisation emergency
// configuration replaces the generic sections once it is confirmed.
func (r *operatorRuntime) emergencyPack(_ context.Context, mode string) (domain.EmergencyResponsePack, error) {
	return domain.EmergencyResponsePack{
		Organisation: "CRAB organisation",
		Mode:         mode,
		Sections: []domain.PackSection{
			{Title: "Operating rule", Body: "The model proposes. One person makes. Another confirms. Nothing is erased."},
			{Title: "Degraded operation", Body: "If live CRAB or PostgreSQL is unavailable, use these paper forms. Preserve original occurrence times and source references. Reconcile named-human transcriptions into CRAB after restoration; do not backdate the digital ledger."},
			{Title: "Authority", Body: "Paper operation does not create new authority. Existing incident roles, compartment boundaries and maker-checker responsibilities continue to apply."},
		},
		Forms: domain.DefaultContingencyForms(),
	}, nil
}

// ---- approvals ----------------------------------------------------------------

type approvalSource struct {
	kind, scopeID, recordType, recordID, compartment, version, maker string
	proposedAt                                                       time.Time
}

func (r *operatorRuntime) Approvals(ctx context.Context, token, incidentID string) (web.ApprovalsView, error) {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return web.ApprovalsView{}, err
	}
	if !ok {
		return web.ApprovalsView{}, app.ErrAuthorisationDenied
	}
	view, err := r.approvals(ctx, principal, strings.TrimSpace(incidentID))
	if err != nil {
		return web.ApprovalsView{}, err
	}
	view.Filter = strings.TrimSpace(incidentID)
	return view, nil
}

// approvals gathers pending governed changes only through the same scoped
// listings each area already uses, then loads each proposal's payload.
func (r *operatorRuntime) approvals(ctx context.Context, principal authz.Principal, filter string) (web.ApprovalsView, error) {
	var sources []approvalSource
	seen := map[string]bool{}
	add := func(s approvalSource) {
		key := s.kind + "|" + s.scopeID + "|" + s.recordType + "|" + s.recordID
		if !seen[key] {
			seen[key] = true
			sources = append(sources, s)
		}
	}
	titles := map[string]string{}
	var incidents []web.IncidentLink

	ids := make([]string, 0, len(principal.Incidents))
	for id, member := range principal.Incidents {
		if member {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	obligations, obligationStore, obligationErr := r.obligationService()
	_ = obligations
	communicationStore, communicationErr := postgres.NewCommunicationStoreFromKernel(r.kernel)
	continuity, continuityErr := r.continuityService()

	for _, id := range ids {
		picture, found, err := r.incident.CommonOperatingPicture(ctx, principal, id, "GENERAL")
		if err != nil || !found {
			continue
		}
		titles[id] = picture.Incident.Title
		incidents = append(incidents, web.IncidentLink{IncidentID: id, Title: picture.Incident.Title, Mode: picture.Incident.Mode, Severity: picture.Incident.Severity})
		if filter != "" && filter != id {
			continue
		}
		for _, compartment := range readableCompartments(principal, id) {
			scoped := picture
			if compartment != "GENERAL" {
				if p, ok, err := r.incident.CommonOperatingPicture(ctx, principal, id, compartment); err == nil && ok {
					scoped = p
				}
			}
			for _, p := range scoped.Pending {
				kind := "incident"
				if p.RecordType == domain.TypeIncidentDeclaration {
					kind = "declaration"
				}
				add(approvalSource{kind, id, p.RecordType, p.RecordID, p.Compartment, p.Version, p.Maker, p.ProposedAt})
			}
			if continuityErr == nil {
				if pending, err := continuity.PendingSeverityChanges(ctx, principal, id, compartment); err == nil {
					for _, p := range pending {
						version, _ := domain.ProposalVersion(p)
						add(approvalSource{"severity", id, p.RecordType, p.RecordID, p.Compartment, version, p.Maker, p.Timestamp})
					}
				}
			}
			if obligationErr == nil {
				if pending, err := obligationStore.PendingForScope(ctx, id, compartment); err == nil {
					for _, p := range pending {
						add(approvalSource{"obligation", p.ScopeID, p.RecordType, p.RecordID, p.Compartment, p.Version, p.Maker, p.ProposedAt})
					}
				}
			}
			if communicationErr == nil {
				if pending, err := communicationStore.PendingForIncident(ctx, id, compartment); err == nil {
					for _, p := range pending {
						add(approvalSource{"communication", p.IncidentID, p.RecordType, p.RecordID, p.Compartment, p.Version, p.Maker, p.ProposedAt})
					}
				}
			}
			if pending, err := r.kernel.PendingRecoveryProposals(ctx, id, compartment); err == nil {
				for _, p := range pending {
					add(approvalSource{"recovery", p.IncidentID, p.RecordType, p.RecordID, p.Compartment, p.Version, p.Maker, p.ProposedAt})
				}
			}
		}
		if pending, err := r.relationStore.PendingRelationships(ctx, id); err == nil {
			for _, p := range pending {
				// Shown only to people who can already see both incidents.
				if principal.Incidents[p.RelatedIncidentID] && principal.Compartments[p.RelatedIncidentID]["GENERAL"] {
					add(approvalSource{"relationship", id, domain.TypeIncidentRelationship, p.RecordID, "GENERAL", p.Version, p.Maker, p.ProposedAt})
				}
			}
		}
		allowed := principal.Compartments[id]
		if pending, err := r.access.PendingAccessForIncident(ctx, id); err == nil {
			for _, p := range pending {
				if p.Compartment != "" && p.Compartment != "GENERAL" && !allowed[p.Compartment] {
					continue
				}
				add(approvalSource{"access", p.ScopeID, p.RecordType, p.RecordID, p.Compartment, p.Version, p.Maker, p.ProposedAt})
			}
		}
	}

	if filter == "" {
		if principal.Role == authz.RoleMajor {
			if declarations, err := r.incidents.PendingDeclarations(ctx); err == nil {
				for _, d := range declarations {
					titles[d.IncidentID] = d.Declaration.Title
					add(approvalSource{"declaration", d.IncidentID, domain.TypeIncidentDeclaration, d.RecordID, "GENERAL", d.Version, d.Maker, time.Time{}})
				}
			}
			if obligationErr == nil {
				if pending, err := obligationStore.PendingForScope(ctx, "", "SYSTEM"); err == nil {
					for _, p := range pending {
						add(approvalSource{"obligation", p.ScopeID, p.RecordType, p.RecordID, p.Compartment, p.Version, p.Maker, p.ProposedAt})
					}
				}
			}
		}
		if principal.Role == authz.RoleMajor {
			if pending, err := r.organisation.Pending(ctx, principal); err == nil {
				for _, p := range pending {
					add(approvalSource{"organisation", "", p.RecordType, p.RecordID, "SYSTEM", p.Version, p.Maker, p.At})
				}
			}
		}
		if principal.Custodian {
			if pending, err := r.access.PendingAccessAdministration(ctx); err == nil {
				for _, p := range pending {
					add(approvalSource{"access", p.ScopeID, p.RecordType, p.RecordID, p.Compartment, p.Version, p.Maker, p.ProposedAt})
				}
			}
		}
	}

	out := web.ApprovalsView{Incidents: incidents}
	for _, s := range sources {
		item := r.approvalView(ctx, s, titles)
		item.CanDecide = s.maker != principal.HumanID
		if item.CanDecide {
			out.Waiting = append(out.Waiting, item)
		} else {
			out.AwaitingOther = append(out.AwaitingOther, item)
		}
	}
	return out, nil
}

func (r *operatorRuntime) approvalView(ctx context.Context, s approvalSource, titles map[string]string) web.ApprovalView {
	item := web.ApprovalView{
		Kind: s.kind, Label: approvalLabel(s.recordType), Compartment: s.compartment, Maker: s.maker,
		ScopeID: s.scopeID, RecordType: s.recordType, RecordID: s.recordID, Version: s.version,
	}
	if s.kind != "obligation" || s.scopeID != "" {
		item.IncidentID = s.scopeID
		item.IncidentTitle = titles[s.scopeID]
	}
	if s.kind == "organisation" {
		if s.compartment == "SYSTEM" {
			item.Compartment = ""
		}
		if !s.proposedAt.IsZero() {
			item.ProposedAt = s.proposedAt.UTC().Format(displayTime)
		}
		if payload, found, err := r.kernel.ProposalPayload(ctx, s.kind, s.scopeID, s.recordType, s.recordID); err == nil && found {
			item.Summary, item.Consequence, item.Proposed = organisationSummary(s.recordType, payload)
		}
		return item
	}
	if !s.proposedAt.IsZero() {
		item.ProposedAt = s.proposedAt.UTC().Format(displayTime)
	}
	payload, found, err := r.kernel.ProposalPayload(ctx, s.kind, s.scopeID, s.recordType, s.recordID)
	values := map[string]any{}
	if err == nil && found {
		item.Proposed, values = payloadFields(payload)
	}
	item.Reason = firstString(values, "reason_evidence", "reason", "rationale", "applicability_reason")
	if template := firstString(values, "checklist_template_id"); template != "" {
		item.Group = "Checklist " + template + " · " + s.scopeID
	}
	item.Summary, item.Consequence = approvalSummary(s, values, item.Label)
	if s.kind == "severity" && item.IncidentID != "" {
		if current := r.currentSeverity(ctx, item.IncidentID); current != "" {
			item.Effective = []web.Field{{Name: "Severity", Value: current}}
		}
	}
	return item
}

func (r *operatorRuntime) currentSeverity(ctx context.Context, incidentID string) string {
	if r.incidents == nil {
		return ""
	}
	picture, found, err := r.incidents.CommonOperatingPicture(ctx, incidentID)
	if err != nil || !found {
		return ""
	}
	return picture.Incident.Severity
}

func approvalLabel(recordType string) string {
	labels := map[string]string{
		domain.TypeIncidentDeclaration:       "Incident declaration",
		domain.TypeDecision:                  "Decision",
		domain.TypeAction:                    "Action",
		domain.TypeOperationalRoleAssignment: "Operational role",
		domain.TypeSeverityChange:            "Severity change",
		domain.TypeCommunicationIssue:        "Communication issue",
		domain.TypeCommunicationCadence:      "Report cadence",
		domain.TypeObligationDefinition:      "Obligation definition",
		domain.TypeObligationTrigger:         "Obligation clock",
		domain.TypeObligationChange:          "Obligation change",
		domain.TypeStandDown:                 "Stand-down",
		domain.TypePIR:                       "Post-Incident Review",
		domain.TypeRetention:                 "Retention and hold",
		domain.TypeClosure:                   "Incident closure",
		"security.incident_membership":       "Incident access",
		"security.compartment_membership":    "Compartment read-in",
		"security.incident_sponsor":          "Incident sponsor",
		"security.compartment_controller":    "Compartment controller",
		"security.isolate_mode":              "Isolate mode",
		"security.output_permission":         "Print or download permission",
		domain.TypeSeverityScale:             "Severity scale",
		domain.TypeIncidentRelationship:      "Related incident",
		domain.TypeEmergencyConfiguration:    "Emergency information",
		domain.TypeChecklistTemplate:         "Checklist template",
		domain.TypeSinglePersonWindow:        "Single-person window",
	}
	if label, ok := labels[recordType]; ok {
		return label
	}
	name := recordType
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	name = strings.ReplaceAll(name, "_", " ")
	if name == "" {
		return "Change"
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

func approvalSummary(s approvalSource, v map[string]any, label string) (string, string) {
	str := func(keys ...string) string { return firstString(v, keys...) }
	switch s.recordType {
	case domain.TypeIncidentDeclaration:
		return "Declare incident: " + str("title"), "The incident becomes active and its declared severity takes effect."
	case domain.TypeSeverityChange:
		return "Change severity to " + str("severity"), "Severity becomes " + str("severity") + " on the Board, brief and handover."
	case domain.TypeDecision:
		return str("decision"), "The decision is recorded as confirmed."
	case domain.TypeAction:
		status := str("status")
		if status != "" && status != domain.ActionStatusNotStarted {
			return "Action " + strings.ReplaceAll(status, "_", " ") + ": " + str("title"), ""
		}
		return "New action: " + str("title") + " (owner " + str("owner_human_id") + ")", ""
	case domain.TypeOperationalRoleAssignment:
		who := str("human_id")
		if b, _ := v["deputy"].(bool); b {
			return "Make " + who + " deputy " + str("role_name"), ""
		}
		return "Make " + who + " " + str("role_name"), "The previous holder's history is kept."
	case domain.TypeCommunicationIssue:
		return "Issue: " + str("subject"), "The approved wording may then be sent on its channels. Exercise notifications are suppressed."
	case domain.TypeCommunicationCadence:
		return "Report cadence: " + str("name"), ""
	case domain.TypeObligationTrigger:
		return "Start obligation clock " + str("definition_id"), "The provisional clock becomes firm."
	case domain.TypeObligationChange:
		return "Change obligation clock " + str("clock_id") + " (" + str("kind") + ")", "The original deadline and rule are preserved."
	case domain.TypeObligationDefinition:
		return "Obligation: " + str("name"), "Becomes available to start clocks in every incident."
	case domain.TypeIncidentRelationship:
		if b, _ := v["active"].(bool); !b {
			return "Remove link between " + s.scopeID + " and " + str("related_incident_id"), "The link is withdrawn. History is kept."
		}
		return "Relate " + s.scopeID + " to " + str("related_incident_id"), "Each incident shows the link to people who can see both. Nobody gains access, and nothing is merged."
	case domain.TypeStandDown:
		return "Stand down the emergency response", "The emergency phase ends. The incident stays open for review."
	case domain.TypePIR:
		return "Confirm the Post-Incident Review", "Closure becomes possible once other prerequisites are met."
	case domain.TypeRetention:
		return "Retention: " + str("retention_class"), ""
	case domain.TypeClosure:
		return "Close the incident", "The incident becomes read-only. Privileged and shareable closure records are produced."
	case "security.incident_membership", "security.compartment_membership":
		verb := "Grant"
		if b, ok := v["active"].(bool); ok && !b {
			verb = "Revoke"
		}
		target := "incident access"
		if s.recordType == "security.compartment_membership" {
			target = str("compartment") + " read-in"
		}
		consequence := ""
		if exp := str("expires_at"); exp != "" && verb == "Grant" {
			consequence = "Access ends automatically at " + exp + "."
		}
		return verb + " " + target + " for " + str("human_id"), consequence
	}
	return label, ""
}

func firstString(v map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := v[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// payloadFields turns a proposal payload into labelled fields in stored order.
func payloadFields(payload []byte) ([]web.Field, map[string]any) {
	values := map[string]any{}
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()
	var fields []web.Field
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return nil, values
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			break
		}
		key, _ := keyTok.(string)
		var raw any
		if err := dec.Decode(&raw); err != nil {
			break
		}
		values[key] = raw
		if text := fieldText(raw); text != "" {
			fields = append(fields, web.Field{Name: fieldName(key), Value: text})
		}
	}
	return fields, values
}

func fieldName(key string) string {
	names := map[string]string{
		"owner_human_id": "Owner", "human_id": "Person", "role_name": "Role", "reason_evidence": "Reason and evidence",
		"assigned_to_human_id": "Assigned to", "source_ref": "Source", "due_at": "Due", "interval_minutes": "Every (minutes)",
		"first_due_at": "First due", "draft_sha256": "Approved wording (SHA-256)", "draft_record_id": "Draft",
		"definition_id": "Obligation", "trigger_at": "Trigger time", "source_record_id": "Source record",
		"new_deadline": "New deadline", "expires_at": "Expires", "effective_at": "Effective from",
	}
	if name, ok := names[key]; ok {
		return name
	}
	key = strings.ReplaceAll(key, "_", " ")
	if key == "" {
		return ""
	}
	return strings.ToUpper(key[:1]) + key[1:]
}

// machineWord matches stored enum values such as not_started.
var machineWord = regexp.MustCompile(`^[a-z]+(_[a-z]+)+$`)

func fieldText(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		if parsed, err := time.Parse(time.RFC3339Nano, t); err == nil {
			return parsed.UTC().Format(displayTime)
		}
		if machineWord.MatchString(t) {
			return strings.ReplaceAll(t, "_", " ")
		}
		return t
	case bool:
		if t {
			return "Yes"
		}
		return "No"
	case json.Number:
		return t.String()
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			if s := fieldText(item); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprint(t)
		}
		return string(b)
	}
}

// DecideProposal is the single browser decision path. The kind selects an
// existing application decision service; each one re-loads the stored
// proposal, binds the exact version and refuses maker-as-confirmer.
func (r *operatorRuntime) DecideProposal(ctx context.Context, token string, in web.DecisionInput) error {
	switch in.Kind {
	case "declaration", "incident":
		return r.Decide(ctx, token, in.ScopeID, in.RecordType, in.RecordID, in.Version, in.Outcome)
	case "severity":
		return r.DecideSeverity(ctx, token, in.ScopeID, in.RecordID, in.Version, in.Outcome)
	case "obligation":
		return r.DecideObligation(ctx, token, in.ScopeID, in.RecordType, in.RecordID, in.Version, in.Outcome)
	case "communication":
		return r.DecideCommunication(ctx, token, in.ScopeID, in.RecordType, in.RecordID, in.Version, in.Outcome)
	case "access":
		return r.DecideAccess(ctx, token, in.ScopeID, in.RecordType, in.RecordID, in.Version, in.Outcome)
	case "relationship":
		principal, ok, err := r.principal(ctx, token)
		if err != nil {
			return err
		}
		if !ok {
			return app.ErrAuthorisationDenied
		}
		_, err = r.relationships.Decide(ctx, principal, in.ScopeID, in.RecordID, in.Version, in.Outcome, r.kernel.Clock().Read().Now)
		return err
	case "organisation":
		if !domain.IsOrganisationType(in.RecordType) {
			return errors.New("unknown approval kind")
		}
		return r.DecideOrganisation(ctx, token, in.RecordID, in.Version, in.Outcome)
	case "recovery":
		principal, ok, err := r.principal(ctx, token)
		if err != nil {
			return err
		}
		if !ok {
			return app.ErrAuthorisationDenied
		}
		_, err = r.recovery.Decide(ctx, principal, in.ScopeID, in.RecordType, in.RecordID, in.Version, in.Outcome, time.Now().UTC())
		return err
	}
	return errors.New("unknown approval kind")
}

func (r *operatorRuntime) emergencyChangedAt(ctx context.Context) time.Time {
	at, err := r.organisation.EmergencyChangedAt(ctx)
	if err != nil {
		return time.Time{}
	}
	return at
}

func (r *operatorRuntime) ProposeRelationship(ctx context.Context, token, incidentID, relatedID, reason string, active bool) error {
	principal, picture, err := r.scope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	scope := domain.Scope{IncidentID: incidentID, Mode: picture.Incident.Mode, Compartment: "GENERAL"}
	_, err = r.relationships.Propose(ctx, principal, scope, newOperatorID("REL"), domain.IncidentRelationship{RelatedIncidentID: strings.TrimSpace(relatedID), Reason: strings.TrimSpace(reason), Active: active}, r.kernel.Clock().Read().Now)
	return err
}

// ActivateChecklist proposes one action per step of a confirmed template.
// Owners come from the step's named person, else the current holder of its
// default role on this incident, else the person activating it. Every action
// still needs a second person (T-207).
func (r *operatorRuntime) ActivateChecklist(ctx context.Context, token, incidentID, templateID string) error {
	principal, picture, err := r.scope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	templates, err := r.organisation.ChecklistTemplates(ctx, principal)
	if err != nil {
		return err
	}
	var chosen *domain.GovernedChecklistTemplate
	for i := range templates {
		if templates[i].TemplateID == templateID && templates[i].Active {
			chosen = &templates[i]
		}
	}
	if chosen == nil {
		return errors.New("choose a confirmed checklist template")
	}
	holders := map[string]string{}
	for _, role := range picture.OperationalRoles {
		if !role.Assignment.Deputy {
			holders[strings.ToLower(role.Assignment.RoleName)] = role.Assignment.HumanID
		}
	}
	owners := map[string]string{}
	for _, step := range chosen.Steps {
		switch {
		case step.DefaultOwnerHumanID != "":
		case holders[strings.ToLower(step.DefaultRole)] != "":
			owners[step.StepID] = holders[strings.ToLower(step.DefaultRole)]
		default:
			owners[step.StepID] = principal.HumanID
		}
	}
	scope := domain.Scope{IncidentID: incidentID, Mode: picture.Incident.Mode, Compartment: "GENERAL"}
	_, err = r.incident.ActivateChecklist(ctx, principal, scope, chosen.TemplateID, newOperatorID("CHK"), chosen.ChecklistTemplate, owners, r.kernel.Clock().Read().Now)
	return err
}
