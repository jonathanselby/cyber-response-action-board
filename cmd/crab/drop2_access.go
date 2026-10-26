package main

import (
	"context"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/config"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/presentation/web"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/access"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

func accessDesignationViews(values []access.DesignationView) []web.AccessDesignationView {
	out := make([]web.AccessDesignationView, 0, len(values))
	for _, value := range values {
		out = append(out, web.AccessDesignationView{HumanID: value.HumanID, IncidentID: value.IncidentID, Compartment: value.Compartment})
	}
	return out
}

func accessMembershipViews(values []access.MembershipView) []web.AccessMembershipView {
	out := make([]web.AccessMembershipView, 0, len(values))
	for _, value := range values {
		item := web.AccessMembershipView{HumanID: value.HumanID, IncidentID: value.IncidentID, Compartment: value.Compartment, Reason: value.Reason, Status: value.Status, Implicit: value.Implicit}
		if value.EffectiveAt != nil {
			item.EffectiveAt = value.EffectiveAt.UTC().Format(displayTime)
		}
		if value.ExpiresAt != nil {
			item.ExpiresAt = value.ExpiresAt.UTC().Format(displayTime)
		}
		out = append(out, item)
	}
	return out
}

func (r *operatorRuntime) AccessAdministration(ctx context.Context, token string, now time.Time) (web.AccessAdministrationPage, error) {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return web.AccessAdministrationPage{}, err
	}
	if !ok {
		return web.AccessAdministrationPage{}, app.ErrAuthorisationDenied
	}
	view, err := r.accessService.Administration(ctx, principal, now)
	if err != nil {
		return web.AccessAdministrationPage{}, err
	}
	pending, err := r.access.PendingAccessAdministration(ctx)
	if err != nil {
		return web.AccessAdministrationPage{}, err
	}
	out := web.AccessAdministrationPage{Sponsors: accessDesignationViews(view.Sponsors), Controllers: accessDesignationViews(view.Controllers), IncidentMemberships: accessMembershipViews(view.IncidentMemberships), CompartmentMemberships: accessMembershipViews(view.CompartmentMemberships)}
	for _, value := range pending {
		out.Pending = append(out.Pending, web.AccessPendingView{ScopeID: value.ScopeID, RecordID: value.RecordID, RecordType: value.RecordType, Compartment: value.Compartment, Version: value.Version, Maker: value.Maker, Mode: value.Mode, ProposedAt: value.ProposedAt.UTC().Format(displayTime)})
	}
	for _, value := range view.OutputPermissions {
		out.OutputPermissions = append(out.OutputPermissions, web.AccessOutputView{HumanID: value.HumanID, Action: value.Action})
	}
	return out, nil
}

func (r *operatorRuntime) ProposeOutputPermission(ctx context.Context, token, humanID, action string, active bool) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	_, err = r.accessService.ProposeOutputPermission(ctx, principal, newOperatorID("OUTPUT"), humanID, authz.Action(action), active, r.kernel.Clock().Read().Now)
	return err
}

func (r *operatorRuntime) IncidentAccess(ctx context.Context, token, incidentID string, now time.Time) (web.AccessAdministrationPage, error) {
	principal, _, err := r.scope(ctx, token, incidentID)
	if err != nil {
		return web.AccessAdministrationPage{}, err
	}
	view, err := r.access.AdministrationView(ctx, now)
	if err != nil {
		return web.AccessAdministrationPage{}, err
	}
	pending, err := r.access.PendingAccessForIncident(ctx, incidentID)
	if err != nil {
		return web.AccessAdministrationPage{}, err
	}
	out := web.AccessAdministrationPage{}
	for _, value := range view.Sponsors {
		if value.IncidentID == incidentID {
			out.Sponsors = append(out.Sponsors, web.AccessDesignationView{HumanID: value.HumanID, IncidentID: value.IncidentID, Compartment: value.Compartment})
		}
	}
	for _, value := range view.IncidentMemberships {
		if value.IncidentID == incidentID {
			out.IncidentMemberships = append(out.IncidentMemberships, accessMembershipViews([]access.MembershipView{value})[0])
		}
	}
	allowedCompartments := principal.Compartments[incidentID]
	for _, value := range view.Controllers {
		if value.IncidentID == incidentID && allowedCompartments[value.Compartment] {
			out.Controllers = append(out.Controllers, web.AccessDesignationView{HumanID: value.HumanID, IncidentID: value.IncidentID, Compartment: value.Compartment})
		}
	}
	for _, value := range view.CompartmentMemberships {
		if value.IncidentID == incidentID && allowedCompartments[value.Compartment] {
			out.CompartmentMemberships = append(out.CompartmentMemberships, accessMembershipViews([]access.MembershipView{value})[0])
		}
	}
	for _, value := range pending {
		if value.Compartment != "" && value.Compartment != "GENERAL" && !allowedCompartments[value.Compartment] {
			continue
		}
		out.Pending = append(out.Pending, web.AccessPendingView{ScopeID: value.ScopeID, RecordID: value.RecordID, RecordType: value.RecordType, Compartment: value.Compartment, Version: value.Version, Maker: value.Maker, Mode: value.Mode, ProposedAt: value.ProposedAt.UTC().Format(displayTime)})
	}
	return out, nil
}

func (r *operatorRuntime) ProposeSponsor(ctx context.Context, token, incidentID, humanID string, active bool) error {
	principal, picture, err := r.scope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	scope := domain.Scope{IncidentID: incidentID, Mode: picture.Incident.Mode, Compartment: "GENERAL"}
	_, err = r.accessService.ProposeSponsorDesignation(ctx, principal, scope, newOperatorID("SPONSOR"), access.IncidentSponsorDesignation{HumanID: humanID, IncidentID: incidentID, Active: active}, time.Now().UTC())
	return err
}

func (r *operatorRuntime) ProposeController(ctx context.Context, token, incidentID, humanID, compartment string, active bool) error {
	principal, picture, err := r.scope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	scope := domain.Scope{IncidentID: incidentID, Mode: picture.Incident.Mode, Compartment: compartment}
	_, err = r.accessService.ProposeControllerDesignation(ctx, principal, scope, newOperatorID("CTRL"), access.CompartmentControllerDesignation{HumanID: humanID, IncidentID: incidentID, Compartment: compartment, Active: active}, time.Now().UTC())
	return err
}

func (r *operatorRuntime) ProposeIncidentMembership(ctx context.Context, token, incidentID, humanID string, active bool, effectiveAt, expiresAt *time.Time, reason string) error {
	principal, picture, err := r.scope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	scope := domain.Scope{IncidentID: incidentID, Mode: picture.Incident.Mode, Compartment: "GENERAL"}
	_, err = r.accessService.ProposeIncidentMembership(ctx, principal, scope, newOperatorID("MEM"), access.IncidentMembershipChange{HumanID: humanID, IncidentID: incidentID, Active: active, EffectiveAt: effectiveAt, ExpiresAt: expiresAt, Reason: reason}, time.Now().UTC())
	return err
}

func (r *operatorRuntime) ProposeCompartmentMembership(ctx context.Context, token, incidentID, humanID, compartment string, active bool, effectiveAt, expiresAt *time.Time, reason string) error {
	principal, picture, err := r.scope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	scope := domain.Scope{IncidentID: incidentID, Mode: picture.Incident.Mode, Compartment: compartment}
	_, err = r.accessService.ProposeCompartmentMembership(ctx, principal, scope, newOperatorID("CMEM"), access.CompartmentMembershipChange{HumanID: humanID, IncidentID: incidentID, Compartment: compartment, Active: active, EffectiveAt: effectiveAt, ExpiresAt: expiresAt, Reason: reason}, time.Now().UTC())
	return err
}

func (r *operatorRuntime) DecideAccess(ctx context.Context, token, scopeID, recordType, recordID, version, outcome string) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	_, err = r.accessService.Decide(ctx, principal, scopeID, recordType, recordID, version, outcome, time.Now().UTC())
	return err
}

func (r *operatorRuntime) Readiness(ctx context.Context, token string) (web.ReadinessPage, error) {
	if _, ok, err := r.principal(ctx, token); err != nil {
		return web.ReadinessPage{}, err
	} else if !ok {
		return web.ReadinessPage{}, app.ErrAuthorisationDenied
	}
	cfg, err := config.Load()
	if err != nil {
		return web.ReadinessPage{}, err
	}
	value := app.CurrentReadiness(cfg)
	check := app.Check{Name: "postgresql", Status: "pass", Message: "schema, runtime role and full ledger verification passed at operator-runtime startup"}
	if r.kernel.IntegrityFailed() {
		check.Status = "fail"
		check.Message = "ledger verification failed; read-only integrity mode"
	}
	setReadinessCheck(&value, check)
	enrichReadiness(cfg, &value, r.kernel.Clock(), r.emergencyChangedAt(ctx))
	value = app.FinaliseReadiness(value)
	out := web.ReadinessPage{Ready: value.Ready}
	for _, item := range value.Checks {
		out.Checks = append(out.Checks, web.ReadinessCheckView{Name: humanCheckName(item.Name), Status: item.Status, Message: item.Message})
	}
	return out, nil
}

var _ = authz.RoleMajor
