package postgres

import (
	"context"
	"encoding/json"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

type PendingDeclaration struct {
	IncidentID  string
	RecordID    string
	Version     string
	Maker       string
	Mode        string
	Declaration domain.IncidentDeclaration
}

func (s *IncidentStore) PendingDeclarations(ctx context.Context) ([]PendingDeclaration, error) {
	rows, err := s.pool.Query(ctx, `SELECT incident_id,record_id,version,maker,mode,payload FROM crab.incident_proposals WHERE record_type=$1 ORDER BY proposed_at,incident_id,record_id`, domain.TypeIncidentDeclaration)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PendingDeclaration
	for rows.Next() {
		var v PendingDeclaration
		var payload []byte
		if err := rows.Scan(&v.IncidentID, &v.RecordID, &v.Version, &v.Maker, &v.Mode, &payload); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(payload, &v.Declaration); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
