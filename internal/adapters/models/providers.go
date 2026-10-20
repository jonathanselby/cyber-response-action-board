package models

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

const maxProviderResponseBytes = int64(8 << 20)

func readProviderBody(r io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxProviderResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxProviderResponseBytes {
		return nil, errors.New("model provider response exceeds limit")
	}
	return body, nil
}

type OllamaProvider struct {
	BaseURL string
	Client  *http.Client
}

func (OllamaProvider) Name() string { return "ollama" }

func (p OllamaProvider) Generate(ctx context.Context, model, prompt string) (domain.ModelProviderResult, error) {
	base := strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
	if base == "" {
		base = "http://localhost:11434"
	}
	payload, err := json.Marshal(struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
		Stream bool   `json:"stream"`
	}{Model: model, Prompt: prompt, Stream: false})
	if err != nil {
		return domain.ModelProviderResult{}, fmt.Errorf("encode ollama request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/generate", bytes.NewReader(payload))
	if err != nil {
		return domain.ModelProviderResult{}, fmt.Errorf("build ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := providerClient(p.Client).Do(req)
	if err != nil {
		return domain.ModelProviderResult{}, fmt.Errorf("call ollama: %w", err)
	}
	defer resp.Body.Close()
	body, err := readProviderBody(resp.Body)
	if err != nil {
		return domain.ModelProviderResult{}, fmt.Errorf("read ollama response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return domain.ModelProviderResult{}, fmt.Errorf("ollama returned %s", resp.Status)
	}
	var decoded struct {
		Response        string `json:"response"`
		PromptEvalCount int    `json:"prompt_eval_count"`
		EvalCount       int    `json:"eval_count"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return domain.ModelProviderResult{}, fmt.Errorf("decode ollama response: %w", err)
	}
	if strings.TrimSpace(decoded.Response) == "" {
		return domain.ModelProviderResult{}, errors.New("ollama response contained no text output")
	}
	return domain.ModelProviderResult{Output: decoded.Response, InputTokens: decoded.PromptEvalCount, OutputTokens: decoded.EvalCount}, nil
}

type AzureProvider struct {
	Endpoint    string
	APIKey      string
	BearerToken string
	Client      *http.Client
}

func (AzureProvider) Name() string { return "azure" }

func (p AzureProvider) Generate(ctx context.Context, model, prompt string) (domain.ModelProviderResult, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(p.Endpoint), "/")
	if endpoint == "" {
		return domain.ModelProviderResult{}, errors.New("azure endpoint is required")
	}
	if p.APIKey == "" && p.BearerToken == "" {
		return domain.ModelProviderResult{}, errors.New("azure authentication is required")
	}
	if !strings.HasSuffix(endpoint, "/openai/v1") {
		endpoint += "/openai/v1"
	}
	payload, err := json.Marshal(struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}{Model: model, Messages: []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}{{Role: "user", Content: prompt}}})
	if err != nil {
		return domain.ModelProviderResult{}, fmt.Errorf("encode azure request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return domain.ModelProviderResult{}, fmt.Errorf("build azure request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+p.BearerToken)
	} else {
		req.Header.Set("api-key", p.APIKey)
	}
	resp, err := providerClient(p.Client).Do(req)
	if err != nil {
		return domain.ModelProviderResult{}, fmt.Errorf("call azure: %w", err)
	}
	defer resp.Body.Close()
	body, err := readProviderBody(resp.Body)
	if err != nil {
		return domain.ModelProviderResult{}, fmt.Errorf("read azure response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return domain.ModelProviderResult{}, fmt.Errorf("azure returned %s", resp.Status)
	}
	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return domain.ModelProviderResult{}, fmt.Errorf("decode azure response: %w", err)
	}
	if len(decoded.Choices) == 0 || strings.TrimSpace(decoded.Choices[0].Message.Content) == "" {
		return domain.ModelProviderResult{}, errors.New("azure response contained no text output")
	}
	return domain.ModelProviderResult{Output: decoded.Choices[0].Message.Content, InputTokens: decoded.Usage.PromptTokens, OutputTokens: decoded.Usage.CompletionTokens}, nil
}

type BedrockProvider struct {
	Region      string
	BaseURL     string
	BearerToken string
	Client      *http.Client
}

func (BedrockProvider) Name() string { return "bedrock" }

func (p BedrockProvider) Generate(ctx context.Context, model, prompt string) (domain.ModelProviderResult, error) {
	if strings.TrimSpace(p.BearerToken) == "" {
		return domain.ModelProviderResult{}, errors.New("bedrock bearer token is required")
	}
	base := strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
	if base == "" {
		if strings.TrimSpace(p.Region) == "" {
			return domain.ModelProviderResult{}, errors.New("bedrock region is required")
		}
		base = "https://bedrock-runtime." + strings.TrimSpace(p.Region) + ".amazonaws.com"
	}
	payload, err := json.Marshal(map[string]any{"messages": []any{map[string]any{"role": "user", "content": []any{map[string]any{"text": prompt}}}}})
	if err != nil {
		return domain.ModelProviderResult{}, fmt.Errorf("encode bedrock request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/model/"+url.PathEscape(model)+"/converse", bytes.NewReader(payload))
	if err != nil {
		return domain.ModelProviderResult{}, fmt.Errorf("build bedrock request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.BearerToken)
	resp, err := providerClient(p.Client).Do(req)
	if err != nil {
		return domain.ModelProviderResult{}, fmt.Errorf("call bedrock: %w", err)
	}
	defer resp.Body.Close()
	body, err := readProviderBody(resp.Body)
	if err != nil {
		return domain.ModelProviderResult{}, fmt.Errorf("read bedrock response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return domain.ModelProviderResult{}, fmt.Errorf("bedrock returned %s", resp.Status)
	}
	var decoded struct {
		Output struct {
			Message struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		} `json:"output"`
		Usage struct {
			InputTokens  int `json:"inputTokens"`
			OutputTokens int `json:"outputTokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return domain.ModelProviderResult{}, fmt.Errorf("decode bedrock response: %w", err)
	}
	for _, block := range decoded.Output.Message.Content {
		if strings.TrimSpace(block.Text) != "" {
			return domain.ModelProviderResult{Output: block.Text, InputTokens: decoded.Usage.InputTokens, OutputTokens: decoded.Usage.OutputTokens}, nil
		}
	}
	return domain.ModelProviderResult{}, errors.New("bedrock response contained no text output")
}

type VertexProvider struct {
	ProjectID   string
	Location    string
	BaseURL     string
	AccessToken string
	Client      *http.Client
}

func (VertexProvider) Name() string { return "vertex" }

func (p VertexProvider) Generate(ctx context.Context, model, prompt string) (domain.ModelProviderResult, error) {
	project := strings.TrimSpace(p.ProjectID)
	if project == "" {
		return domain.ModelProviderResult{}, errors.New("vertex project ID is required")
	}
	location := strings.TrimSpace(p.Location)
	if location == "" {
		location = "global"
	}
	if strings.TrimSpace(p.AccessToken) == "" {
		return domain.ModelProviderResult{}, errors.New("vertex access token is required")
	}
	base := strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
	if base == "" {
		if location == "global" {
			base = "https://aiplatform.googleapis.com"
		} else {
			base = "https://" + location + "-aiplatform.googleapis.com"
		}
	}
	payload, err := json.Marshal(map[string]any{"contents": []any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": prompt}}}}})
	if err != nil {
		return domain.ModelProviderResult{}, fmt.Errorf("encode vertex request: %w", err)
	}
	endpoint := fmt.Sprintf("%s/v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent", base, url.PathEscape(project), url.PathEscape(location), url.PathEscape(model))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return domain.ModelProviderResult{}, fmt.Errorf("build vertex request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.AccessToken)
	resp, err := providerClient(p.Client).Do(req)
	if err != nil {
		return domain.ModelProviderResult{}, fmt.Errorf("call vertex: %w", err)
	}
	defer resp.Body.Close()
	body, err := readProviderBody(resp.Body)
	if err != nil {
		return domain.ModelProviderResult{}, fmt.Errorf("read vertex response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return domain.ModelProviderResult{}, fmt.Errorf("vertex returned %s", resp.Status)
	}
	var decoded struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return domain.ModelProviderResult{}, fmt.Errorf("decode vertex response: %w", err)
	}
	for _, candidate := range decoded.Candidates {
		for _, part := range candidate.Content.Parts {
			if strings.TrimSpace(part.Text) != "" {
				return domain.ModelProviderResult{Output: part.Text, InputTokens: decoded.UsageMetadata.PromptTokenCount, OutputTokens: decoded.UsageMetadata.CandidatesTokenCount}, nil
			}
		}
	}
	return domain.ModelProviderResult{}, errors.New("vertex response contained no text output")
}

func providerClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return http.DefaultClient
}
