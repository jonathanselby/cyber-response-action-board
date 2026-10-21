package main

import (
	"context"
	"io"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

func (r *operatorRuntime) EvidenceRegister(ctx context.Context, token, incidentID, compartment string) ([]domain.EvidenceView, error) {
	principal, scope, err := r.drop2IncidentScope(ctx, token, incidentID, compartment)
	if err != nil {
		return nil, err
	}
	return r.evidence.Register(ctx, principal, scope.IncidentID, scope.Compartment)
}

func (r *operatorRuntime) CaptureEvidence(ctx context.Context, token, incidentID, compartment, filename, mediaType, description, collector, source string, collectedAt, occurredAt *time.Time, sourceLocation, currentHandler, relatedRef string, src io.Reader) error {
	principal, scope, err := r.drop2IncidentScope(ctx, token, incidentID, compartment)
	if err != nil {
		return err
	}
	_, err = r.evidence.Capture(ctx, principal, scope, newOperatorID("EVID"), appEvidenceCapture(filename, mediaType, description, collector, source, collectedAt, occurredAt, sourceLocation, currentHandler, relatedRef), src, time.Now().UTC())
	return err
}

func appEvidenceCapture(filename, mediaType, description, collector, source string, collectedAt, occurredAt *time.Time, sourceLocation, currentHandler, relatedRef string) app.EvidenceCapture {
	return app.EvidenceCapture{OriginalFilename: filename, DeclaredMediaType: mediaType, Description: description, Collector: collector, Source: source, CollectedAt: collectedAt, OccurredAt: occurredAt, SourceLocation: sourceLocation, CurrentHandler: currentHandler, RelatedRef: relatedRef}
}

func (r *operatorRuntime) EvidenceDetail(ctx context.Context, token, evidenceID string) (domain.EvidenceView, error) {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return domain.EvidenceView{}, err
	}
	if !ok {
		return domain.EvidenceView{}, app.ErrAuthorisationDenied
	}
	return r.evidence.Detail(ctx, principal, evidenceID)
}

func (r *operatorRuntime) EvidenceHistory(ctx context.Context, token, evidenceID string) ([]domain.EvidenceHandlingView, error) {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, app.ErrAuthorisationDenied
	}
	return r.evidence.History(ctx, principal, evidenceID)
}

func (r *operatorRuntime) RecordEvidenceHandling(ctx context.Context, token, evidenceID, action, handler, location, note string, occurredAt *time.Time) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	_, err = r.evidence.RecordHandling(ctx, principal, evidenceID, newOperatorID("EH"), domain.EvidenceHandling{Action: action, Handler: handler, Location: location, Note: note, OccurredAt: occurredAt}, time.Now().UTC())
	return err
}

func (r *operatorRuntime) DownloadEvidence(ctx context.Context, token, evidenceID string, dst io.Writer) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	_, _, err = r.evidence.Attachment(ctx, principal, evidenceID, dst)
	return err
}
