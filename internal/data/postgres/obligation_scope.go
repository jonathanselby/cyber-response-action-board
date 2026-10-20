package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

// ClocksInCompartment returns GENERAL plus the explicitly requested compartment.
// It is the application read path; callers must still pass normal authorisation.
func (s *ObligationStore) ClocksInCompartment(ctx context.Context, incidentID, compartment string, now time.Time) ([]domain.ObligationClockView, error) {
	return s.clocksInCompartment(ctx, incidentID, compartment, now, false)
}

// ActiveClocksInCompartment is the common-operating-picture view. Historical
// waived clocks remain in the obligation view but do not appear as active COP work.
func (s *ObligationStore) ActiveClocksInCompartment(ctx context.Context, incidentID, compartment string, now time.Time) ([]domain.ObligationClockView, error) {
	return s.clocksInCompartment(ctx, incidentID, compartment, now, true)
}

func (s *ObligationStore) clocksInCompartment(ctx context.Context, incidentID, compartment string, now time.Time, activeOnly bool) ([]domain.ObligationClockView, error) {
	if strings.TrimSpace(incidentID) == "" || strings.TrimSpace(compartment) == "" {
		return nil, errors.New("incident and compartment are required")
	}
	query := `SELECT clock_id,definition_id,name,lifecycle,status,trigger_at,source_record_id,source_field,original_definition_id,original_deadline,deadline,recipient,owner_text,authority_source,last_change_kind,last_change_reason,time_status,simulated FROM crab.obligation_clocks WHERE incident_id=$1 AND (compartment='GENERAL' OR compartment=$2)`
	if activeOnly {
		query += ` AND status='active'`
	}
	query += ` ORDER BY deadline,clock_id`
	rows, err := s.pool.Query(ctx, query, incidentID, compartment)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.ObligationClockView
	for rows.Next() {
		var value domain.ObligationClockView
		value.IncidentID = incidentID
		if err := rows.Scan(&value.ClockID, &value.DefinitionID, &value.Name, &value.Lifecycle, &value.Status, &value.TriggerAt, &value.SourceRecordID, &value.SourceField, &value.OriginalDefinitionID, &value.OriginalDeadline, &value.Deadline, &value.Recipient, &value.Owner, &value.AuthoritySource, &value.LastChangeKind, &value.LastChangeReason, &value.TimeStatus, &value.Simulated); err != nil {
			return nil, err
		}
		value.Overdue = domain.ObligationOverdue(value.Status, value.Deadline, now)
		result = append(result, value)
	}
	return result, rows.Err()
}
