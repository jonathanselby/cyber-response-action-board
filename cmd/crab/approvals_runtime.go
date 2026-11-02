package main

import (
	"context"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/data/postgres"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

// approvalsBackend lists pending proposals from each store for
// app.ApprovalsService. It applies no policy; errors from a store leave that
// store's list empty, as before, so one unavailable area cannot hide the rest.
type approvalsBackend struct{ r *operatorRuntime }

func (b approvalsBackend) Incident(ctx context.Context, _ authz.Principal, incidentID string) (app.IncidentRef, bool) {
	title, mode, severity, found, err := b.r.incidents.IncidentHeader(ctx, incidentID)
	if err != nil || !found {
		return app.IncidentRef{}, false
	}
	return app.IncidentRef{IncidentID: incidentID, Title: title, Mode: mode, Severity: severity}, true
}

func (b approvalsBackend) PendingInIncident(ctx context.Context, principal authz.Principal, id, compartment string) []app.PendingProposal {
	r := b.r
	var out []app.PendingProposal
	if pending, err := r.incidents.PendingProposals(ctx, id); err == nil {
		for _, p := range pending {
			if p.Compartment != "GENERAL" && p.Compartment != compartment {
				continue
			}
			kind := "incident"
			if p.RecordType == domain.TypeIncidentDeclaration {
				kind = "declaration"
			}
			out = append(out, app.PendingProposal{Kind: kind, ScopeID: id, RecordType: p.RecordType, RecordID: p.RecordID, Compartment: p.Compartment, Version: p.Version, Maker: p.Maker, ProposedAt: p.ProposedAt})
		}
	}
	if continuity, err := r.continuityService(); err == nil {
		if pending, err := continuity.PendingSeverityChanges(ctx, principal, id, compartment); err == nil {
			for _, p := range pending {
				version, _ := domain.ProposalVersion(p)
				out = append(out, app.PendingProposal{Kind: "severity", ScopeID: id, RecordType: p.RecordType, RecordID: p.RecordID, Compartment: p.Compartment, Version: version, Maker: p.Maker, ProposedAt: p.Timestamp})
			}
		}
	}
	if _, store, err := r.obligationService(); err == nil {
		if pending, err := store.PendingForScope(ctx, id, compartment); err == nil {
			for _, p := range pending {
				out = append(out, app.PendingProposal{Kind: "obligation", ScopeID: p.ScopeID, RecordType: p.RecordType, RecordID: p.RecordID, Compartment: p.Compartment, Version: p.Version, Maker: p.Maker, ProposedAt: p.ProposedAt})
			}
		}
	}
	if store, err := postgres.NewCommunicationStoreFromKernel(r.kernel); err == nil {
		if pending, err := store.PendingForIncident(ctx, id, compartment); err == nil {
			for _, p := range pending {
				out = append(out, app.PendingProposal{Kind: "communication", ScopeID: p.IncidentID, RecordType: p.RecordType, RecordID: p.RecordID, Compartment: p.Compartment, Version: p.Version, Maker: p.Maker, ProposedAt: p.ProposedAt})
			}
		}
	}
	if pending, err := r.kernel.PendingRecoveryProposals(ctx, id, compartment); err == nil {
		for _, p := range pending {
			out = append(out, app.PendingProposal{Kind: "recovery", ScopeID: p.IncidentID, RecordType: p.RecordType, RecordID: p.RecordID, Compartment: p.Compartment, Version: p.Version, Maker: p.Maker, ProposedAt: p.ProposedAt})
		}
	}
	return out
}

func (b approvalsBackend) PendingRelationships(ctx context.Context, id string) []app.PendingProposal {
	var out []app.PendingProposal
	if pending, err := b.r.relationStore.PendingRelationships(ctx, id); err == nil {
		for _, p := range pending {
			out = append(out, app.PendingProposal{Kind: "relationship", ScopeID: id, RecordType: domain.TypeIncidentRelationship, RecordID: p.RecordID, Compartment: "GENERAL", Version: p.Version, Maker: p.Maker, ProposedAt: p.ProposedAt, RelatedIncidentID: p.RelatedIncidentID})
		}
	}
	return out
}

func (b approvalsBackend) PendingAccessForIncident(ctx context.Context, id string) []app.PendingProposal {
	var out []app.PendingProposal
	if pending, err := b.r.access.PendingAccessForIncident(ctx, id); err == nil {
		for _, p := range pending {
			out = append(out, app.PendingProposal{Kind: "access", ScopeID: p.ScopeID, RecordType: p.RecordType, RecordID: p.RecordID, Compartment: p.Compartment, Version: p.Version, Maker: p.Maker, ProposedAt: p.ProposedAt})
		}
	}
	return out
}

func (b approvalsBackend) PendingDeclarations(ctx context.Context) ([]app.PendingProposal, map[string]string) {
	var out []app.PendingProposal
	titles := map[string]string{}
	if declarations, err := b.r.incidents.PendingDeclarations(ctx); err == nil {
		for _, d := range declarations {
			titles[d.IncidentID] = d.Declaration.Title
			out = append(out, app.PendingProposal{Kind: "declaration", ScopeID: d.IncidentID, RecordType: domain.TypeIncidentDeclaration, RecordID: d.RecordID, Compartment: "GENERAL", Version: d.Version, Maker: d.Maker})
		}
	}
	return out, titles
}

func (b approvalsBackend) PendingSystemObligations(ctx context.Context) []app.PendingProposal {
	var out []app.PendingProposal
	if _, store, err := b.r.obligationService(); err == nil {
		if pending, err := store.PendingForScope(ctx, "", "SYSTEM"); err == nil {
			for _, p := range pending {
				out = append(out, app.PendingProposal{Kind: "obligation", ScopeID: p.ScopeID, RecordType: p.RecordType, RecordID: p.RecordID, Compartment: p.Compartment, Version: p.Version, Maker: p.Maker, ProposedAt: p.ProposedAt})
			}
		}
	}
	return out
}

func (b approvalsBackend) PendingOrganisation(ctx context.Context, principal authz.Principal) []app.PendingProposal {
	var out []app.PendingProposal
	if pending, err := b.r.organisation.Pending(ctx, principal); err == nil {
		for _, p := range pending {
			out = append(out, app.PendingProposal{Kind: "organisation", RecordType: p.RecordType, RecordID: p.RecordID, Compartment: "SYSTEM", Version: p.Version, Maker: p.Maker, ProposedAt: p.At})
		}
	}
	return out
}

func (b approvalsBackend) PendingAccessAdministration(ctx context.Context) []app.PendingProposal {
	var out []app.PendingProposal
	if pending, err := b.r.access.PendingAccessAdministration(ctx); err == nil {
		for _, p := range pending {
			out = append(out, app.PendingProposal{Kind: "access", ScopeID: p.ScopeID, RecordType: p.RecordType, RecordID: p.RecordID, Compartment: p.Compartment, Version: p.Version, Maker: p.Maker, ProposedAt: p.ProposedAt})
		}
	}
	return out
}

func (b approvalsBackend) Payload(ctx context.Context, kind, scopeID, recordType, recordID string) ([]byte, bool, error) {
	return b.r.kernel.ProposalPayload(ctx, kind, scopeID, recordType, recordID)
}

func (b approvalsBackend) People(ctx context.Context) (map[string]app.PersonLabel, error) {
	records, err := b.r.access.People(ctx, b.r.kernel.Clock().Read().Now)
	if err != nil {
		return nil, err
	}
	out := make(map[string]app.PersonLabel, len(records))
	for _, p := range records {
		label := app.PersonLabel{DisplayName: p.Identity.DisplayName, Organisation: p.Identity.Organisation, External: p.Identity.External, Role: p.Role}
		if label.DisplayName == "" {
			label.DisplayName = p.HumanID
		}
		out[p.HumanID] = label
	}
	return out, nil
}

func (b approvalsBackend) SelfApprovalPermitted(ctx context.Context, humanID, recordType string, now time.Time) bool {
	ok, err := b.r.organisation.Permits(ctx, humanID, recordType, now)
	return err == nil && ok
}

func (b approvalsBackend) CurrentSeverity(ctx context.Context, incidentID string) string {
	if b.r.incidents == nil {
		return ""
	}
	picture, found, err := b.r.incidents.CommonOperatingPicture(ctx, incidentID)
	if err != nil || !found {
		return ""
	}
	return picture.Incident.Severity
}

func (b approvalsBackend) SelfApprovedSince(ctx context.Context, incidentID string, since time.Time) []domain.TimelineEntry {
	entries, _ := b.r.incidents.SelfApprovedSince(ctx, incidentID, "GENERAL", since)
	return entries
}
