package main

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/presentation/web"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

func confirmedBy(v domain.OrganisationView) *web.ConfirmedBy {
	return &web.ConfirmedBy{Maker: v.Maker, Confirmer: v.Confirmer, At: v.At.UTC().Format(displayTime), SelfApproval: v.SelfApprove, TimeUnverified: v.TimeStatus != ledger.TimeTrusted}
}

func (r *operatorRuntime) Organisation(ctx context.Context, token string) (web.OrganisationPage, error) {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return web.OrganisationPage{}, err
	}
	if !ok {
		return web.OrganisationPage{}, app.ErrAuthorisationDenied
	}
	page := web.OrganisationPage{CanPropose: principal.Role == authz.RoleMajor, CanReadContacts: principal.Role == authz.RoleMajor || principal.Custodian}
	if scales, err := r.organisation.Backend.CurrentOrganisation(ctx, domain.TypeSeverityScale); err == nil && len(scales) > 0 {
		var scale domain.SeverityScale
		if json.Unmarshal(scales[0].Payload, &scale) == nil {
			page.Severity = scale.Levels
			page.SeverityMeta = confirmedBy(scales[0])
		}
	} else if err != nil {
		return web.OrganisationPage{}, err
	}
	if page.CanReadContacts {
		config, view, err := r.organisation.EmergencyConfiguration(ctx, principal)
		if err != nil {
			return web.OrganisationPage{}, err
		}
		if view != nil {
			page.Emergency = &config
			page.EmergencyMeta = confirmedBy(*view)
		}
	}
	if templates, err := r.organisation.ChecklistTemplates(ctx, principal); err == nil {
		page.Templates = templates
	}
	page.Windows = r.windowViews(ctx)
	if majors, err := r.access.MajorIDs(ctx); err == nil {
		page.Majors = majors
	}
	if page.CanPropose {
		if pending, err := r.organisation.Pending(ctx, principal); err == nil {
			page.Pending = len(pending)
		}
	}
	return page, nil
}

func (r *operatorRuntime) ProposeSeverityScale(ctx context.Context, token string, labels []string) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	scale := domain.SeverityScale{}
	for i, label := range labels {
		level := domain.SeverityLevel{Label: label}
		if i == 0 {
			level.Description = "Least severe"
		} else if i == len(labels)-1 {
			level.Description = "Most severe"
		}
		scale.Levels = append(scale.Levels, level)
	}
	_, err = r.organisation.ProposeSeverityScale(ctx, principal, newOperatorID("SCALE"), scale, r.kernel.Clock().Read().Now)
	return err
}

func (r *operatorRuntime) ProposeEmergencyConfiguration(ctx context.Context, token string, value domain.EmergencyConfiguration) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	_, err = r.organisation.ProposeEmergencyConfiguration(ctx, principal, newOperatorID("EMERGENCY"), value, r.kernel.Clock().Read().Now)
	return err
}

func (r *operatorRuntime) DecideOrganisation(ctx context.Context, token, recordID, version, outcome string) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	_, err = r.organisation.Decide(ctx, principal, recordID, version, outcome, r.kernel.Clock().Read().Now)
	return err
}

// organisationSummary describes an organisation proposal in operator words.
func organisationSummary(recordType string, payload []byte) (string, string, []web.Field) {
	switch recordType {
	case domain.TypeSeverityScale:
		var v domain.SeverityScale
		_ = json.Unmarshal(payload, &v)
		return "New severity scale", "Responders choose only from these levels. Existing incidents keep their severity until changed.", []web.Field{{Name: "Levels, least severe first", Value: joinLabels(v.Labels())}}
	case domain.TypeEmergencyConfiguration:
		var v domain.EmergencyConfiguration
		_ = json.Unmarshal(payload, &v)
		fields := []web.Field{
			{Name: "Organisation", Value: v.Organisation},
			{Name: "Emergency contacts", Value: countOf(len(v.Contacts))},
			{Name: "Insurers, regulators and advisers", Value: countOf(len(v.Parties))},
			{Name: "Escalation steps", Value: countOf(len(v.Escalation))},
			{Name: "Role cards", Value: countOf(len(v.RoleCards))},
			{Name: "Procedures", Value: countOf(len(v.Procedures))},
		}
		for _, c := range v.Contacts {
			fields = append(fields, web.Field{Name: c.Role, Value: c.Name + " · " + c.Primary})
		}
		return "Emergency information for " + v.Organisation, "The next Emergency Response Pack and every Incident Brief print this information. The current pack becomes stale.", fields
	}
	if recordType == domain.TypeSinglePersonWindow {
		var v domain.SinglePersonWindow
		_ = json.Unmarshal(payload, &v)
		if !v.Active {
			return "Close single-person window " + v.WindowID, "Self-approval stops immediately.", []web.Field{{Name: "Reason", Value: v.Reason}}
		}
		kinds := make([]string, 0, len(v.Kinds))
		for _, k := range v.Kinds {
			kinds = append(kinds, domain.WindowCoverableKinds[k])
		}
		return "Open single-person window " + v.WindowID, "Between these times the named Majors may decide their own proposals of these kinds, with a reason. Each is flagged SELF-APPROVAL.", []web.Field{
			{Name: "From (UTC)", Value: v.StartsAt.UTC().Format(displayTime)},
			{Name: "To (UTC)", Value: v.EndsAt.UTC().Format(displayTime)},
			{Name: "Who", Value: joinWith(v.Humans, ", ")},
			{Name: "Covers", Value: joinWith(kinds, ", ")},
			{Name: "Reason", Value: v.Reason},
		}
	}
	if recordType == domain.TypeChecklistTemplate {
		var v domain.GovernedChecklistTemplate
		_ = json.Unmarshal(payload, &v)
		if !v.Active {
			return "Withdraw checklist: " + v.Name, "Responders can no longer activate it. Actions already created are unchanged.", nil
		}
		fields := []web.Field{{Name: "Source", Value: v.SourceRef}}
		for i, step := range v.Steps {
			detail := step.Priority
			if step.DefaultRole != "" {
				detail += " · " + step.DefaultRole
			}
			fields = append(fields, web.Field{Name: "Step " + strconv.Itoa(i+1), Value: step.Title + " (" + detail + ")"})
		}
		return "Checklist template: " + v.Name, "Responders can activate it on any incident; each step becomes an action that still needs a second person.", fields
	}
	return "Organisation change", "", nil
}

func joinLabels(labels []string) string {
	out := ""
	for i, l := range labels {
		if i > 0 {
			out += " → "
		}
		out += l
	}
	return out
}

func countOf(n int) string {
	if n == 0 {
		return "none"
	}
	return strconv.Itoa(n)
}

func (r *operatorRuntime) ProposeChecklistTemplate(ctx context.Context, token string, value domain.GovernedChecklistTemplate) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	_, err = r.organisation.ProposeChecklistTemplate(ctx, principal, newOperatorID("TEMPLATE"), value, r.kernel.Clock().Read().Now)
	return err
}

func (r *operatorRuntime) ProposeSinglePersonWindow(ctx context.Context, token string, value domain.SinglePersonWindow) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	_, err = r.organisation.ProposeSinglePersonWindow(ctx, principal, newOperatorID("WINDOW"), value, r.kernel.Clock().Read().Now)
	return err
}

// windowViews lists windows with their state now.
func (r *operatorRuntime) windowViews(ctx context.Context) []web.WindowView {
	windows, views, err := r.organisation.Windows(ctx)
	if err != nil {
		return nil
	}
	now := r.kernel.Clock().Read().Now
	out := make([]web.WindowView, 0, len(windows))
	for i, w := range windows {
		state := "ENDED"
		switch {
		case !w.Active:
			state = "CLOSED"
		case now.Before(w.StartsAt):
			state = "UPCOMING"
		case now.Before(w.EndsAt):
			state = "OPEN"
		}
		view := web.WindowView{SinglePersonWindow: w, State: state, Meta: confirmedBy(views[i])}
		labels := make([]string, 0, len(w.Kinds))
		for _, k := range w.Kinds {
			labels = append(labels, domain.WindowCoverableKinds[k])
		}
		view.Kinds = labels
		view.SinglePersonWindow.Kinds = labels
		out = append(out, view)
	}
	return out
}

func joinWith(values []string, sep string) string {
	out := ""
	for i, v := range values {
		if i > 0 {
			out += sep
		}
		out += v
	}
	return out
}
