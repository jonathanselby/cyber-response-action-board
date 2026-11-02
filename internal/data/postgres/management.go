package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

// DecisionLoopProjection records when each incident-scoped governed record
// was proposed and decided. The kernel applies it to every append, so no
// runtime can forget it.
type DecisionLoopProjection struct{}

func (DecisionLoopProjection) Reset(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `TRUNCATE crab.decision_loops`)
	return err
}

func (DecisionLoopProjection) Apply(ctx context.Context, tx pgx.Tx, entry ledger.Entry) error {
	r := entry.Record
	if r.WriteSemantic != "governed" || r.IncidentID == "" {
		return nil
	}
	switch r.Status {
	case "proposed":
		_, err := tx.Exec(ctx, `INSERT INTO crab.decision_loops(record_type,record_id,incident_id,mode,compartment,proposed_at) VALUES($1,$2,$3,$4,$5,$6)
			ON CONFLICT (record_type,record_id) DO UPDATE SET proposed_at=EXCLUDED.proposed_at, decided_at=NULL, outcome=NULL`,
			r.RecordType, r.RecordID, r.IncidentID, r.IncidentMode, r.Compartment, r.Timestamp)
		return err
	case "confirmed", "rejected":
		_, err := tx.Exec(ctx, `UPDATE crab.decision_loops SET decided_at=$3, outcome=$4 WHERE record_type=$1 AND record_id=$2`, r.RecordType, r.RecordID, r.Timestamp, r.Status)
		return err
	}
	return nil
}

// ManagementStore computes the management summary (addendum A3) with a fixed
// set of four aggregate queries, whatever the number of incidents. Scope is
// the (incident, compartment) pairs the viewer may read; the store never
// widens it.
type ManagementStore struct{ pool *pgxpool.Pool }

func NewManagementStore(pool *pgxpool.Pool) (*ManagementStore, error) {
	if pool == nil {
		return nil, errors.New("PostgreSQL pool is required")
	}
	return &ManagementStore{pool: pool}, nil
}

// ManagementQueries is the number of queries ManagementSummary issues.
const ManagementQueries = 4

func (s *ManagementStore) ManagementSummary(ctx context.Context, incidents, compartments []string, now time.Time) (domain.ManagementSummary, error) {
	var out domain.ManagementSummary
	if len(incidents) != len(compartments) {
		return out, errors.New("scope pairs are unbalanced")
	}
	// Live, not closed, with GENERAL in scope.
	const active = `SELECT i.incident_id, i.severity FROM crab.incidents i
		LEFT JOIN crab.incident_recovery_state r ON r.incident_id=i.incident_id
		JOIN unnest($1::text[], $2::text[]) AS p(incident_id, compartment) ON p.incident_id=i.incident_id AND p.compartment='GENERAL'
		WHERE i.mode='live' AND r.closed_at IS NULL`
	rows, err := s.pool.Query(ctx, `SELECT severity, count(*) FROM (`+active+`) a GROUP BY severity ORDER BY severity`, incidents, compartments)
	if err != nil {
		return out, fmt.Errorf("management: active incidents: %w", err)
	}
	for rows.Next() {
		var c domain.SeverityCount
		if err := rows.Scan(&c.Severity, &c.Count); err != nil {
			rows.Close()
			return out, err
		}
		out.ActiveIncidents += c.Count
		out.BySeverity = append(out.BySeverity, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	critical := make([]string, len(domain.CriticalActionPriorities))
	for i, p := range domain.CriticalActionPriorities {
		critical[i] = strings.ToUpper(p)
	}
	// Overdue critical actions in active incidents, in readable compartments
	// (domain.ActionOverdue).
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM crab.incident_actions a
		JOIN unnest($1::text[], $2::text[]) AS p(incident_id, compartment) ON p.incident_id=a.incident_id AND p.compartment=a.compartment
		WHERE a.incident_id IN (SELECT incident_id FROM (`+active+`) x)
		  AND a.due_at IS NOT NULL AND a.due_at < $3 AND a.status NOT IN ('complete','skipped') AND upper(btrim(a.priority)) = ANY($4)`,
		incidents, compartments, now, critical).Scan(&out.OverdueCriticalAction); err != nil {
		return out, fmt.Errorf("management: overdue actions: %w", err)
	}
	// Overdue obligations in active incidents (domain.ObligationOverdue).
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM crab.obligation_clocks c
		JOIN unnest($1::text[], $2::text[]) AS p(incident_id, compartment) ON p.incident_id=c.incident_id AND p.compartment=c.compartment
		WHERE c.incident_id IN (SELECT incident_id FROM (`+active+`) x)
		  AND c.status='active' AND c.deadline < $3`, incidents, compartments, now).Scan(&out.OverdueObligations); err != nil {
		return out, fmt.Errorf("management: overdue obligations: %w", err)
	}
	// Median decision-loop time: confirmed governed records in live incidents
	// in scope, decided in the last seven days.
	var median *float64
	if err := s.pool.QueryRow(ctx, `SELECT count(*), percentile_cont(0.5) WITHIN GROUP (ORDER BY extract(epoch FROM d.decided_at - d.proposed_at))
		FROM crab.decision_loops d
		JOIN unnest($1::text[], $2::text[]) AS p(incident_id, compartment) ON p.incident_id=d.incident_id AND p.compartment=d.compartment
		WHERE d.mode='live' AND d.outcome='confirmed' AND d.decided_at > $3 AND d.decided_at <= $4`,
		incidents, compartments, now.Add(-domain.DecisionLoopWindow), now).Scan(&out.DecisionsInWindow, &median); err != nil {
		return out, fmt.Errorf("management: decision loop: %w", err)
	}
	if median != nil {
		d := time.Duration(*median * float64(time.Second)).Round(time.Second)
		out.MedianDecisionLoop = &d
	}
	return out, nil
}
