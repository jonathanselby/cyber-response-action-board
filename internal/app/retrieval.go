package app

import (
	"context"
	"errors"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

type RetrievalBackend interface {
	PutCorpusBlob(context.Context, domain.CorpusScope, string, string, io.Reader) (domain.BlobReceipt, error)
	ValidateCorpusBlob(context.Context, string, string) error
	AppendCorpus(context.Context, ledger.Record) error
	PendingCorpusSource(context.Context, string) (ledger.Record, bool, error)
	CorpusCandidates(context.Context, string, []string) ([]domain.CorpusCandidate, error)
}

type RetrievalService struct{ Backend RetrievalBackend }

type CorpusSourceInput struct {
	Title             string
	Filename          string
	DeclaredMediaType string
}

func (s RetrievalService) ProposeSource(ctx context.Context, principal authz.Principal, scope domain.CorpusScope, sourceID string, input CorpusSourceInput, src io.Reader, now time.Time) (ledger.Record, error) {
	if err := s.ready(); err != nil {
		return ledger.Record{}, err
	}
	if err := s.authoriseGovernance(principal, scope, authz.ActionPropose); err != nil {
		return ledger.Record{}, err
	}
	if strings.TrimSpace(sourceID) == "" || strings.TrimSpace(input.Title) == "" {
		return ledger.Record{}, errors.New("corpus source id and title are required")
	}
	format, err := domain.CorpusFormatForFilename(input.Filename)
	if err != nil {
		return ledger.Record{}, err
	}
	if src == nil {
		return ledger.Record{}, errors.New("corpus source byte stream is required")
	}
	receipt, err := s.Backend.PutCorpusBlob(ctx, scope, input.Filename, input.DeclaredMediaType, src)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.ValidateCorpusBlob(ctx, receipt.BlobID, format); err != nil {
		return ledger.Record{}, err
	}
	value := domain.CorpusSource{BlobReceipt: receipt, Title: input.Title, Format: format}
	record, err := domain.ProposeCorpusSource(scope, sourceID, principal.HumanID, now, value)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendCorpus(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s RetrievalService) DecideSource(ctx context.Context, principal authz.Principal, sourceID, expectedVersion, outcome string, now time.Time) (ledger.Record, error) {
	if err := s.ready(); err != nil {
		return ledger.Record{}, err
	}
	proposal, found, err := s.Backend.PendingCorpusSource(ctx, sourceID)
	if err != nil {
		return ledger.Record{}, err
	}
	if !found {
		return ledger.Record{}, ErrProposalNotFound
	}
	scope := domain.CorpusScope{IncidentID: proposal.IncidentID, Mode: proposal.IncidentMode, Compartment: proposal.Compartment}
	if err := s.authoriseGovernance(principal, scope, authz.ActionConfirm); err != nil {
		return ledger.Record{}, err
	}
	decided, err := domain.Decide(proposal, expectedVersion, principal.HumanID, outcome, now)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendCorpus(ctx, decided); err != nil {
		return ledger.Record{}, err
	}
	return decided, nil
}

func (s RetrievalService) Retrieve(ctx context.Context, principal authz.Principal, incidentID, compartment, query string, limit int) (domain.RetrievalResult, error) {
	if err := s.ready(); err != nil {
		return domain.RetrievalResult{}, err
	}
	if compartment == "" {
		compartment = "GENERAL"
	}
	if d := authz.Authorise(principal, authz.ActionSearch, authz.ObjectScope{IncidentID: incidentID, Compartment: compartment}); !d.Allowed {
		return domain.RetrievalResult{}, ErrAuthorisationDenied
	}
	compartments := []string{"GENERAL"}
	if compartment != "GENERAL" {
		compartments = append(compartments, compartment)
	}
	sort.Strings(compartments)
	candidates, err := s.Backend.CorpusCandidates(ctx, incidentID, compartments)
	if err != nil {
		return domain.RetrievalResult{}, err
	}
	return domain.RankCorpus(query, candidates, limit)
}

func (s RetrievalService) authoriseGovernance(principal authz.Principal, scope domain.CorpusScope, action authz.Action) error {
	if err := domain.ValidateCorpusScope(scope); err != nil {
		return err
	}
	if scope.Mode == "system" {
		if !principal.Authenticated || principal.HumanID == "" || principal.Role != authz.RoleMajor {
			return ErrAuthorisationDenied
		}
		return nil
	}
	if d := authz.Authorise(principal, action, authz.ObjectScope{IncidentID: scope.IncidentID, Compartment: scope.Compartment}); !d.Allowed {
		return ErrAuthorisationDenied
	}
	return nil
}

func (s RetrievalService) ready() error {
	if s.Backend == nil {
		return errors.New("retrieval backend is required")
	}
	return nil
}
