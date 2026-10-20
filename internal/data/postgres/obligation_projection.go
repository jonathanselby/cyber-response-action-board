package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

type ObligationProjection struct{}

func (ObligationProjection) Reset(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `DELETE FROM crab.incident_timeline WHERE record_type IN ($1,$2)`, domain.TypeObligationTrigger, domain.TypeObligationChange); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `TRUNCATE crab.timed_wait_list, crab.obligation_clocks, crab.obligation_proposals, crab.obligation_definitions`)
	return err
}

func (ObligationProjection) Apply(ctx context.Context, tx pgx.Tx, entry ledger.Entry) error {
	record := entry.Record
	if !obligationType(record.RecordType) {
		return nil
	}
	if record.WriteSemantic != "governed" {
		return errors.New("obligation records require governed semantics")
	}
	if err := validateObligationEnvelope(record); err != nil {
		return err
	}
	data, err := domain.DecodeGovernedData(record.Data)
	if err != nil {
		return err
	}
	scopeID := record.IncidentID
	if record.IncidentMode == "system" {
		scopeID = ""
	}
	if record.Status == "proposed" {
		if err := validateObligationPayload(record); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO crab.obligation_proposals(scope_id,record_id,record_type,compartment,version,maker,payload,proposed_at,mode) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, scopeID, record.RecordID, record.RecordType, record.Compartment, data.Version, record.Maker, []byte(data.Payload), record.Timestamp, record.IncidentMode); err != nil {
			return err
		}
		if record.RecordType == domain.TypeObligationTrigger && record.IncidentMode == "live" {
			if err := upsertTriggerClock(ctx, tx, record, domain.ObligationLifecycleProvisional); err != nil {
				return err
			}
		}
		if record.RecordType == domain.TypeObligationTrigger && record.IncidentMode == "exercise" {
			if err := upsertTriggerClock(ctx, tx, record, domain.ObligationLifecycleProvisional); err != nil {
				return err
			}
		}
		return applyObligationTimeline(ctx, tx, entry)
	}
	if record.Status != "confirmed" && record.Status != "rejected" {
		return errors.New("invalid obligation governance status")
	}
	var version, maker, compartment, mode string
	var payload []byte
	if err := tx.QueryRow(ctx, `SELECT version,maker,compartment,mode,payload FROM crab.obligation_proposals WHERE scope_id=$1 AND record_type=$2 AND record_id=$3`, scopeID, record.RecordType, record.RecordID).Scan(&version, &maker, &compartment, &mode, &payload); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("obligation decision has no matching pending proposal")
		}
		return err
	}
	if version != data.Version || maker != record.Maker || compartment != record.Compartment || mode != record.IncidentMode || !bytes.Equal(payload, []byte(data.Payload)) {
		return errors.New("obligation decision does not match exact pending proposal")
	}
	if record.Status == "confirmed" {
		switch record.RecordType {
		case domain.TypeObligationDefinition:
			if err := applyConfirmedDefinition(ctx, tx, record); err != nil {
				return err
			}
		case domain.TypeObligationTrigger:
			if err := upsertTriggerClock(ctx, tx, record, domain.ObligationLifecycleFirm); err != nil {
				return err
			}
		case domain.TypeObligationChange:
			if err := applyConfirmedObligationChange(ctx, tx, record); err != nil {
				return err
			}
		}
	} else if record.RecordType == domain.TypeObligationTrigger {
		if _, err := tx.Exec(ctx, `DELETE FROM crab.timed_wait_list WHERE incident_id=$1 AND clock_id=$2`, record.IncidentID, record.RecordID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM crab.obligation_clocks WHERE incident_id=$1 AND clock_id=$2 AND lifecycle='provisional'`, record.IncidentID, record.RecordID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM crab.obligation_proposals WHERE scope_id=$1 AND record_type=$2 AND record_id=$3`, scopeID, record.RecordType, record.RecordID); err != nil {
		return err
	}
	return applyObligationTimeline(ctx, tx, entry)
}

func validateObligationEnvelope(record ledger.Record) error {
	switch record.RecordType {
	case domain.TypeObligationDefinition:
		if record.IncidentMode != "system" || record.IncidentID != "" || record.Compartment != "SYSTEM" {
			return errors.New("obligation definition requires system scope")
		}
	case domain.TypeObligationTrigger, domain.TypeObligationChange:
		if record.IncidentID == "" || (record.IncidentMode != "live" && record.IncidentMode != "exercise") {
			return errors.New("incident obligation record requires live/exercise scope")
		}
	}
	return nil
}

func validateObligationPayload(record ledger.Record) error {
	switch record.RecordType {
	case domain.TypeObligationDefinition:
		value, err := domain.DecodePayload[domain.ObligationDefinition](record)
		if err != nil {
			return err
		}
		return domain.ValidateObligationDefinition(value)
	case domain.TypeObligationTrigger:
		value, err := domain.DecodePayload[domain.ObligationTrigger](record)
		if err != nil {
			return err
		}
		return domain.ValidateObligationTrigger(value)
	case domain.TypeObligationChange:
		value, err := domain.DecodePayload[domain.ObligationChange](record)
		if err != nil {
			return err
		}
		return domain.ValidateObligationChange(value)
	}
	return errors.New("unsupported obligation type")
}

func applyConfirmedDefinition(ctx context.Context, tx pgx.Tx, record ledger.Record) error {
	def, err := domain.DecodePayload[domain.ObligationDefinition](record)
	if err != nil {
		return err
	}
	calendar, _ := json.Marshal(def.Calendar)
	warnings, _ := json.Marshal(def.WarningBeforeMinutes)
	_, err = tx.Exec(ctx, `INSERT INTO crab.obligation_definitions(definition_id,name,trigger_field,duration,unit,calendar,applicability_condition,recipient,owner_text,authority_source,warning_before_minutes,maker,confirmer,confirmed_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) ON CONFLICT(definition_id) DO UPDATE SET name=EXCLUDED.name,trigger_field=EXCLUDED.trigger_field,duration=EXCLUDED.duration,unit=EXCLUDED.unit,calendar=EXCLUDED.calendar,applicability_condition=EXCLUDED.applicability_condition,recipient=EXCLUDED.recipient,owner_text=EXCLUDED.owner_text,authority_source=EXCLUDED.authority_source,warning_before_minutes=EXCLUDED.warning_before_minutes,maker=EXCLUDED.maker,confirmer=EXCLUDED.confirmer,confirmed_at=EXCLUDED.confirmed_at`, record.RecordID, def.Name, def.TriggerField, def.Duration, def.Unit, calendar, def.ApplicabilityCondition, def.Recipient, def.Owner, def.AuthoritySource, warnings, record.Maker, record.Confirmer, record.Timestamp)
	return err
}

func loadObligationDefinition(ctx context.Context, tx pgx.Tx, id string) (domain.ObligationDefinition, error) {
	var def domain.ObligationDefinition
	var calendar, warnings []byte
	if err := tx.QueryRow(ctx, `SELECT name,trigger_field,duration,unit,calendar,applicability_condition,recipient,owner_text,authority_source,warning_before_minutes FROM crab.obligation_definitions WHERE definition_id=$1`, id).Scan(&def.Name, &def.TriggerField, &def.Duration, &def.Unit, &calendar, &def.ApplicabilityCondition, &def.Recipient, &def.Owner, &def.AuthoritySource, &warnings); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return def, errors.New("obligation definition is not confirmed")
		}
		return def, err
	}
	if err := json.Unmarshal(calendar, &def.Calendar); err != nil {
		return def, err
	}
	if err := json.Unmarshal(warnings, &def.WarningBeforeMinutes); err != nil {
		return def, err
	}
	return def, domain.ValidateObligationDefinition(def)
}

func upsertTriggerClock(ctx context.Context, tx pgx.Tx, record ledger.Record, lifecycle string) error {
	trigger, err := domain.DecodePayload[domain.ObligationTrigger](record)
	if err != nil {
		return err
	}
	def, err := loadObligationDefinition(ctx, tx, trigger.DefinitionID)
	if err != nil {
		return err
	}
	if trigger.SourceField != def.TriggerField {
		return errors.New("obligation trigger field does not match confirmed definition")
	}
	calculated, err := domain.CalculateObligationDeadline(def, trigger.TriggerAt)
	if err != nil {
		return err
	}
	simulated := record.IncidentMode == "exercise"
	confirmer := ""
	var confirmedAt any
	if lifecycle == domain.ObligationLifecycleFirm {
		confirmer = record.Confirmer
		confirmedAt = record.Timestamp
	}
	_, err = tx.Exec(ctx, `INSERT INTO crab.obligation_clocks(incident_id,clock_id,compartment,definition_id,name,lifecycle,status,trigger_at,source_record_id,source_field,original_definition_id,original_deadline,deadline,recipient,owner_text,authority_source,time_status,simulated,maker,confirmer,confirmed_at) VALUES($1,$2,$3,$4,$5,$6,'active',$7,$8,$9,$4,$10,$10,$11,$12,$13,$14,$15,$16,$17,$18) ON CONFLICT(incident_id,clock_id) DO UPDATE SET lifecycle=EXCLUDED.lifecycle,status='active',deadline=EXCLUDED.deadline,recipient=EXCLUDED.recipient,owner_text=EXCLUDED.owner_text,authority_source=EXCLUDED.authority_source,time_status=EXCLUDED.time_status,simulated=EXCLUDED.simulated,confirmer=EXCLUDED.confirmer,confirmed_at=EXCLUDED.confirmed_at`, record.IncidentID, record.RecordID, record.Compartment, trigger.DefinitionID, def.Name, lifecycle, trigger.TriggerAt, trigger.SourceRecordID, trigger.SourceField, calculated.Deadline, def.Recipient, def.Owner, def.AuthoritySource, record.TimeStatus, simulated, record.Maker, confirmer, confirmedAt)
	if err != nil {
		return err
	}
	return rebuildClockWaits(ctx, tx, record.IncidentID, record.RecordID, lifecycle, simulated, calculated.Deadline, def.WarningBeforeMinutes, domain.ObligationStatusActive)
}

func applyConfirmedObligationChange(ctx context.Context, tx pgx.Tx, record ledger.Record) error {
	change, err := domain.DecodePayload[domain.ObligationChange](record)
	if err != nil {
		return err
	}
	var triggerAt time.Time
	var originalDefinitionID string
	var simulated bool
	if err := tx.QueryRow(ctx, `SELECT trigger_at,original_definition_id,simulated FROM crab.obligation_clocks WHERE incident_id=$1 AND clock_id=$2`, record.IncidentID, change.ClockID).Scan(&triggerAt, &originalDefinitionID, &simulated); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("obligation change references unknown clock")
		}
		return err
	}
	status := domain.ObligationStatusActive
	var definitionID, name, recipient, owner, authority string
	var deadline time.Time
	var warnings []int
	if err := tx.QueryRow(ctx, `SELECT definition_id,name,recipient,owner_text,authority_source,deadline FROM crab.obligation_clocks WHERE incident_id=$1 AND clock_id=$2`, record.IncidentID, change.ClockID).Scan(&definitionID, &name, &recipient, &owner, &authority, &deadline); err != nil {
		return err
	}
	switch change.Kind {
	case "amend", "extend":
		deadline = *change.NewDeadline
	case "waive":
		status = domain.ObligationStatusWaived
	case "replace":
		def, err := loadObligationDefinition(ctx, tx, change.ReplacementDefinitionID)
		if err != nil {
			return err
		}
		calculated, err := domain.CalculateObligationDeadline(def, triggerAt)
		if err != nil {
			return err
		}
		definitionID, name, recipient, owner, authority, deadline, warnings = change.ReplacementDefinitionID, def.Name, def.Recipient, def.Owner, def.AuthoritySource, calculated.Deadline, def.WarningBeforeMinutes
	}
	if warnings == nil {
		def, err := loadObligationDefinition(ctx, tx, definitionID)
		if err != nil {
			return err
		}
		warnings = def.WarningBeforeMinutes
	}
	_, err = tx.Exec(ctx, `UPDATE crab.obligation_clocks SET definition_id=$3,name=$4,status=$5,deadline=$6,recipient=$7,owner_text=$8,authority_source=$9,last_change_kind=$10,last_change_reason=$11,confirmer=$12,confirmed_at=$13 WHERE incident_id=$1 AND clock_id=$2`, record.IncidentID, change.ClockID, definitionID, name, status, deadline, recipient, owner, authority, change.Kind, change.Reason, record.Confirmer, record.Timestamp)
	if err != nil {
		return err
	}
	_ = originalDefinitionID
	return rebuildClockWaits(ctx, tx, record.IncidentID, change.ClockID, domain.ObligationLifecycleFirm, simulated, deadline, warnings, status)
}

func rebuildClockWaits(ctx context.Context, tx pgx.Tx, incidentID, clockID, lifecycle string, simulated bool, deadline time.Time, warnings []int, status string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM crab.timed_wait_list WHERE incident_id=$1 AND clock_id=$2`, incidentID, clockID); err != nil {
		return err
	}
	if status != domain.ObligationStatusActive {
		return nil
	}
	for _, minutes := range warnings {
		wake := deadline.Add(-time.Duration(minutes) * time.Minute)
		if _, err := tx.Exec(ctx, `INSERT INTO crab.timed_wait_list(incident_id,clock_id,wake_at,wake_kind,lifecycle,simulated) VALUES($1,$2,$3,'obligation_warning',$4,$5) ON CONFLICT DO NOTHING`, incidentID, clockID, wake, lifecycle, simulated); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `INSERT INTO crab.timed_wait_list(incident_id,clock_id,wake_at,wake_kind,lifecycle,simulated) VALUES($1,$2,$3,'obligation_deadline',$4,$5) ON CONFLICT DO NOTHING`, incidentID, clockID, deadline, lifecycle, simulated)
	return err
}

func applyObligationTimeline(ctx context.Context, tx pgx.Tx, entry ledger.Entry) error {
	record := entry.Record
	if record.IncidentID == "" {
		return nil
	}
	summary := ""
	switch record.RecordType {
	case domain.TypeObligationTrigger:
		value, err := domain.DecodePayload[domain.ObligationTrigger](record)
		if err != nil {
			return err
		}
		summary = fmt.Sprintf("Obligation clock %s from %s", record.RecordID, value.SourceRecordID)
	case domain.TypeObligationChange:
		value, err := domain.DecodePayload[domain.ObligationChange](record)
		if err != nil {
			return err
		}
		summary = fmt.Sprintf("Obligation %s: %s", value.ClockID, value.Kind)
	default:
		return nil
	}
	_, err := tx.Exec(ctx, `INSERT INTO crab.incident_timeline(sequence,incident_id,record_id,record_type,governance,compartment,summary,recorded_at,awareness,time_status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'[]'::jsonb,$9)`, entry.Seq, record.IncidentID, record.RecordID, record.RecordType, record.Status, record.Compartment, summary, record.Timestamp, record.TimeStatus)
	return err
}

func obligationType(recordType string) bool {
	return recordType == domain.TypeObligationDefinition || recordType == domain.TypeObligationTrigger || recordType == domain.TypeObligationChange
}

type ObligationStore struct{ pool *pgxpool.Pool }

func NewObligationStore(pool *pgxpool.Pool) (*ObligationStore, error) {
	if pool == nil {
		return nil, errors.New("PostgreSQL pool is required")
	}
	return &ObligationStore{pool: pool}, nil
}

func (s *ObligationStore) Clocks(ctx context.Context, incidentID string, now time.Time) ([]domain.ObligationClockView, error) {
	rows, err := s.pool.Query(ctx, `SELECT clock_id,definition_id,name,lifecycle,status,trigger_at,source_record_id,source_field,original_definition_id,original_deadline,deadline,recipient,owner_text,authority_source,last_change_kind,last_change_reason,time_status,simulated FROM crab.obligation_clocks WHERE incident_id=$1 ORDER BY deadline,clock_id`, incidentID)
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

func (s *ObligationStore) DueWakeups(ctx context.Context, now time.Time, limit int) ([]TimedWakeup, error) {
	if limit <= 0 || limit > 1000 {
		return nil, errors.New("timed wait-list limit must be between 1 and 1000")
	}
	rows, err := s.pool.Query(ctx, `SELECT incident_id,clock_id,wake_at,wake_kind,lifecycle,simulated FROM crab.timed_wait_list WHERE wake_at <= $1 ORDER BY wake_at,incident_id,clock_id LIMIT $2`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []TimedWakeup
	for rows.Next() {
		var value TimedWakeup
		if err := rows.Scan(&value.IncidentID, &value.ClockID, &value.WakeAt, &value.Kind, &value.Lifecycle, &value.Simulated); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

type TimedWakeup struct {
	IncidentID string
	ClockID    string
	WakeAt     time.Time
	Kind       string
	Lifecycle  string
	Simulated  bool
}
