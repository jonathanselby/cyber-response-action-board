package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFileThenEnvironmentOverride(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "crab.json")
	if err := os.WriteFile(p, []byte(`{"listen_addr":"127.0.0.1:9000","page_size_default":"LETTER","evidence_max_bytes":1024}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CRAB_CONFIG_FILE", p)
	t.Setenv("CRAB_LISTEN_ADDR", "127.0.0.1:9100")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != "127.0.0.1:9100" {
		t.Fatalf("listen override = %q", cfg.ListenAddr)
	}
	if cfg.PageSizeDefault != "LETTER" || cfg.EvidenceMaxBytes != 1024 {
		t.Fatalf("file settings lost: %+v", cfg)
	}
}

func TestLoadRejectsUnknownDeploymentKey(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "crab.json")
	if err := os.WriteFile(p, []byte(`{"incident_severity":"critical"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CRAB_CONFIG_FILE", p)
	if _, err := Load(); err == nil {
		t.Fatal("expected unknown deployment key to be rejected")
	}
}

func TestNonLoopbackRequiresHTTPSAndTLS(t *testing.T) {
	cfg := Defaults()
	cfg.ListenAddr = "0.0.0.0:8080"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected non-loopback bind without TLS to be refused")
	}
	cfg.PublicBaseURL = "https://crab.example.test"
	cfg.TLSCertFile = "/etc/crab/tls.crt"
	cfg.TLSKeyFile = "/etc/crab/tls.key"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("secure non-loopback config refused: %v", err)
	}
}

func TestTLSFilesMustBeConfiguredTogether(t *testing.T) {
	cfg := Defaults()
	cfg.TLSCertFile = "/etc/crab/tls.crt"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected certificate without key to be refused")
	}
}

func TestLoopbackHTTPRemainsAvailableForLocalBootstrap(t *testing.T) {
	cfg := Defaults()
	cfg.ListenAddr = "127.0.0.1:8080"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("loopback bootstrap should remain available: %v", err)
	}
}
