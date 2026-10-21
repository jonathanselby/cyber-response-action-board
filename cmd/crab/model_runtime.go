package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/jonathanselby/cyber-response-action-board/internal/adapters/models"
	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/config"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

type modelLaneDeployment struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type modelProviderDeployment struct {
	BaseURL   string `json:"base_url,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`
	Region    string `json:"region,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	Location  string `json:"location,omitempty"`
	AuthEnv   string `json:"auth_env,omitempty"`
	AuthKind  string `json:"auth_kind,omitempty"`
}

type modelDeploymentFile struct {
	Simple    modelLaneDeployment                `json:"simple"`
	Standard  modelLaneDeployment                `json:"standard"`
	Complex   modelLaneDeployment                `json:"complex"`
	Providers map[string]modelProviderDeployment `json:"providers"`
}

func loadModelDeployment(cfg config.Config) (*models.Router, error) {
	if strings.TrimSpace(cfg.ModelConfigFile) == "" {
		return nil, nil
	}
	var file modelDeploymentFile
	if err := readPrivateJSONFile(cfg.ModelConfigFile, &file); err != nil {
		return nil, fmt.Errorf("model deployment config: %w", err)
	}
	lanes := models.Config{Simple: deploymentLane(file.Simple), Standard: deploymentLane(file.Standard), Complex: deploymentLane(file.Complex)}
	used := map[string]bool{}
	for _, lane := range []models.Lane{lanes.Simple, lanes.Standard, lanes.Complex} {
		if lane.Provider == "" || lane.Model == "" {
			return nil, errors.New("model deployment requires simple, standard and complex provider/model lanes")
		}
		used[lane.Provider] = true
	}
	providers := make([]models.Provider, 0, len(used))
	for name := range used {
		p := file.Providers[name]
		switch name {
		case "ollama":
			baseURL := strings.TrimSpace(cfg.OllamaURL)
			if baseURL == "" {
				baseURL = strings.TrimSpace(p.BaseURL)
			}
			if err := validateLocalModelEndpoint(baseURL); err != nil {
				return nil, err
			}
			providers = append(providers, models.OllamaProvider{BaseURL: baseURL})
		case "azure":
			if err := validateManagedEndpoint(p.Endpoint); err != nil {
				return nil, fmt.Errorf("azure endpoint: %w", err)
			}
			auth, err := deploymentAuth(p.AuthEnv)
			if err != nil {
				return nil, err
			}
			if strings.EqualFold(strings.TrimSpace(p.AuthKind), "bearer") {
				providers = append(providers, models.AzureProvider{Endpoint: p.Endpoint, BearerToken: auth})
			} else {
				providers = append(providers, models.AzureProvider{Endpoint: p.Endpoint, APIKey: auth})
			}
		case "bedrock":
			if strings.TrimSpace(p.Region) == "" && strings.TrimSpace(p.BaseURL) == "" {
				return nil, errors.New("bedrock model deployment requires region or base URL")
			}
			if strings.TrimSpace(p.BaseURL) != "" {
				if err := validateManagedEndpoint(p.BaseURL); err != nil {
					return nil, fmt.Errorf("bedrock base URL: %w", err)
				}
			}
			auth, err := deploymentAuth(p.AuthEnv)
			if err != nil {
				return nil, err
			}
			providers = append(providers, models.BedrockProvider{Region: p.Region, BaseURL: p.BaseURL, BearerToken: auth})
		case "vertex":
			if strings.TrimSpace(p.ProjectID) == "" {
				return nil, errors.New("vertex model deployment requires project ID")
			}
			if strings.TrimSpace(p.BaseURL) != "" {
				if err := validateManagedEndpoint(p.BaseURL); err != nil {
					return nil, fmt.Errorf("vertex base URL: %w", err)
				}
			}
			auth, err := deploymentAuth(p.AuthEnv)
			if err != nil {
				return nil, err
			}
			providers = append(providers, models.VertexProvider{ProjectID: p.ProjectID, Location: p.Location, BaseURL: p.BaseURL, AccessToken: auth})
		default:
			return nil, fmt.Errorf("unsupported model provider %q", name)
		}
	}
	router, err := models.NewRouter(lanes, providers...)
	if err != nil {
		return nil, err
	}
	for _, lane := range []domain.ModelLane{domain.ModelLaneSimple, domain.ModelLaneStandard, domain.ModelLaneComplex} {
		if _, err := router.Lane(lane); err != nil {
			return nil, err
		}
	}
	return router, nil
}

func deploymentAuth(envName string) (string, error) {
	envName = strings.TrimSpace(envName)
	if envName == "" {
		return "", errors.New("managed model provider requires auth_env")
	}
	value := strings.TrimSpace(os.Getenv(envName))
	if value == "" {
		return "", fmt.Errorf("managed model authentication environment %q is empty", envName)
	}
	return value, nil
}

func deploymentLane(value modelLaneDeployment) models.Lane {
	provider := strings.ToLower(strings.TrimSpace(value.Provider))
	return models.Lane{Provider: provider, Model: strings.TrimSpace(value.Model), Managed: provider != "" && provider != "ollama"}
}

func validateLocalModelEndpoint(value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("ollama endpoint must be an HTTP(S) loopback URL")
	}
	host := strings.TrimSpace(u.Hostname())
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("ollama endpoint must remain loopback-only so a local lane cannot bypass managed-provider controls")
	}
	return nil
}

func validateManagedEndpoint(value string) error {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || u.Host == "" || u.Scheme != "https" {
		return errors.New("managed provider endpoint must use HTTPS")
	}
	return nil
}

func modelDeploymentReadiness(cfg config.Config) app.Check {
	if strings.TrimSpace(cfg.ModelConfigFile) == "" {
		return app.Check{Name: "model_lanes", Status: "warn", Message: "model deployment is not configured; model assistance is optional and deterministic core remains available"}
	}
	if _, err := loadModelDeployment(cfg); err != nil {
		return app.Check{Name: "model_lanes", Status: "fail", Message: err.Error()}
	}
	return app.Check{Name: "model_lanes", Status: "pass", Message: "three model lanes and referenced providers are configured; managed-provider I/O remains separately authorised"}
}
