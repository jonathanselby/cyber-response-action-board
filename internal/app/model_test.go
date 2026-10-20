package app

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

type modelRetrievalBackend struct {
	candidates []domain.CorpusCandidate
}

func (b *modelRetrievalBackend) PutCorpusBlob(context.Context, domain.CorpusScope, string, string, io.Reader) (domain.BlobReceipt, error) {
	return domain.BlobReceipt{}, errors.New("not used")
}
func (b *modelRetrievalBackend) ValidateCorpusBlob(context.Context, string, string) error { return errors.New("not used") }
func (b *modelRetrievalBackend) AppendCorpus(context.Context, ledger.Record) error { return errors.New("not used") }
func (b *modelRetrievalBackend) PendingCorpusSource(context.Context, string) (ledger.Record, bool, error) {
	return ledger.Record{}, false, errors.New("not used")
}
func (b *modelRetrievalBackend) CorpusCandidates(context.Context, string, []string) ([]domain.CorpusCandidate, error) {
	return append([]domain.CorpusCandidate(nil), b.candidates...), nil
}

type fakeManagedModelPolicy struct {
	allowed bool
	err     error
	mode    string
}

func (p *fakeManagedModelPolicy) AllowsManagedModel(_ context.Context, _ authz.Principal, scope domain.Scope, _ []domain.RetrievalCitation) (bool, error) {
	p.mode = scope.Mode
	return p.allowed, p.err
}

type fakeModelBackend struct {
	mu      sync.Mutex
	lane    domain.ModelLanePolicy
	result  domain.ModelProviderResult
	err     error
	calls   int
	started chan struct{}
	release chan struct{}
}

func (b *fakeModelBackend) Lane(domain.ModelLane) (domain.ModelLanePolicy, error) { return b.lane, nil }
func (b *fakeModelBackend) Generate(ctx context.Context, _ domain.ModelLane, _ string) (domain.ModelProviderResult, error) {
	b.mu.Lock()
	b.calls++
	started := b.started
	release := b.release
	result := b.result
	err := b.err
	b.mu.Unlock()
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
			return domain.ModelProviderResult{}, ctx.Err()
		}
	}
	return result, err
}
func (b *fakeModelBackend) callCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls
}

func modelTestPrincipal(modelSend bool) authz.Principal {
	outputs := map[authz.Action]bool{}
	if modelSend {
		outputs[authz.ActionModelSend] = true
	}
	return authz.Principal{Authenticated: true, HumanID: "alice", Role: authz.RoleMinor, Incidents: map[string]bool{"INC-MODEL": true}, Compartments: map[string]map[string]bool{"INC-MODEL": {"GENERAL": true}}, Outputs: outputs}
}

func modelTestRetrieval() *RetrievalService {
	candidate := domain.CorpusCandidate{SourceID: "RUNBOOK", Title: "Containment", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", IncidentID: "", Compartment: "GENERAL", Locator: "lines:1-2", Passage: "Isolate affected ransomware hosts from the network.", Terms: domain.LexicalTerms("Isolate affected ransomware hosts from the network.")}
	return &RetrievalService{Backend: &modelRetrievalBackend{candidates: []domain.CorpusCandidate{candidate}}}
}

func modelDraftRequest() ModelDraftRequest {
	return ModelDraftRequest{SpecID: "retrieval-summary", Lane: domain.ModelLaneSimple, IncidentID: "INC-MODEL", Mode: "live", Query: "ransomware isolate", Instruction: "Summarise the containment step."}
}

func TestManagedModelSensitivityRefusesBeforeProviderCall(t *testing.T) {
	backend := &fakeModelBackend{lane: domain.ModelLanePolicy{Provider: "azure", Model: "managed-model", Managed: true}, result: domain.ModelProviderResult{Output: "should not run"}}
	policy := &fakeManagedModelPolicy{allowed: false}
	service, err := NewModelService(modelTestRetrieval(), backend, policy, 1)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := service.Draft(context.Background(), modelTestPrincipal(true), modelDraftRequest())
	if !errors.Is(err, ErrManagedModelSensitivity) || backend.callCount() != 0 || len(draft.Citations) != 1 || !draft.Untrusted || policy.mode != "live" {
		t.Fatalf("managed sensitivity gate err=%v calls=%d mode=%q draft=%+v", err, backend.callCount(), policy.mode, draft)
	}
}

func TestManagedModelRequiresSeparateSendPermission(t *testing.T) {
	backend := &fakeModelBackend{lane: domain.ModelLanePolicy{Provider: "bedrock", Model: "managed-model", Managed: true}, result: domain.ModelProviderResult{Output: "draft"}}
	policy := &fakeManagedModelPolicy{allowed: true}
	service, err := NewModelService(modelTestRetrieval(), backend, policy, 1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Draft(context.Background(), modelTestPrincipal(false), modelDraftRequest())
	if !errors.Is(err, ErrAuthorisationDenied) || backend.callCount() != 0 {
		t.Fatalf("managed send permission err=%v calls=%d", err, backend.callCount())
	}
}

func TestModelUnavailableKeepsSourcesAndDoesNotReroute(t *testing.T) {
	backend := &fakeModelBackend{lane: domain.ModelLanePolicy{Provider: "ollama", Model: "local-model", Managed: false}, err: errors.New("model unavailable")}
	service, err := NewModelService(modelTestRetrieval(), backend, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := service.Draft(context.Background(), modelTestPrincipal(false), modelDraftRequest())
	if err == nil || backend.callCount() != 1 || len(draft.Citations) != 1 || draft.Failure == "" || draft.Trace.Provider != "ollama" || draft.Accepted {
		t.Fatalf("unavailable model calls=%d err=%v draft=%+v", backend.callCount(), err, draft)
	}
}

func TestModelOutputValidationKeepsUntrustedTextAndSources(t *testing.T) {
	backend := &fakeModelBackend{lane: domain.ModelLanePolicy{Provider: "ollama", Model: "local-model", Managed: false}, result: domain.ModelProviderResult{Output: "   ", InputTokens: 12, OutputTokens: 1}}
	service, err := NewModelService(modelTestRetrieval(), backend, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := service.Draft(context.Background(), modelTestPrincipal(false), modelDraftRequest())
	if !errors.Is(err, ErrModelOutputInvalid) || draft.Content != "   " || len(draft.Citations) != 1 || !draft.Untrusted || draft.Accepted || draft.Trace.InputTokens != 12 {
		t.Fatalf("invalid output err=%v draft=%+v", err, draft)
	}
}

func TestModelQueueShedsNewGenerationWithoutBlockingCoreWork(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	backend := &fakeModelBackend{lane: domain.ModelLanePolicy{Provider: "ollama", Model: "local-model", Managed: false}, result: domain.ModelProviderResult{Output: "draft"}, started: started, release: release}
	service, err := NewModelService(modelTestRetrieval(), backend, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	principal := modelTestPrincipal(false)
	req := modelDraftRequest()
	firstDone := make(chan error, 1)
	go func() {
		_, err := service.Draft(context.Background(), principal, req)
		firstDone <- err
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("first model generation did not start")
	}
	if _, err := service.Draft(context.Background(), principal, req); !errors.Is(err, ErrModelQueueFull) {
		t.Fatalf("second model request was not shed: %v", err)
	}
	result, err := service.Retrieval.Retrieve(context.Background(), principal, "INC-MODEL", "GENERAL", "ransomware isolate", 5)
	if err != nil || result.Refused || len(result.Citations) != 1 {
		t.Fatalf("deterministic retrieval blocked by model saturation result=%+v err=%v", result, err)
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
}

func TestModelDraftIsAlwaysUntrustedAndDoesNotBecomeGovernedState(t *testing.T) {
	backend := &fakeModelBackend{lane: domain.ModelLanePolicy{Provider: "ollama", Model: "local-model", Managed: false}, result: domain.ModelProviderResult{Output: "Suggested containment wording.", InputTokens: 20, OutputTokens: 5}}
	service, err := NewModelService(modelTestRetrieval(), backend, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := service.Draft(context.Background(), modelTestPrincipal(false), modelDraftRequest())
	if err != nil || !draft.Untrusted || !draft.Accepted || draft.Trace.PromptSHA256 == "" || len(draft.Trace.CitationIDs) != 1 || draft.Trace.Provider != "ollama" || draft.Trace.Success != true {
		t.Fatalf("draft=%+v err=%v", draft, err)
	}
	if draft.OutputType != "untrusted_summary" {
		t.Fatalf("model output exposed as governed type: %q", draft.OutputType)
	}
}

func TestModelPolicyReceivesExerciseMode(t *testing.T) {
	backend := &fakeModelBackend{lane: domain.ModelLanePolicy{Provider: "vertex", Model: "managed-model", Managed: true}, result: domain.ModelProviderResult{Output: "draft"}}
	policy := &fakeManagedModelPolicy{allowed: true}
	service, err := NewModelService(modelTestRetrieval(), backend, policy, 1)
	if err != nil {
		t.Fatal(err)
	}
	req := modelDraftRequest()
	req.Mode = "exercise"
	if _, err := service.Draft(context.Background(), modelTestPrincipal(true), req); err != nil {
		t.Fatal(err)
	}
	if policy.mode != "exercise" {
		t.Fatalf("managed model policy received mode %q", policy.mode)
	}
}
