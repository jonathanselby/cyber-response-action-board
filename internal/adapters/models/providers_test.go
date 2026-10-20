package models

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOllamaProviderWireContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" || r.Method != http.MethodPost {
			t.Fatalf("request %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["model"] != "local-model" || body["prompt"] != "hello" || body["stream"] != false {
			t.Fatalf("body=%v", body)
		}
		_, _ = w.Write([]byte(`{"response":"local draft","prompt_eval_count":7,"eval_count":3}`))
	}))
	defer server.Close()
	result, err := (OllamaProvider{BaseURL: server.URL, Client: server.Client()}).Generate(context.Background(), "local-model", "hello")
	if err != nil || result.Output != "local draft" || result.InputTokens != 7 || result.OutputTokens != 3 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestAzureProviderWireContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openai/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("path=%q auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"azure draft"}}],"usage":{"prompt_tokens":11,"completion_tokens":5}}`))
	}))
	defer server.Close()
	result, err := (AzureProvider{Endpoint: server.URL, BearerToken: "token", Client: server.Client()}).Generate(context.Background(), "deployment", "hello")
	if err != nil || result.Output != "azure draft" || result.InputTokens != 11 || result.OutputTokens != 5 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestBedrockProviderWireContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/converse") || r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("path=%q auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"output":{"message":{"content":[{"text":"bedrock draft"}]}},"usage":{"inputTokens":13,"outputTokens":6}}`))
	}))
	defer server.Close()
	result, err := (BedrockProvider{BaseURL: server.URL, BearerToken: "token", Client: server.Client()}).Generate(context.Background(), "model/id", "hello")
	if err != nil || result.Output != "bedrock draft" || result.InputTokens != 13 || result.OutputTokens != 6 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestVertexProviderWireContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/projects/project/locations/australia-southeast1/publishers/google/models/model:generateContent") || r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("path=%q auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"vertex draft"}]}}],"usageMetadata":{"promptTokenCount":17,"candidatesTokenCount":8}}`))
	}))
	defer server.Close()
	result, err := (VertexProvider{ProjectID: "project", Location: "australia-southeast1", BaseURL: server.URL, AccessToken: "token", Client: server.Client()}).Generate(context.Background(), "model", "hello")
	if err != nil || result.Output != "vertex draft" || result.InputTokens != 17 || result.OutputTokens != 8 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestManagedProviderErrorsDoNotEchoResponseBodies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("sensitive provider body"))
	}))
	defer server.Close()
	_, err := (AzureProvider{Endpoint: server.URL, APIKey: "key", Client: server.Client()}).Generate(context.Background(), "model", "hello")
	if err == nil || strings.Contains(err.Error(), "sensitive provider body") {
		t.Fatalf("provider error leaked response body: %v", err)
	}
}
