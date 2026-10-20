package app

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

var (
	ErrManagedModelSensitivity = errors.New("managed model lane is not approved for this input")
	ErrModelQueueFull          = errors.New("model capacity is full; generation shed")
	ErrModelSourceUnsupported  = errors.New("authorised sources do not support model drafting request")
	ErrModelOutputInvalid      = errors.New("model output failed deterministic validation")
)

var modelRequestCounter uint64

type ModelBackend interface {
	Lane(domain.ModelLane) (domain.ModelLanePolicy, error)
	Generate(context.Context, domain.ModelLane, string) (domain.ModelProviderResult, error)
}

type ManagedModelPolicy interface {
	AllowsManagedModel(context.Context, authz.Principal, domain.Scope, []domain.RetrievalCitation) (bool, error)
}

type ModelDraftRequest struct {
	SpecID      string
	Lane        domain.ModelLane
	IncidentID  string
	Mode        string
	Compartment string
	Query       string
	Instruction string
	Limit       int
}

type ModelService struct {
	Retrieval     *RetrievalService
	Backend       ModelBackend
	ManagedPolicy ManagedModelPolicy
	slots         chan struct{}
	now           func() time.Time
}

func NewModelService(retrieval *RetrievalService, backend ModelBackend, managedPolicy ManagedModelPolicy, maxConcurrent int) (*ModelService, error) {
	if retrieval == nil || backend == nil {
		return nil, errors.New("model service requires retrieval and model backend")
	}
	if maxConcurrent <= 0 {
		return nil, errors.New("model concurrency must be positive")
	}
	return &ModelService{Retrieval: retrieval, Backend: backend, ManagedPolicy: managedPolicy, slots: make(chan struct{}, maxConcurrent), now: time.Now}, nil
}

func (s *ModelService) Draft(ctx context.Context, principal authz.Principal, req ModelDraftRequest) (domain.ModelDraft, error) {
	if s == nil || s.Retrieval == nil || s.Backend == nil || s.slots == nil {
		return domain.ModelDraft{}, errors.New("model service is not configured")
	}
	if req.Mode != "live" && req.Mode != "exercise" {
		return domain.ModelDraft{}, errors.New("model draft requires live or exercise incident mode")
	}
	spec, ok := domain.ModelDraftSpecByID(req.SpecID)
	if !ok {
		return domain.ModelDraft{}, fmt.Errorf("unknown model draft spec %q", req.SpecID)
	}
	if !req.Lane.Valid() || !domain.ModelSpecAllowsLane(spec, req.Lane) {
		return domain.ModelDraft{}, fmt.Errorf("model lane %q is not allowed for spec %q", req.Lane, spec.ID)
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		return domain.ModelDraft{}, ErrModelQueueFull
	}

	compartment := req.Compartment
	if compartment == "" {
		compartment = "GENERAL"
	}
	retrieved, err := s.Retrieval.Retrieve(ctx, principal, req.IncidentID, compartment, req.Query, req.Limit)
	if err != nil {
		return domain.ModelDraft{}, err
	}
	draft := domain.ModelDraft{SpecID: spec.ID, PromptVersion: spec.PromptVersion, OutputType: spec.OutputType, Untrusted: true, Citations: retrieved.Citations}
	if retrieved.Refused {
		draft.Failure = retrieved.Reason
		return draft, ErrModelSourceUnsupported
	}

	lane, err := s.Backend.Lane(req.Lane)
	if err != nil {
		draft.Failure = err.Error()
		return draft, err
	}
	started := s.now()
	draft.Trace = domain.ModelTrace{
		Timestamp:     started.UTC(),
		RequestID:     fmt.Sprintf("model-%d-%d", started.UnixNano(), atomic.AddUint64(&modelRequestCounter, 1)),
		SpecID:        spec.ID,
		PromptVersion: spec.PromptVersion,
		Lane:          req.Lane,
		Provider:      lane.Provider,
		Model:         lane.Model,
		Managed:       lane.Managed,
		CitationIDs:   domain.ModelCitationIDs(retrieved.Citations),
	}

	if lane.Managed {
		scope := domain.Scope{IncidentID: req.IncidentID, Mode: req.Mode, Compartment: compartment}
		if s.ManagedPolicy == nil {
			return s.failDraft(draft, started, ErrManagedModelSensitivity)
		}
		allowed, policyErr := s.ManagedPolicy.AllowsManagedModel(ctx, principal, scope, retrieved.Citations)
		if policyErr != nil {
			return s.failDraft(draft, started, policyErr)
		}
		if !allowed {
			return s.failDraft(draft, started, ErrManagedModelSensitivity)
		}
		if d := authz.Authorise(principal, authz.ActionModelSend, authz.ObjectScope{IncidentID: req.IncidentID, Compartment: compartment}); !d.Allowed {
			return s.failDraft(draft, started, ErrAuthorisationDenied)
		}
	}

	prompt, promptHash, err := domain.BuildModelPrompt(spec, req.Instruction, retrieved.Citations)
	if err != nil {
		return s.failDraft(draft, started, err)
	}
	draft.Trace.PromptSHA256 = promptHash
	providerResult, err := s.Backend.Generate(ctx, req.Lane, prompt)
	draft.Trace.DurationMS = s.now().Sub(started).Milliseconds()
	draft.Trace.InputTokens = providerResult.InputTokens
	draft.Trace.OutputTokens = providerResult.OutputTokens
	if err != nil {
		draft.Trace.Error = err.Error()
		draft.Failure = err.Error()
		return draft, err
	}
	draft.Content = providerResult.Output
	if err := domain.ValidateModelOutput(spec, providerResult.Output); err != nil {
		draft.Trace.Error = err.Error()
		draft.Failure = err.Error()
		return draft, fmt.Errorf("%w: %v", ErrModelOutputInvalid, err)
	}
	draft.Trace.Success = true
	draft.Validated = true
	return draft, nil
}

func (s *ModelService) failDraft(draft domain.ModelDraft, started time.Time, err error) (domain.ModelDraft, error) {
	draft.Trace.DurationMS = s.now().Sub(started).Milliseconds()
	draft.Trace.Error = err.Error()
	draft.Failure = err.Error()
	return draft, err
}
