package models

import (
	"context"
	"errors"
	"testing"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

type stubProvider struct {
	name   string
	result domain.ModelProviderResult
	err    error
	calls  int
	model  string
	prompt string
}

func (p *stubProvider) Name() string { return p.name }
func (p *stubProvider) Generate(_ context.Context, model, prompt string) (domain.ModelProviderResult, error) {
	p.calls++
	p.model = model
	p.prompt = prompt
	return p.result, p.err
}

func TestModelRouterUsesExactlyConfiguredLaneWithoutReroute(t *testing.T) {
	simple := &stubProvider{name: "simple", err: errors.New("unavailable")}
	standard := &stubProvider{name: "standard", result: domain.ModelProviderResult{Output: "wrong lane"}}
	router, err := NewRouter(Config{
		Simple:   Lane{Provider: "simple", Model: "small", Managed: false},
		Standard: Lane{Provider: "standard", Model: "medium", Managed: true},
		Complex:  Lane{Provider: "standard", Model: "large", Managed: true},
	}, simple, standard)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := router.Lane(domain.ModelLaneSimple)
	if err != nil || policy.Provider != "simple" || policy.Model != "small" || policy.Managed {
		t.Fatalf("simple lane policy=%+v err=%v", policy, err)
	}
	if _, err := router.Generate(context.Background(), domain.ModelLaneSimple, "draft"); err == nil {
		t.Fatal("failed configured lane unexpectedly succeeded")
	}
	if simple.calls != 1 || standard.calls != 0 {
		t.Fatalf("automatic reroute occurred simple=%d standard=%d", simple.calls, standard.calls)
	}
}

func TestModelRouterRequiresRegisteredFullyConfiguredProvider(t *testing.T) {
	provider := &stubProvider{name: "local"}
	router, err := NewRouter(Config{Simple: Lane{Provider: "local", Model: "qwen", Managed: false}}, provider)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := router.Lane(domain.ModelLaneSimple); err != nil {
		t.Fatal(err)
	}
	if _, err := router.Lane(domain.ModelLaneStandard); err == nil {
		t.Fatal("empty standard lane unexpectedly accepted")
	}
	bad, err := NewRouter(Config{Simple: Lane{Provider: "missing", Model: "x"}}, provider)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bad.Lane(domain.ModelLaneSimple); err == nil {
		t.Fatal("unregistered provider unexpectedly accepted")
	}
}
