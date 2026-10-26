package main

import (
	"context"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/data/postgres"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/presentation/web"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

func (r *operatorRuntime) obligationService() (app.ObligationService, *postgres.ObligationStore, error) {
	runtime, store, err := postgres.NewObligationRuntimeFromKernel(r.kernel, r.signer)
	if err != nil {
		return app.ObligationService{}, nil, err
	}
	return app.ObligationService{Backend: runtime, Window: r.organisation}, store, nil
}

func obligationPendingItems(values []postgres.PendingObligationView) []web.PendingObligationItem {
	out := make([]web.PendingObligationItem, 0, len(values))
	for _, value := range values {
		out = append(out, web.PendingObligationItem{ScopeID: value.ScopeID, RecordID: value.RecordID, RecordType: value.RecordType, Compartment: value.Compartment, Version: value.Version, Maker: value.Maker, Mode: value.Mode, ProposedAt: value.ProposedAt.UTC().Format(displayTime)})
	}
	return out
}

func (r *operatorRuntime) GlobalObligations(ctx context.Context, token string) (web.GlobalObligationsPage, error) {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return web.GlobalObligationsPage{}, err
	}
	if !ok || principal.Role != authz.RoleMajor {
		return web.GlobalObligationsPage{}, app.ErrAuthorisationDenied
	}
	_, store, err := r.obligationService()
	if err != nil {
		return web.GlobalObligationsPage{}, err
	}
	definitions, err := store.Definitions(ctx)
	if err != nil {
		return web.GlobalObligationsPage{}, err
	}
	pending, err := store.PendingForScope(ctx, "", "SYSTEM")
	if err != nil {
		return web.GlobalObligationsPage{}, err
	}
	out := web.GlobalObligationsPage{Pending: obligationPendingItems(pending)}
	for _, value := range definitions {
		out.Definitions = append(out.Definitions, web.ObligationDefinitionItem{DefinitionID: value.DefinitionID, Definition: value.Definition, Maker: value.Maker, Confirmer: value.Confirmer, ConfirmedAt: value.ConfirmedAt.UTC().Format(displayTime)})
	}
	return out, nil
}

func (r *operatorRuntime) IncidentObligations(ctx context.Context, token, incidentID, compartment string, now time.Time) (web.IncidentObligationsPage, error) {
	principal, scope, err := r.drop2IncidentScope(ctx, token, incidentID, compartment)
	if err != nil {
		return web.IncidentObligationsPage{}, err
	}
	service, store, err := r.obligationService()
	if err != nil {
		return web.IncidentObligationsPage{}, err
	}
	clocks, err := service.Clocks(ctx, principal, scope.IncidentID, scope.Compartment, now)
	if err != nil {
		return web.IncidentObligationsPage{}, err
	}
	pending, err := store.PendingForScope(ctx, scope.IncidentID, scope.Compartment)
	if err != nil {
		return web.IncidentObligationsPage{}, err
	}
	return web.IncidentObligationsPage{IncidentID: scope.IncidentID, Compartment: scope.Compartment, Clocks: clocks, Pending: obligationPendingItems(pending)}, nil
}

func (r *operatorRuntime) ProposeObligationDefinition(ctx context.Context, token string, def domain.ObligationDefinition) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	service, _, err := r.obligationService()
	if err != nil {
		return err
	}
	_, err = service.ProposeDefinition(ctx, principal, newOperatorID("OBDEF"), def, time.Now().UTC())
	return err
}

func (r *operatorRuntime) ProposeObligationTrigger(ctx context.Context, token, incidentID, compartment string, value domain.ObligationTrigger) error {
	principal, scope, err := r.drop2IncidentScope(ctx, token, incidentID, compartment)
	if err != nil {
		return err
	}
	service, _, err := r.obligationService()
	if err != nil {
		return err
	}
	_, err = service.ProposeTrigger(ctx, principal, scope, newOperatorID("CLOCK"), value, time.Now().UTC())
	return err
}

func (r *operatorRuntime) ProposeObligationChange(ctx context.Context, token, incidentID, compartment string, value domain.ObligationChange) error {
	principal, scope, err := r.drop2IncidentScope(ctx, token, incidentID, compartment)
	if err != nil {
		return err
	}
	service, _, err := r.obligationService()
	if err != nil {
		return err
	}
	_, err = service.ProposeChange(ctx, principal, scope, newOperatorID("OBCHG"), value, time.Now().UTC())
	return err
}

func (r *operatorRuntime) DecideObligation(ctx context.Context, token, scopeID, recordType, recordID, version, outcome string) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	service, _, err := r.obligationService()
	if err != nil {
		return err
	}
	_, err = service.Decide(ctx, principal, scopeID, recordType, recordID, version, outcome, time.Now().UTC())
	return err
}
