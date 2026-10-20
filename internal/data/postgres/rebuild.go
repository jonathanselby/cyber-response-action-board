package postgres

// RebuildProjections returns the complete deterministic projection set for the
// authoritative ledger. Order matters during replay where one projection
// establishes state consumed by another: incident state precedes declaration
// access, and the incident timeline owner precedes timeline contributors.
func RebuildProjections() []Projection {
	return []Projection{
		AccessProjection{},
		IncidentProjection{},
		DeclarationAccessProjection{},
		ObligationProjection{},
		EvidenceProjection{},
		CorpusProjection{},
		CommunicationProjection{},
		RecoveryProjection{},
		PaperProjection{},
	}
}
