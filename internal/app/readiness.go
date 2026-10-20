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

// CurrentReadiness reports implemented capability without inventing runtime
// state. PostgreSQL, ledger and installation-state checks are enriched by the
// composition root when a live deployment is available.
func CurrentReadiness(cfg config.Config) Readiness {
	postgres := Check{Name: "postgresql", Status: "not-configured", Message: "CRAB_DATABASE_URL and CRAB_LEDGER_KEYRING_FILE are required for durable-store verification"}
	if cfg.DatabaseURL != "" && cfg.LedgerKeyringFile != "" {
		postgres = Check{Name: "postgresql", Status: "pending", Message: "runtime verification pending"}
	}
	checks := []Check{
		{Name: "deployment_config", Status: "pass"},
		{Name: "portable_ledger_verifier", Status: "pass"},
		postgres,
		{Name: "authentication_authorisation", Status: "implemented", Message: "Slice 2 security kernel, projections and ceremonies are built; live installation state still requires secure setup wiring"},
		{Name: "incident_semantic_kernel", Status: "implemented", Message: "Slice 3 Record/Propose/Decide application kernel and rebuildable common-operating-picture projections are built"},
		{Name: "actions_checklists_timeline", Status: "implemented", Message: "Slice 4 governed Action lifecycle, configured checklist activation, explicit incident timeline and dedicated Action/timeline renderers are built; authenticated browser routing remains later presentation work"},
		{Name: "obligations_timed_waits", Status: "implemented", Message: "Slice 5 governed organisation-defined obligations, deterministic provisional/firm clocks, amendments and bounded rebuildable timed wait list are built; notifier delivery and authenticated browser UX remain later work"},
	}
	return Readiness{Ready: false, Checks: checks}
}
