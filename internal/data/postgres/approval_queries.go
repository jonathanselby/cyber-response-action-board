package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// proposalTables is the closed map from an approval kind to the rebuildable
// pending-proposal table that already holds its payload. It adds no state: it
// lets the composition root show the proposed version a confirmer is deciding.
var proposalTables = map[string]struct{ table, scope string }{
	"declaration":   {"crab.incident_proposals", "incident_id"},
	"incident":      {"crab.incident_proposals", "incident_id"},
	"severity":      {"crab.continuity_proposals", "incident_id"},
	"obligation":    {"crab.obligation_proposals", "scope_id"},
	"communication": {"crab.communication_proposals", "incident_id"},
	"access":        {"crab.access_proposals", "scope_id"},
	"recovery":      {"crab.incident_recovery_proposals", "incident_id"},
	"organisation":  {"crab.organisation_proposals", "''::text"},
	"relationship":  {"crab.incident_relationship_proposals", "incident_id"},
}

// ProposalPayload returns the stored payload of one pending proposal. Callers
// must only ask for proposals already returned through an authorised listing.
func (k *Kernel) ProposalPayload(ctx context.Context, kind, scopeID, recordType, recordID string) ([]byte, bool, error) {
	spec, ok := proposalTables[kind]
	if !ok {
		return nil, false, errors.New("unknown proposal kind")
	}
	var payload []byte
	err := k.pool.QueryRow(ctx, `SELECT payload FROM `+spec.table+` WHERE `+spec.scope+`=$1 AND record_type=$2 AND record_id=$3`, scopeID, recordType, recordID).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return payload, true, nil
}

type PendingRecoveryView struct {
	IncidentID  string
	RecordID    string
	RecordType  string
	Compartment string
	Version     string
	Maker       string
	ProposedAt  time.Time
}

// PendingRecoveryProposals lists pending stand-down, PIR, retention and
// closure proposals for one incident in GENERAL plus one optional compartment.
func (k *Kernel) PendingRecoveryProposals(ctx context.Context, incidentID, compartment string) ([]PendingRecoveryView, error) {
	if compartment == "" {
		compartment = "GENERAL"
	}
	rows, err := k.pool.Query(ctx, `SELECT incident_id,record_id,record_type,compartment,version,maker,proposed_at FROM crab.incident_recovery_proposals WHERE incident_id=$1 AND compartment IN ('GENERAL',$2) ORDER BY proposed_at,record_type,record_id`, incidentID, compartment)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PendingRecoveryView
	for rows.Next() {
		var v PendingRecoveryView
		if err := rows.Scan(&v.IncidentID, &v.RecordID, &v.RecordType, &v.Compartment, &v.Version, &v.Maker, &v.ProposedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
