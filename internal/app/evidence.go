package app

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

type EvidenceBackend interface {
	PutEvidenceBlob(context.Context, domain.Scope, string, string, io.Reader) (domain.BlobReceipt, error)
	AppendEvidence(context.Context, ledger.Record) error
	Evidence(context.Context, string) (domain.EvidenceView, bool, error)
	EvidenceHistory(context.Context, string) ([]domain.EvidenceHandlingView, error)
	EvidenceRegister(context.Context, string, string) ([]domain.EvidenceView, error)
	VerifyEvidenceBlob(context.Context, string) (domain.BlobReceipt, error)
	CopyVerifiedEvidenceBlob(context.Context, string, io.Writer) (domain.BlobReceipt, error)
}

type EvidenceService struct{ Backend EvidenceBackend }

type EvidenceCapture struct {
	OriginalFilename  string
	DeclaredMediaType string
	Description       string
	Collector         string
	Source            string
	CollectedAt       *time.Time
	OccurredAt        *time.Time
	SourceLocation    string
	CurrentHandler    string
	RelatedRef        string
}

func (s EvidenceService) Capture(ctx context.Context, principal authz.Principal, scope domain.Scope, evidenceID string, input EvidenceCapture, src io.Reader, now time.Time) (ledger.Record, error) {
	if err := s.ready(); err != nil {
		return ledger.Record{}, err
	}
	if d := authz.Authorise(principal, authz.ActionRecord, authz.ObjectScope{IncidentID: scope.IncidentID, Compartment: scope.Compartment}); !d.Allowed {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	if err := domain.ValidateEvidenceCaptureMetadata(input.OriginalFilename, input.Collector, input.Source, input.CurrentHandler); err != nil {
		return ledger.Record{}, err
	}
	if src == nil {
		return ledger.Record{}, errors.New("evidence byte stream is required")
	}
	receipt, err := s.Backend.PutEvidenceBlob(ctx, scope, input.OriginalFilename, input.DeclaredMediaType, src)
	if err != nil {
		return ledger.Record{}, err
	}
	value := domain.Evidence{
		BlobReceipt:    receipt,
		Description:    input.Description,
		Collector:      input.Collector,
		Source:         input.Source,
		CollectedAt:    input.CollectedAt,
		OccurredAt:     input.OccurredAt,
		SourceLocation: input.SourceLocation,
		CurrentHandler: input.CurrentHandler,
		RelatedRef:     input.RelatedRef,
	}
	record, err := domain.RecordEvidence(scope, evidenceID, principal.HumanID, now, value)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendEvidence(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s EvidenceService) Detail(ctx context.Context, principal authz.Principal, evidenceID string) (domain.EvidenceView, error) {
	if err := s.ready(); err != nil {
		return domain.EvidenceView{}, err
	}
	view, found, err := s.Backend.Evidence(ctx, evidenceID)
	if err != nil {
		return domain.EvidenceView{}, err
	}
	if !found {
		return domain.EvidenceView{}, ErrAuthorisationDenied
	}
	if d := authz.Authorise(principal, authz.ActionRead, authz.ObjectScope{IncidentID: view.IncidentID, Compartment: view.Compartment}); !d.Allowed {
		return domain.EvidenceView{}, ErrAuthorisationDenied
	}
	return view, nil
}

func (s EvidenceService) Register(ctx context.Context, principal authz.Principal, incidentID, compartment string) ([]domain.EvidenceView, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if compartment == "" {
		compartment = "GENERAL"
	}
	if d := authz.Authorise(principal, authz.ActionRead, authz.ObjectScope{IncidentID: incidentID, Compartment: compartment}); !d.Allowed {
		return nil, ErrAuthorisationDenied
	}
	return s.Backend.EvidenceRegister(ctx, incidentID, compartment)
}

func (s EvidenceService) History(ctx context.Context, principal authz.Principal, evidenceID string) ([]domain.EvidenceHandlingView, error) {
	if _, err := s.Detail(ctx, principal, evidenceID); err != nil {
		return nil, err
	}
	return s.Backend.EvidenceHistory(ctx, evidenceID)
}

func (s EvidenceService) RecordHandling(ctx context.Context, principal authz.Principal, evidenceID, recordID string, handling domain.EvidenceHandling, now time.Time) (ledger.Record, error) {
	view, err := s.Detail(ctx, principal, evidenceID)
	if err != nil {
		return ledger.Record{}, err
	}
	if d := authz.Authorise(principal, authz.ActionRecord, authz.ObjectScope{IncidentID: view.IncidentID, Compartment: view.Compartment}); !d.Allowed {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	handling.EvidenceID = evidenceID
	scope := domain.Scope{IncidentID: view.IncidentID, Mode: view.Mode, Compartment: view.Compartment}
	record, err := domain.RecordEvidenceHandling(scope, recordID, principal.HumanID, now, handling)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendEvidence(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s EvidenceService) VerifyAttachment(ctx context.Context, principal authz.Principal, evidenceID string) (domain.BlobReceipt, error) {
	view, err := s.Detail(ctx, principal, evidenceID)
	if err != nil {
		return domain.BlobReceipt{}, err
	}
	return s.Backend.VerifyEvidenceBlob(ctx, view.Evidence.BlobID)
}

func (s EvidenceService) Attachment(ctx context.Context, principal authz.Principal, evidenceID string, dst io.Writer) (domain.EvidenceView, string, error) {
	if err := s.ready(); err != nil {
		return domain.EvidenceView{}, "", err
	}
	view, found, err := s.Backend.Evidence(ctx, evidenceID)
	if err != nil {
		return domain.EvidenceView{}, "", err
	}
	if !found {
		return domain.EvidenceView{}, "", ErrAuthorisationDenied
	}
	if d := authz.Authorise(principal, authz.ActionDownload, authz.ObjectScope{IncidentID: view.IncidentID, Compartment: view.Compartment}); !d.Allowed {
		return domain.EvidenceView{}, "", ErrAuthorisationDenied
	}
	if dst == nil {
		return domain.EvidenceView{}, "", errors.New("attachment writer is required")
	}
	mode := domain.EvidencePresentationMode(view.Evidence.DeclaredMediaType, view.Evidence.DetectedMediaType)
	if _, err := s.Backend.CopyVerifiedEvidenceBlob(ctx, view.Evidence.BlobID, dst); err != nil {
		return domain.EvidenceView{}, mode, err
	}
	return view, mode, nil
}

func (s EvidenceService) ready() error {
	if s.Backend == nil {
		return errors.New("evidence backend is required")
	}
	return nil
}
