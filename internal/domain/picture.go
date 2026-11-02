package domain

import "time"

type IncidentState struct {
	IncidentID string
	Mode       string
	Title      string
	Summary    string
	Severity   string
	DeclaredAt time.Time
}

type FactView struct {
	RecordID    string
	Compartment string
	Fact        Fact
	Maker       string
	RecordedAt  time.Time
}

type UncertaintyView struct {
	RecordID    string
	Compartment string
	Uncertainty Uncertainty
	Maker       string
	RecordedAt  time.Time
}

type DecisionView struct {
	RecordID    string
	Compartment string
	Decision    Decision
	Maker       string
	Confirmer   string
	ConfirmedAt time.Time
}

type ActionView struct {
	RecordID    string
	Compartment string
	Action      Action
	Maker       string
	Confirmer   string
	ConfirmedAt time.Time
	Overdue     bool
}

type OperationalRoleView struct {
	RecordID    string
	Compartment string
	Assignment  OperationalRoleAssignment
	Maker       string
	Confirmer   string
	ConfirmedAt time.Time
	// External is set by the application tier when the role holder is an
	// external person, so every view and print can say so in words. It is not
	// stored.
	External bool `json:",omitempty"`
}

type PendingProposal struct {
	RecordID    string
	RecordType  string
	Compartment string
	Version     string
	Maker       string
	ProposedAt  time.Time
}

type OperatingPicture struct {
	Incident         IncidentState
	Facts            []FactView
	Unknowns         []UncertaintyView
	Hypotheses       []UncertaintyView
	Decisions        []DecisionView
	Actions          []ActionView
	Obligations      []ObligationClockView
	OperationalRoles []OperationalRoleView
	Pending          []PendingProposal
	Timeline         []TimelineEntry
}
