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

// CurrentReadiness reports only implemented capabilities. It deliberately does
// not claim production readiness before the durable store and security boundary
// have been built and proven.
func CurrentReadiness(cfg config.Config) Readiness {
	checks := []Check{
		{Name: "deployment_config", Status: "pass"},
		{Name: "portable_ledger_verifier", Status: "pass"},
		{Name: "postgresql", Status: "not-built", Message: "frozen Slice 1 durable-store work not yet implemented"},
		{Name: "authentication_authorisation", Status: "not-built", Message: "frozen Slice 2 security boundary not yet implemented"},
	}
	if cfg.DatabaseURL == "" {
		checks[2].Message = "CRAB_DATABASE_URL not configured; durable-store slice not yet implemented"
	}
	return Readiness{Ready: false, Checks: checks}
}
