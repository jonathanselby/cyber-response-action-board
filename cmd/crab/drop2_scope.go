package main

import (
	"context"
	"strings"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

func (r *operatorRuntime) drop2IncidentScope(ctx context.Context, token, incidentID, compartment string) (authz.Principal, domain.Scope, error) {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return authz.Principal{}, domain.Scope{}, err
	}
	if !ok {
		return authz.Principal{}, domain.Scope{}, app.ErrAuthorisationDenied
	}
	incidentID = strings.TrimSpace(incidentID)
	compartment = strings.TrimSpace(compartment)
	if compartment == "" {
		compartment = "GENERAL"
	}
	picture, found, err := r.incident.CommonOperatingPicture(ctx, principal, incidentID, compartment)
	if err != nil {
		return authz.Principal{}, domain.Scope{}, err
	}
	if !found {
		return authz.Principal{}, domain.Scope{}, app.ErrAuthorisationDenied
	}
	return principal, domain.Scope{IncidentID: incidentID, Mode: picture.Incident.Mode, Compartment: compartment}, nil
}
