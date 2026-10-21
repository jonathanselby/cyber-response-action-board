package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jonathanselby/cyber-response-action-board/internal/config"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

func writePrivateDeploymentFile(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestModelDeploymentComposesThreeLanes(t *testing.T) {
	t.Setenv("CRAB_TEST_MODEL_AUTH", "test-value")
	path := writePrivateDeploymentFile(t, "models.json", `{
  "simple":{"provider":"ollama","model":"local-model"},
  "standard":{"provider":"azure","model":"standard-model"},
  "complex":{"provider":"azure","model":"complex-model"},
  "providers":{
    "ollama":{"base_url":"http://127.0.0.1:11434"},
    "azure":{"endpoint":"https://model.example.test","auth_env":"CRAB_TEST_MODEL_AUTH"}
  }
}`)
	router, err := loadModelDeployment(config.Config{ModelConfigFile: path})
	if err != nil {
		t.Fatal(err)
	}
	local, err := router.Lane(domain.ModelLaneSimple)
	if err != nil {
		t.Fatal(err)
	}
	managed, err := router.Lane(domain.ModelLaneStandard)
	if err != nil {
		t.Fatal(err)
	}
	if local.Managed {
		t.Fatal("ollama lane must remain local")
	}
	if !managed.Managed || managed.Provider != "azure" {
		t.Fatalf("managed lane=%+v", managed)
	}
}

func TestModelDeploymentRefusesRemoteOllama(t *testing.T) {
	path := writePrivateDeploymentFile(t, "models.json", `{
  "simple":{"provider":"ollama","model":"a"},
  "standard":{"provider":"ollama","model":"b"},
  "complex":{"provider":"ollama","model":"c"},
  "providers":{"ollama":{"base_url":"https://models.example.test"}}
}`)
	if _, err := loadModelDeployment(config.Config{ModelConfigFile: path}); err == nil {
		t.Fatal("expected remote local-lane endpoint to be refused")
	}
}

func TestNotificationDeploymentDerivesExternalBoundary(t *testing.T) {
	path := writePrivateDeploymentFile(t, "notify.json", `{
  "channels":[
    {"name":"local","type":"local_file","target":"/tmp/crab-notify.jsonl","destination_id":"local-audit"},
    {"name":"external","type":"webhook","target":"https://notify.example.test/crab","destination_id":"external-api","timeout_seconds":5}
  ]
}`)
	channels, err := loadNotificationDeployment(config.Config{NotificationConfigFile: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 2 {
		t.Fatalf("channels=%d", len(channels))
	}
	for _, channel := range channels {
		if channel.Type == "local_file" && channel.External {
			t.Fatal("local file channel cannot become external")
		}
		if channel.Type == "webhook" && !channel.External {
			t.Fatal("webhook must be treated as external")
		}
	}
}

func TestDeploymentFilesMustBePrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notify.json")
	if err := os.WriteFile(path, []byte(`{"channels":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadNotificationDeployment(config.Config{NotificationConfigFile: path}); err == nil {
		t.Fatal("expected group-readable deployment file to be refused")
	}
}
