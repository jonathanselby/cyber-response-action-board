package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	defaultListenAddr       = "127.0.0.1:8080"
	defaultPageSize         = "A4"
	defaultEvidenceMaxBytes = int64(500 << 20)
)

// Config contains deployment configuration only. Organisation business rules
// belong in governed application data and must not be smuggled in here.
type Config struct {
	DatabaseURL            string `json:"database_url,omitempty"`
	MaintenanceDatabaseURL string `json:"maintenance_database_url,omitempty"`
	ListenAddr             string `json:"listen_addr,omitempty"`
	PublicBaseURL          string `json:"public_base_url,omitempty"`
	TLSCertFile            string `json:"tls_cert_file,omitempty"`
	TLSKeyFile             string `json:"tls_key_file,omitempty"`
	SessionKeyFile         string `json:"session_key_file,omitempty"`
	ServerSigningKeyFile   string `json:"server_signing_key_file,omitempty"`
	LedgerKeyringFile      string `json:"ledger_keyring_file,omitempty"`
	CredentialWrapKeyFile  string `json:"credential_wrap_key_file,omitempty"`
	ExportDir              string `json:"export_dir,omitempty"`
	SecondaryExportDir     string `json:"secondary_export_dir,omitempty"`
	PageSizeDefault        string `json:"page_size_default,omitempty"`
	EvidenceMaxBytes       int64  `json:"evidence_max_bytes,omitempty"`
	OllamaURL              string `json:"ollama_url,omitempty"`
	ModelConfigFile        string `json:"model_config_file,omitempty"`
	NotificationConfigFile string `json:"notification_config_file,omitempty"`
}

func Defaults() Config {
	return Config{
		ListenAddr:       defaultListenAddr,
		PageSizeDefault:  defaultPageSize,
		EvidenceMaxBytes: defaultEvidenceMaxBytes,
	}
}

// Load is file-first, then applies deployment environment overrides.
func Load() (Config, error) {
	cfg := Defaults()
	if path := strings.TrimSpace(os.Getenv("CRAB_CONFIG_FILE")); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return Config{}, fmt.Errorf("read CRAB_CONFIG_FILE: %w", err)
		}
		dec := json.NewDecoder(strings.NewReader(string(b)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&cfg); err != nil {
			return Config{}, fmt.Errorf("parse CRAB_CONFIG_FILE: %w", err)
		}
	}

	applyString(&cfg.DatabaseURL, "CRAB_DATABASE_URL")
	applyString(&cfg.MaintenanceDatabaseURL, "CRAB_MAINTENANCE_DATABASE_URL")
	applyString(&cfg.ListenAddr, "CRAB_LISTEN_ADDR")
	applyString(&cfg.PublicBaseURL, "CRAB_PUBLIC_BASE_URL")
	applyString(&cfg.TLSCertFile, "CRAB_TLS_CERT_FILE")
	applyString(&cfg.TLSKeyFile, "CRAB_TLS_KEY_FILE")
	applyString(&cfg.SessionKeyFile, "CRAB_SESSION_KEY_FILE")
	applyString(&cfg.ServerSigningKeyFile, "CRAB_SERVER_SIGNING_KEY_FILE")
	applyString(&cfg.LedgerKeyringFile, "CRAB_LEDGER_KEYRING_FILE")
	applyString(&cfg.CredentialWrapKeyFile, "CRAB_CREDENTIAL_WRAP_KEY_FILE")
	applyString(&cfg.ExportDir, "CRAB_EXPORT_DIR")
	applyString(&cfg.SecondaryExportDir, "CRAB_SECONDARY_EXPORT_DIR")
	applyString(&cfg.PageSizeDefault, "CRAB_PAGE_SIZE_DEFAULT")
	applyString(&cfg.OllamaURL, "CRAB_OLLAMA_URL")
	applyString(&cfg.ModelConfigFile, "CRAB_MODEL_CONFIG_FILE")
	applyString(&cfg.NotificationConfigFile, "CRAB_NOTIFICATION_CONFIG_FILE")
	if raw := strings.TrimSpace(os.Getenv("CRAB_EVIDENCE_MAX_BYTES")); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("CRAB_EVIDENCE_MAX_BYTES: %w", err)
		}
		cfg.EvidenceMaxBytes = n
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func applyString(dst *string, key string) {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		*dst = v
	}
}

func (c Config) IsLoopbackListen() bool {
	host, _, err := net.SplitHostPort(c.ListenAddr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.ListenAddr) == "" {
		return errors.New("listen_addr must not be empty")
	}
	if _, _, err := net.SplitHostPort(c.ListenAddr); err != nil {
		return fmt.Errorf("listen_addr: %w", err)
	}
	hasCert := strings.TrimSpace(c.TLSCertFile) != ""
	hasKey := strings.TrimSpace(c.TLSKeyFile) != ""
	if hasCert != hasKey {
		return errors.New("tls_cert_file and tls_key_file must be configured together")
	}
	if !c.IsLoopbackListen() {
		if !hasCert || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(c.PublicBaseURL)), "https://") {
			return errors.New("non-loopback serve requires TLS certificate/key and an https public_base_url")
		}
	}
	if c.PageSizeDefault != "A4" && c.PageSizeDefault != "LETTER" {
		return errors.New("page_size_default must be A4 or LETTER")
	}
	if c.EvidenceMaxBytes <= 0 {
		return errors.New("evidence_max_bytes must be positive")
	}
	return nil
}
