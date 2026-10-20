package models

import (
	"context"
	"errors"
	"fmt"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

type Provider interface {
	Name() string
	Generate(context.Context, string, string) (domain.ModelProviderResult, error)
}

type Lane struct {
	Provider string
	Model    string
	Managed  bool
}

type Config struct {
	Simple   Lane
	Standard Lane
	Complex  Lane
}

type Router struct {
	config    Config
	providers map[string]Provider
}

func NewRouter(config Config, providers ...Provider) (*Router, error) {
	registry := make(map[string]Provider, len(providers))
	for _, provider := range providers {
		if provider == nil {
			return nil, errors.New("model provider cannot be nil")
		}
		name := provider.Name()
		if name == "" {
			return nil, errors.New("model provider name cannot be empty")
		}
		if _, exists := registry[name]; exists {
			return nil, fmt.Errorf("duplicate model provider %q", name)
		}
		registry[name] = provider
	}
	return &Router{config: config, providers: registry}, nil
}

func (r *Router) Lane(lane domain.ModelLane) (domain.ModelLanePolicy, error) {
	configured, err := r.configuredLane(lane)
	if err != nil {
		return domain.ModelLanePolicy{}, err
	}
	if configured.Provider == "" || configured.Model == "" {
		return domain.ModelLanePolicy{}, fmt.Errorf("model lane %q is not fully configured", lane)
	}
	if _, ok := r.providers[configured.Provider]; !ok {
		return domain.ModelLanePolicy{}, fmt.Errorf("model provider %q is not registered", configured.Provider)
	}
	return domain.ModelLanePolicy{Provider: configured.Provider, Model: configured.Model, Managed: configured.Managed}, nil
}

func (r *Router) Generate(ctx context.Context, lane domain.ModelLane, prompt string) (domain.ModelProviderResult, error) {
	configured, err := r.configuredLane(lane)
	if err != nil {
		return domain.ModelProviderResult{}, err
	}
	if configured.Provider == "" || configured.Model == "" {
		return domain.ModelProviderResult{}, fmt.Errorf("model lane %q is not fully configured", lane)
	}
	provider, ok := r.providers[configured.Provider]
	if !ok {
		return domain.ModelProviderResult{}, fmt.Errorf("model provider %q is not registered", configured.Provider)
	}
	return provider.Generate(ctx, configured.Model, prompt)
}

func (r *Router) configuredLane(lane domain.ModelLane) (Lane, error) {
	switch lane {
	case domain.ModelLaneSimple:
		return r.config.Simple, nil
	case domain.ModelLaneStandard:
		return r.config.Standard, nil
	case domain.ModelLaneComplex:
		return r.config.Complex, nil
	default:
		return Lane{}, fmt.Errorf("invalid model lane %q", lane)
	}
}
