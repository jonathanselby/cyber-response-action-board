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
		{Name: "access_governance_compartments", Status: "implemented", Message: "Slice 6 Sponsor/Controller -> Custodian governance, expiring incident/compartment access, metadata-only administration and selective-release scope are built; PostgreSQL RLS was spiked and deliberately not retained because it would require a second privileged projection/rebuild path"},
		{Name: "evidence_immutable_blobs", Status: "implemented", Message: "Slice 7 bounded PostgreSQL hostile-byte storage, SHA-256 integrity verification, opaque per-upload IDs, no cross-incident dedupe, scoped evidence register/history and fail-closed attachment presentation are built; authenticated browser route composition remains later presentation work"},
		{Name: "corpus_deterministic_retrieval", Status: "implemented", Message: "Slice 8 governed Markdown/DOCX corpus ingestion, immutable original blobs, bounded passive extraction, database scope filtering before deterministic lexical ranking, stable citations and unsupported-query refusal are built; no embedding model, vector database or network is required"},
	}
	return Readiness{Ready: false, Checks: checks}
}
