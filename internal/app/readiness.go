package app

import "github.com/jonathanselby/cyber-response-action-board/internal/config"

type Check struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type Readiness struct {
	Ready  bool    `json:"ready"`
	Checks []Check `json:"checks"`
}

// CurrentReadiness reports only implemented capabilities. PostgreSQL runtime
// verification is filled by the composition root because the application tier
// does not import the concrete data adapter.
func CurrentReadiness(cfg config.Config) Readiness {
	postgres := Check{Name: "postgresql", Status: "not-configured", Message: "CRAB_DATABASE_URL and CRAB_LEDGER_KEYRING_FILE are required for durable-store verification"}
	if cfg.DatabaseURL != "" && cfg.LedgerKeyringFile != "" {
		postgres = Check{Name: "postgresql", Status: "pending", Message: "runtime verification pending"}
	}
	checks := []Check{
		{Name: "deployment_config", Status: "pass"},
		{Name: "portable_ledger_verifier", Status: "pass"},
		postgres,
		{Name: "authentication_authorisation", Status: "not-built", Message: "frozen Slice 2 security boundary not yet implemented"},
	}
	return Readiness{Ready: false, Checks: checks}
}
