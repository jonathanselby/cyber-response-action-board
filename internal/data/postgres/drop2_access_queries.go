package postgres

import (
	"context"
	"time"
)

type PendingAccessView struct {
	ScopeID     string
	RecordID    string
	RecordType  string
	Compartment string
	Version     string
	Maker       string
	Mode        string
	ProposedAt  time.Time
}

func (s *AccessStore) PendingAccessAdministration(ctx context.Context) ([]PendingAccessView, error) {
	rows, err := s.pool.Query(ctx, `SELECT scope_id,record_id,record_type,compartment,version,maker,mode,proposed_at FROM crab.access_proposals ORDER BY proposed_at,scope_id,record_type,record_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []PendingAccessView
	for rows.Next() {
		var value PendingAccessView
		if err := rows.Scan(&value.ScopeID, &value.RecordID, &value.RecordType, &value.Compartment, &value.Version, &value.Maker, &value.Mode, &value.ProposedAt); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (s *AccessStore) PendingAccessForIncident(ctx context.Context, incidentID string) ([]PendingAccessView, error) {
	rows, err := s.pool.Query(ctx, `SELECT scope_id,record_id,record_type,compartment,version,maker,mode,proposed_at FROM crab.access_proposals WHERE scope_id=$1 ORDER BY proposed_at,record_type,record_id`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []PendingAccessView
	for rows.Next() {
		var value PendingAccessView
		if err := rows.Scan(&value.ScopeID, &value.RecordID, &value.RecordType, &value.Compartment, &value.Version, &value.Maker, &value.Mode, &value.ProposedAt); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}
