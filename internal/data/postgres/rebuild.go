package postgres

// RebuildProjections returns the complete deterministic projection set for the
// authoritative ledger. Order matters during replay where one projection
// establishes state consumed by another: incident state precedes declaration
// access and continuity (confirmed severity updates current incident state), and
// the incident timeline owner precedes timeline contributors.
func RebuildProjections() []Projection {
	return []Projection{
		AccessProjection{},
		IncidentProjection{},
		DeclarationAccessProjection{},
		ContinuityProjection{},
		ObligationProjection{},
		EvidenceProjection{},
		CorpusProjection{},
		CommunicationProjection{},
		RecoveryProjection{},
		PaperProjection{},
		OrganisationProjection{},
		RelationshipProjection{},
	}
}
