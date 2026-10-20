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

// CurrentReadiness reports built capability without inventing deployment state.
// Live deployment checks are enriched by the composition root.
func CurrentReadiness(cfg config.Config) Readiness {
	postgres := Check{Name: "postgresql", Status: "not-configured", Message: "CRAB_DATABASE_URL and CRAB_LEDGER_KEYRING_FILE are required for durable-store verification"}
	if cfg.DatabaseURL != "" && cfg.LedgerKeyringFile != "" {
		postgres = Check{Name: "postgresql", Status: "pending", Message: "runtime verification pending"}
	}
	return Readiness{Checks: []Check{
		{Name: "deployment_config", Status: "pass"},
		{Name: "portable_ledger_verifier", Status: "pass"},
		postgres,
		{Name: "authentication_authorisation", Status: "pass", Message: "local credentials, sessions, bootstrap, isolate, break-glass, OOB verification and deny-by-default authorisation are built"},
		{Name: "incident_semantic_kernel", Status: "pass", Message: "Record/Propose/Decide and semantic incident projections are built"},
		{Name: "actions_checklists_timeline", Status: "pass", Message: "governed actions, checklists and rebuildable timeline are built"},
		{Name: "obligations_timed_waits", Status: "pass", Message: "governed obligations, deterministic clocks and bounded rebuildable timed waits are built"},
		{Name: "access_governance_compartments", Status: "pass", Message: "Sponsor/Controller to Custodian governance and scoped incident/compartment access are built"},
		{Name: "evidence_immutable_blobs", Status: "pass", Message: "bounded immutable PostgreSQL blobs, evidence register and fail-closed presentation are built"},
		{Name: "corpus_deterministic_retrieval", Status: "pass", Message: "governed Markdown/DOCX ingestion and deterministic filter-before-rank retrieval are built"},
		{Name: "model_assistance", Status: "pass", Message: "optional three-lane model drafting is built; deterministic core does not depend on a model"},
		{Name: "communications_notifier", Status: "pass", Message: "structured communications, local sink and bounded webhook delivery are built"},
		{Name: "print_paper_resilience", Status: "pass", Message: "authorised local PDF composition, last-good copies and paper reconciliation are built"},
	}}
}

// FinaliseReadiness treats warnings as visible degraded readiness but fails closed
// for checks that are failed, pending or not configured.
func FinaliseReadiness(r Readiness) Readiness {
	r.Ready = true
	for _, check := range r.Checks {
		switch check.Status {
		case "pass", "warn", "implemented":
			// visible but non-blocking
		default:
			r.Ready = false
		}
	}
	return r
}
