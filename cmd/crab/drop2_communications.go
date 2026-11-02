package main

import (
	"context"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/data/postgres"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/presentation/web"
)

func communicationPendingItems(values []postgres.PendingCommunicationView) []web.PendingCommunicationItem {
	out := make([]web.PendingCommunicationItem, 0, len(values))
	for _, value := range values {
		out = append(out, web.PendingCommunicationItem{IncidentID: value.IncidentID, RecordID: value.RecordID, RecordType: value.RecordType, Compartment: value.Compartment, Version: value.Version, Maker: value.Maker, Mode: value.Mode, ProposedAt: value.ProposedAt.UTC().Format(displayTime)})
	}
	return out
}

func (r *operatorRuntime) Communications(ctx context.Context, token, incidentID, compartment string) (web.CommunicationsPage, error) {
	principal, scope, err := r.drop2IncidentScope(ctx, token, incidentID, compartment)
	if err != nil {
		return web.CommunicationsPage{}, err
	}
	store, err := postgres.NewCommunicationStoreFromKernel(r.kernel)
	if err != nil {
		return web.CommunicationsPage{}, err
	}
	drafts, err := store.Drafts(ctx, scope.IncidentID, scope.Compartment)
	if err != nil {
		return web.CommunicationsPage{}, err
	}
	issued, err := store.Issued(ctx, scope.IncidentID, scope.Compartment)
	if err != nil {
		return web.CommunicationsPage{}, err
	}
	cadences, err := store.Cadences(ctx, scope.IncidentID, scope.Compartment)
	if err != nil {
		return web.CommunicationsPage{}, err
	}
	enquiries, err := store.IncomingEnquiries(ctx, scope.IncidentID, scope.Compartment)
	if err != nil {
		return web.CommunicationsPage{}, err
	}
	pending, err := store.PendingForIncident(ctx, scope.IncidentID, scope.Compartment)
	if err != nil {
		return web.CommunicationsPage{}, err
	}
	page := web.CommunicationsPage{IncidentID: scope.IncidentID, Compartment: scope.Compartment, Drafts: drafts, Issued: issued, Cadences: cadences, Enquiries: enquiries, Pending: communicationPendingItems(pending), Deliveries: map[string]web.DeliveryView{}}
	for _, c := range issued {
		d, err := r.communication.Delivery(ctx, principal, c)
		if err != nil {
			return web.CommunicationsPage{}, err
		}
		page.Deliveries[c.RecordID] = deliveryView(d)
	}
	if r.noticeStore != nil {
		notices, err := r.noticeStore.Notices(ctx, scope.IncidentID, 50)
		if err != nil {
			return web.CommunicationsPage{}, err
		}
		for _, n := range app.NoticeLines(principal, notices) {
			page.Notices = append(page.Notices, web.NoticeRow{Kind: n.Kind, Recipient: n.Recipient, Channel: n.Channel, Status: n.Status, StatusText: n.StatusText, At: n.At.UTC().Format(displayTime)})
		}
	}
	return page, nil
}

func deliveryView(d app.DeliveryView) web.DeliveryView {
	attempts := func(in []app.DeliveryAttempt) []web.DeliveryAttemptView {
		out := make([]web.DeliveryAttemptView, 0, len(in))
		for _, a := range in {
			out = append(out, web.DeliveryAttemptView{Channel: a.Channel, Status: a.Status, StatusText: a.StatusText, Detail: a.Detail, At: a.At.UTC().Format(displayTime)})
		}
		return out
	}
	v := web.DeliveryView{Earlier: attempts(d.Earlier), Failed: d.Failed, CanResend: d.CanResend}
	for _, r := range d.Requests {
		v.Requests = append(v.Requests, web.DeliveryRequestView{By: r.By, At: r.At.UTC().Format(displayTime), Resend: r.Resend, Attempts: attempts(r.Attempts), Missing: r.Missing})
	}
	return v
}

func (r *operatorRuntime) ResendCommunication(ctx context.Context, token, incidentID, communicationID string) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	_, err = r.communication.ResendFailed(ctx, principal, incidentID, communicationID, r.kernel.Clock().Read().Now)
	return err
}

func (r *operatorRuntime) RecordCommunicationDraft(ctx context.Context, token, incidentID, compartment string, value domain.CommunicationDraft) error {
	principal, scope, err := r.drop2IncidentScope(ctx, token, incidentID, compartment)
	if err != nil {
		return err
	}
	_, err = r.communication.RecordDraft(ctx, principal, scope, newOperatorID("COMDRAFT"), value, r.kernel.Clock().Read().Now)
	return err
}

func (r *operatorRuntime) ProposeCommunicationIssue(ctx context.Context, token, incidentID, draftID string) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	_, err = r.communication.ProposeIssue(ctx, principal, incidentID, draftID, newOperatorID("COMISSUE"), r.kernel.Clock().Read().Now)
	return err
}

func (r *operatorRuntime) ProposeCommunicationCadence(ctx context.Context, token, incidentID, compartment string, value domain.CommunicationCadence) error {
	principal, scope, err := r.drop2IncidentScope(ctx, token, incidentID, compartment)
	if err != nil {
		return err
	}
	_, err = r.communication.ProposeCadence(ctx, principal, scope, newOperatorID("CADENCE"), value, r.kernel.Clock().Read().Now)
	return err
}

func (r *operatorRuntime) RecordIncomingEnquiry(ctx context.Context, token, incidentID, compartment string, value domain.IncomingEnquiry) error {
	principal, scope, err := r.drop2IncidentScope(ctx, token, incidentID, compartment)
	if err != nil {
		return err
	}
	_, err = r.communication.RecordIncomingEnquiry(ctx, principal, scope, newOperatorID("ENQ"), value, r.kernel.Clock().Read().Now)
	return err
}

func (r *operatorRuntime) GenerateSituationReport(ctx context.Context, token, incidentID, cadenceID, compartment string) error {
	principal, _, err := r.drop2IncidentScope(ctx, token, incidentID, compartment)
	if err != nil {
		return err
	}
	_, err = r.communication.GenerateDueSituationReportDraft(ctx, principal, incidentID, cadenceID, newOperatorID("SITREP"), r.kernel.Clock().Read().Now)
	return err
}

func (r *operatorRuntime) NotifyCommunication(ctx context.Context, token, incidentID, communicationID string, channels []string) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	_, err = r.communication.NotifyIssued(ctx, principal, incidentID, communicationID, channels, r.kernel.Clock().Read().Now)
	return err
}

func (r *operatorRuntime) DecideCommunication(ctx context.Context, token, incidentID, recordType, recordID, version, outcome string) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	_, err = r.communication.Decide(ctx, principal, incidentID, recordType, recordID, version, outcome, r.kernel.Clock().Read().Now)
	return err
}
