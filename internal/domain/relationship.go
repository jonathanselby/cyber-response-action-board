package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

// TypeIncidentRelationship asserts that two incidents are related (PRD 23,
// AC-84, EDD 13.14). It merges nothing and grants nothing.
const TypeIncidentRelationship = "incident.relationship"

type IncidentRelationship struct {
	RelatedIncidentID string `json:"related_incident_id"`
	Reason            string `json:"reason"`
	Active            bool   `json:"active"`
}

func ValidateIncidentRelationship(incidentID string, v IncidentRelationship) error {
	related := strings.TrimSpace(v.RelatedIncidentID)
	if related == "" || related == strings.TrimSpace(incidentID) {
		return errors.New("choose a different incident to relate")
	}
	if strings.TrimSpace(v.Reason) == "" || len(v.Reason) > 1000 {
		return errors.New("a relationship needs a reason of at most 1000 characters")
	}
	return nil
}

// ProposeIncidentRelationship records the assertion under incident A in
// GENERAL; restricted compartments are never linked.
func ProposeIncidentRelationship(scope Scope, recordID, maker string, now time.Time, v IncidentRelationship) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if scope.Compartment != "GENERAL" {
		return ledger.Record{}, errors.New("incident relationships are recorded in GENERAL")
	}
	if err := ValidateIncidentRelationship(scope.IncidentID, v); err != nil {
		return ledger.Record{}, err
	}
	return proposedRecord(scope, TypeIncidentRelationship, recordID, maker, now, v)
}

// RelationshipView is one confirmed link as seen from one incident.
type RelationshipView struct {
	IncidentID        string
	RelatedIncidentID string
	Reason            string
	RecordID          string
	Maker             string
	Confirmer         string
	ConfirmedAt       time.Time
}
