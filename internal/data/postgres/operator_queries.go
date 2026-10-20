package postgres

import (
	"context"
	"encoding/json"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

type PendingDeclarationView struct {
	IncidentID  string
	RecordID    string
	Version     string
	Maker       string
	Mode        string
	Declaration domain.IncidentDeclaration
}

func (s *IncidentStore) PendingDeclarations(ctx context.Context) ([]PendingDeclarationView, error) {
	rows, err := s.pool.Query(ctx, `SELECT incident_id,record_id,version,maker,mode,payload FROM crab.incident_proposals WHERE record_type=$1 ORDER BY proposed_at,incident_id`, domain.TypeIncidentDeclaration)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PendingDeclarationView
	for rows.Next() {
		var v PendingDeclarationView
		var raw []byte
		if err := rows.Scan(&v.IncidentID, &v.RecordID, &v.Version, &v.Maker, &v.Mode, &raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &v.Declaration); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
