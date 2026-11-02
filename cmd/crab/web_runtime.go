package main

// Slice 15 browser runtime: composition of existing application services for
// the go-app-shell presentation tier. Every method authorises through the
// application tier; nothing here introduces a new mechanism or state store.

import (
	"context"
	"errors"
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
	principal, session, ok, err := r.principals.ForToken(ctx, token)
	if err != nil || !ok {
		return web.FrameView{}, false, err
	}
	caps := app.CapabilitiesOf(principal)
	frame := web.FrameView{HumanID: principal.HumanID, Role: caps.Role, HoldsCustodian: caps.IsCustodian}
	frame.Status.BreakGlass = session.BreakGlass
	frame.Status.TimeStatus = r.kernel.Clock().Read().Status
	if open, err := r.organisation.OpenWindow(ctx, r.kernel.Clock().Read().Now); err == nil {
		frame.Status.WindowOpen = open
	}
	frame.Status.IntegrityFailed = r.kernel.IntegrityFailed()
	if isolate, err := r.access.IsolateMode(ctx); err == nil {
		frame.Status.Isolate = isolate
	}
	if cfg, err := config.Load(); err == nil {
		frame.Status.PackStale = packFreshnessCheck(cfg, r.emergencyChangedAt(ctx)).Status != "pass"
	}
	// Only the count is needed here, on every page (C11: building the whole
	// view, self-approvals included, was the largest cost per request).
	if queue, err := (app.ApprovalsService{Backend: approvalsBackend{r}}).Queue(ctx, principal, "", r.kernel.Clock().Read().Now); err == nil {
		frame.Status.PendingApprovals = len(queue.Waiting)
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
	failedNotifications := 0
	now := r.kernel.Clock().Read().Now
	ordered, clocks := app.OrderBoard(picture, picture.Obligations, now)
	out := web.BoardView{Picture: ordered, Compartment: compartment, Clocks: clocks, Phase: humanPhase(domain.PhaseActiveResponse), Limit: app.BoardSectionLimit}
	sort.SliceStable(out.Picture.Timeline, func(i, j int) bool {
		return out.Picture.Timeline[i].RecordedAt.After(out.Picture.Timeline[j].RecordedAt)
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
			cutoff := r.kernel.Clock().Read().Now.Add(-24 * time.Hour)
			for _, v := range issued {
				if v.IssuedAt.After(cutoff) {
					out.IssuedRecently = append(out.IssuedRecently, v)
				}
				// A failure stays on the Board until it is resent (T-233).
				if outcomes, err := store.NotificationOutcomes(ctx, incidentID, v.RecordID); err == nil {
					failed, _ := app.FailedChannels(outcomes)
					failedNotifications += len(failed)
				}
			}
		}
	}

	if view, err := r.access.AdministrationView(ctx, r.kernel.Clock().Read().Now); err == nil {
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
	focus := app.BoardFocusInput{Viewer: principal.HumanID, TimeStatus: r.kernel.Clock().Read().Status, IntegrityFailed: r.kernel.IntegrityFailed(), Picture: out.Picture, Clocks: out.Clocks, FailedNotifications: failedNotifications, Now: now}
	if out.LatestHandover != nil {
		at := out.LatestHandover.RecordedAt
		focus.LastHandoverAt = &at
	}
	for _, item := range app.BoardNow(focus) {
		out.Now = append(out.Now, web.NowItem{Text: item.Text, Link: item.Link, Tone: item.Tone})
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
	_, err = r.incident.RecordUncertainty(ctx, principal, scope, newOperatorID("UNC"), domain.Uncertainty{Kind: kind, Statement: statement}, r.kernel.Clock().Read().Now)
	return err
}

func (r *operatorRuntime) ProposeDecision(ctx context.Context, token, incidentID, decision, rationale string) error {
	principal, scope, err := r.generalScope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	_, err = r.incident.ProposeDecision(ctx, principal, scope, newOperatorID("DEC"), domain.Decision{Decision: decision, Rationale: rationale}, r.kernel.Clock().Read().Now)
	return err
}

func (r *operatorRuntime) ProposeRoleAssignment(ctx context.Context, token, incidentID, roleName, humanID string, deputy bool) error {
	principal, scope, err := r.generalScope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	_, err = r.incident.ProposeOperationalRoleAssignment(ctx, principal, scope, newOperatorID("ROLE"), domain.OperationalRoleAssignment{RoleName: roleName, HumanID: humanID, Deputy: deputy}, r.kernel.Clock().Read().Now)
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
		_, err = r.incident.ProposeAction(ctx, principal, scope, actionID, next, r.kernel.Clock().Read().Now)
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
	blockers, err := r.recovery.ClosureBlockers(ctx, principal, incidentID, r.kernel.Clock().Read().Now)
	if err != nil {
		return web.CloseOutView{}, err
	}
	out.Blockers = blockers
	out.CanReopen = r.recovery.CanReopen(ctx, principal, incidentID)
	return out, nil
}

func (r *operatorRuntime) ProposeReopening(ctx context.Context, token, incidentID, reason string) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	picture, found, err := r.incident.CommonOperatingPicture(ctx, principal, incidentID, "GENERAL")
	if err != nil {
		return err
	}
	if !found {
		return app.ErrAuthorisationDenied
	}
	scope := domain.Scope{IncidentID: incidentID, Mode: picture.Incident.Mode, Compartment: "GENERAL"}
	_, err = r.recovery.ProposeReopening(ctx, principal, scope, newOperatorID("REOPEN"), domain.Reopening{Reason: strings.TrimSpace(reason)}, r.kernel.Clock().Read().Now)
	return err
}

func (r *operatorRuntime) RecordRecoveryUpdate(ctx context.Context, token, incidentID string, input web.RecoveryUpdateInput) error {
	principal, scope, err := r.generalScope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	value := domain.RecoveryUpdate{ItemID: newOperatorID("ITEM"), Summary: input.Summary, OwnerHumanID: input.OwnerHumanID, RestorationPriority: input.RestorationPriority, Verified: input.Verified, ResidualRisk: input.ResidualRisk, HandoffRef: input.HandoffRef}
	_, err = r.recovery.RecordRecoveryUpdate(ctx, principal, scope, newOperatorID("RECOV"), value, r.kernel.Clock().Read().Now)
	return err
}

func (r *operatorRuntime) ProposeStandDown(ctx context.Context, token, incidentID, reason string) error {
	principal, scope, err := r.generalScope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	_, err = r.recovery.ProposeStandDown(ctx, principal, scope, newOperatorID("STANDDOWN"), domain.StandDown{Reason: reason}, r.kernel.Clock().Read().Now)
	return err
}

func (r *operatorRuntime) ProposePIR(ctx context.Context, token, incidentID, summary string, lessons, longTermWork []string) error {
	principal, scope, err := r.generalScope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	_, err = r.recovery.ProposePIR(ctx, principal, scope, newOperatorID("PIR"), domain.PIR{ExecutiveSummary: summary, HumanLessons: lessons, LongTermWork: longTermWork}, r.kernel.Clock().Read().Now)
	return err
}

func (r *operatorRuntime) ProposeRetention(ctx context.Context, token, incidentID string, input web.RetentionInput) error {
	principal, scope, err := r.generalScope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	value := domain.RetentionDisposition{RetentionClass: input.RetentionClass, Hold: input.Hold, HoldReason: input.HoldReason, DispositionEligibleAfter: input.DispositionEligibleAfter, HandoffRef: input.HandoffRef}
	_, err = r.recovery.ProposeRetention(ctx, principal, scope, newOperatorID("RETAIN"), value, r.kernel.Clock().Read().Now)
	return err
}

func (r *operatorRuntime) ProposeClosure(ctx context.Context, token, incidentID string, input web.ClosureInput) error {
	principal, scope, err := r.generalScope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	value := domain.Closure{Reason: input.Reason, PrivilegedCompartment: input.PrivilegedCompartment, PrivilegedSummary: input.PrivilegedSummary, ShareableSummary: input.ShareableSummary}
	_, err = r.recovery.ProposeClosure(ctx, principal, scope, newOperatorID("CLOSE"), value, r.kernel.Clock().Read().Now)
	return err
}

func (r *operatorRuntime) TranscribePaper(ctx context.Context, token, incidentID string, input web.PaperInput) error {
	principal, scope, err := r.generalScope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	value := domain.PaperTranscription{Statement: input.Statement, PaperOccurredAt: input.PaperOccurredAt, SourceReference: input.SourceReference, TimeConfidence: input.TimeConfidence}
	_, err = r.incident.RecordPaperTranscription(ctx, principal, scope, newOperatorID("PAPER"), value, r.kernel.Clock().Read().Now)
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
	if _, err := service.GenerateIncidentBrief(ctx, principal, incidentID, "GENERAL", nil, pageSize, printMode, r.kernel.Clock().Read().Now); err != nil {
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

// approvals translates the application approvals queue for the browser.
func (r *operatorRuntime) approvals(ctx context.Context, principal authz.Principal, filter string) (web.ApprovalsView, error) {
	queue, err := app.ApprovalsService{Backend: approvalsBackend{r}}.Queue(ctx, principal, filter, r.kernel.Clock().Read().Now)
	if err != nil {
		return web.ApprovalsView{}, err
	}
	out := web.ApprovalsView{SelfApprovals: r.recentSelfApprovals(ctx, principal)}
	for _, i := range queue.Incidents {
		out.Incidents = append(out.Incidents, web.IncidentLink{IncidentID: i.IncidentID, Title: i.Title, Mode: i.Mode, Severity: i.Severity})
	}
	for _, i := range queue.Waiting {
		out.Waiting = append(out.Waiting, approvalView(i))
	}
	for _, i := range queue.AwaitingOther {
		out.AwaitingOther = append(out.AwaitingOther, approvalView(i))
	}
	return out, nil
}

func approvalView(i app.ApprovalItem) web.ApprovalView {
	v := web.ApprovalView{Group: i.Group, Kind: i.Kind, Label: i.Label, IncidentID: i.IncidentID, IncidentTitle: i.IncidentTitle, Compartment: i.Compartment,
		Summary: i.Summary, Effective: webFields(i.Effective), Proposed: webFields(i.Proposed), Maker: i.Maker, MakerName: i.MakerName, MakerRole: i.MakerRole,
		MakerExternal: i.MakerExternal, Reason: i.Reason, Consequence: i.Consequence, RejectConsequence: i.RejectConsequence,
		ScopeID: i.ScopeID, RecordType: i.RecordType, RecordID: i.RecordID, Version: i.Version, CanDecide: i.CanDecide, CanSelfApprove: i.CanSelfApprove}
	if !i.ProposedAt.IsZero() {
		v.ProposedAt = i.ProposedAt.UTC().Format(displayTime)
	}
	return v
}

func webFields(in []app.Field) []web.Field {
	out := make([]web.Field, 0, len(in))
	for _, f := range in {
		out = append(out, web.Field{Name: f.Name, Value: f.Value})
	}
	return out
}

// DecideProposal is the single browser decision path. The kind selects an
// existing application decision service; each one re-loads the stored
// proposal, binds the exact version and refuses maker-as-confirmer.
func (r *operatorRuntime) DecideProposal(ctx context.Context, token string, in web.DecisionInput) error {
	if in.SelfReason != "" {
		// Only kinds a single-person window can cover carry the self-approval
		// request; access, organisation, relationship and declaration never do.
		switch in.Kind {
		case "incident", "severity", "communication", "obligation", "recovery":
			ctx = app.WithSelfApproval(ctx, in.SelfReason)
		default:
			return app.ErrNoWindow
		}
	}
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
		_, err = r.recovery.Decide(ctx, principal, in.ScopeID, in.RecordType, in.RecordID, in.Version, in.Outcome, r.kernel.Clock().Read().Now)
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

// recentSelfApprovals lists self-approved decisions in the last 24 hours in
// incidents the viewer can see (AC-07 single-approval dashboard).
func (r *operatorRuntime) recentSelfApprovals(ctx context.Context, principal authz.Principal) []web.SelfApprovalView {
	var out []web.SelfApprovalView
	for _, a := range (app.ApprovalsService{Backend: approvalsBackend{r}}).RecentSelfApprovals(ctx, principal, r.kernel.Clock().Read().Now) {
		out = append(out, web.SelfApprovalView{IncidentID: a.IncidentID, Summary: a.Summary, At: a.At.UTC().Format(displayTime), TimeUnverified: a.TimeUnverified})
	}
	return out
}

func humanRecordLabel(recordType string) string { return app.ApprovalLabel(recordType) }
