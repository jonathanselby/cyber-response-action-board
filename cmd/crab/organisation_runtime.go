package main

import (
	"context"
	"encoding/json"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/presentation/web"
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
	caps := app.CapabilitiesOf(principal)
	page := web.OrganisationPage{CanPropose: caps.CanProposeOrganisation, CanReadContacts: caps.CanReadContacts}
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
