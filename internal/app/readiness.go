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
// state. Live deployment checks are enriched by the composition root.
func CurrentReadiness(cfg config.Config) Readiness {
	postgres := Check{Name: "postgresql", Status: "not-configured", Message: "CRAB_DATABASE_URL and CRAB_LEDGER_KEYRING_FILE are required for durable-store verification"}
	if cfg.DatabaseURL != "" && cfg.LedgerKeyringFile != "" {
		postgres = Check{Name: "postgresql", Status: "pending", Message: "runtime verification pending"}
	}
	keyCustody := Check{Name: "key_custody", Status: "not-configured", Message: "local key files are not fully configured"}
	if cfg.LedgerKeyringFile != "" && cfg.SessionKeyFile != "" && cfg.ServerSigningKeyFile != "" && cfg.CredentialWrapKeyFile != "" {
		keyCustody = Check{Name: "key_custody", Status: "pending", Message: "configured key files require runtime readability checks"}
	}
	backupRestore := Check{Name: "backup_restore", Status: "implemented", Message: "Slice 13 PostgreSQL-native backup, SHA-256 metadata, verified replacement-host restore and deterministic rebuild are built; live rehearsal state is deployment evidence"}
	checks := []Check{
		{Name: "deployment_config", Status: "pass"},
		{Name: "portable_ledger_verifier", Status: "pass"},
		postgres,
		keyCustody,
		{Name: "authentication_authorisation", Status: "implemented", Message: "Slice 2 security kernel, projections and ceremonies are built; live installation state still requires secure setup wiring"},
		{Name: "incident_semantic_kernel", Status: "implemented", Message: "Slice 3 Record/Propose/Decide application kernel and rebuildable common-operating-picture projections are built"},
		{Name: "actions_checklists_timeline", Status: "implemented", Message: "Slice 4 governed Action lifecycle, configured checklist activation, explicit incident timeline and dedicated Action/timeline renderers are built"},
		{Name: "obligations_timed_waits", Status: "implemented", Message: "Slice 5 governed organisation-defined obligations, deterministic provisional/firm clocks, amendments and bounded rebuildable timed wait list are built"},
		{Name: "access_governance_compartments", Status: "implemented", Message: "Slice 6 Sponsor/Controller -> Custodian governance, expiring incident/compartment access, metadata-only administration and selective-release scope are built"},
		{Name: "evidence_immutable_blobs", Status: "implemented", Message: "Slice 7 bounded PostgreSQL hostile-byte storage, SHA-256 integrity verification, opaque per-upload IDs and scoped evidence register/history are built"},
		{Name: "corpus_deterministic_retrieval", Status: "implemented", Message: "Slice 8 governed Markdown/DOCX corpus ingestion, immutable original blobs, scope-first deterministic lexical retrieval and stable citations are built"},
		{Name: "model_assistance", Status: "implemented", Message: "Slice 9 static prompt-versioned draft specs, explicit three-lane routing, sensitivity/model-send gates, provenance traces and bounded shedding are built"},
		{Name: "communications_notifier", Status: "implemented", Message: "Slice 10 semantic communications, deterministic situation-report cadence, notifier adapters, immutable outcomes and exercise suppression-before-I/O are built"},
		{Name: "print_paper_resilience", Status: "implemented", Message: "Slice 11 authorised print composition, deterministic A4/Letter PDF rendering, atomic last-good copies and paper-event reconciliation are built"},
		{Name: "recovery_pir_retention_closure", Status: "implemented", Message: "Slice 12 recovery coordination, derived phase, maker-checker stand-down/PIR/retention/closure, compartment-safe closure and closed read-only enforcement are built"},
		backupRestore,
	}
	return FinaliseReadiness(Readiness{Checks: checks})
}

func FinaliseReadiness(r Readiness) Readiness {
	r.Ready = true
	for _, check := range r.Checks {
		switch check.Status {
		case "fail", "not-configured", "pending":
			r.Ready = false
		}
	}
	return r
}
