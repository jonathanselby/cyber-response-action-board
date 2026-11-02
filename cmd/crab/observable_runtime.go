package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/presentation/web"
)

// Observable and management-summary adapters (addenda A2, A3). They resolve
// the session and translate types; every rule is in app.ObservableService and
// app.ManagementService.

func (r *operatorRuntime) Observables(ctx context.Context, token, incidentID, compartment string) ([]web.ObservableRow, error) {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, app.ErrAuthorisationDenied
	}
	items, err := r.observables.List(ctx, principal, incidentID, compartment)
	if err != nil {
		return nil, err
	}
	rows := make([]web.ObservableRow, 0, len(items))
	for _, item := range items {
		v := item.View
		rows = append(rows, web.ObservableRow{ObservableID: v.Observable.ObservableID, Type: v.Observable.Type, Value: v.Observable.Value, Status: v.Observable.Status,
			Source: v.Observable.Source, Note: v.Observable.Note, FirstSeen: v.Observable.FirstSeen.UTC().Format(displayTime), TimeConfidence: v.Observable.TimeConfidence,
			Maker: v.Maker, RecordedAt: v.RecordedAt.UTC().Format(displayTime), Compartment: v.Compartment, Version: v.Version, Related: item.Related})
	}
	return rows, nil
}

func (r *operatorRuntime) RecordObservable(ctx context.Context, token, incidentID string, in web.ObservableInput) error {
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
	compartment := in.Compartment
	if compartment == "" {
		compartment = "GENERAL"
	}
	scope := domain.Scope{IncidentID: incidentID, Mode: picture.Incident.Mode, Compartment: compartment}
	_, err = r.observables.Record(ctx, principal, scope, newOperatorID("OBS"), newOperatorID("OBSREC"), app.ObservableInput{Type: in.Type, Value: in.Value, Status: in.Status, Source: in.Source, TimeConfidence: in.TimeConfidence, Note: in.Note, FirstSeen: in.FirstSeen}, r.kernel.Clock().Read().Now)
	return err
}

func (r *operatorRuntime) ChangeObservableStatus(ctx context.Context, token, observableID, status, source, note string) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	_, err = r.observables.ChangeStatus(ctx, principal, observableID, newOperatorID("OBSREC"), status, source, note, r.kernel.Clock().Read().Now)
	return err
}

func managementView(s domain.ManagementSummary) *web.ManagementView {
	v := &web.ManagementView{ActiveIncidents: s.ActiveIncidents, OverdueCriticalActions: s.OverdueCriticalAction, OverdueObligations: s.OverdueObligations, Decisions: s.DecisionsInWindow}
	for _, c := range s.BySeverity {
		v.BySeverity = append(v.BySeverity, web.SeverityCountView{Severity: c.Severity, Count: c.Count})
	}
	if s.MedianDecisionLoop != nil {
		v.MedianDecisionLoop = humanDuration(*s.MedianDecisionLoop)
	}
	return v
}

// humanDuration formats a duration as hours and minutes, or seconds under a
// minute.
func humanDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	h, m := int(d.Hours()), int(d.Minutes())%60
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dh %02dm", h, m)
}
