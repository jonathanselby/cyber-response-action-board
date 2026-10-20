package postgres

import (
	"context"
	"fmt"
	"time"
)

// ActiveCustodianSessionCount reports normal Custodian sessions that are
// demonstrably usable now. Recovery must not be used while two such sessions
// already provide an ordinary governance quorum.
func (s *AccessStore) ActiveCustodianSessionCount(ctx context.Context, now time.Time) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT count(DISTINCT s.human_id)
		FROM crab.sessions s
		JOIN crab.custodian_designations c ON c.human_id = s.human_id
		WHERE c.active AND s.expires_at > $1`, now.UTC()).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active Custodian sessions: %w", err)
	}
	return count, nil
}
