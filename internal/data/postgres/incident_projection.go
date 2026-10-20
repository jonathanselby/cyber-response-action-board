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

type IncidentProjection struct{}

func (IncidentProjection) Reset(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `TRUNCATE crab.incident_timeline, crab.incident_proposals, crab.incident_operational_roles, crab.incident_actions, crab.incident_decisions, crab.incident_uncertainties, crab.incident_facts, crab.incidents`)
	return err
}

func (IncidentProjection) Apply(ctx context.Context, tx pgx.Tx, entry ledger.Entry) error {
	record := entry.Record
	if !slice3Type(record.RecordType) {
		return nil
	}
	if record.IncidentID == "" || (record.IncidentMode != "live" && record.IncidentMode != "exercise") {
		return errors.New("incident projection requires live/exercise incident scope")
	}

	if record.WriteSemantic == "record" {
		switch record.RecordType {
		case domain.TypeFact:
			fact, err := domain.DecodePayload[domain.Fact](record)
			if err != nil {
				return err
			}
			if err := domain.ValidateFactTiming(fact); err != nil {
				return err
			}
			awareness, err := json.Marshal(fact.Awareness)
			if err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO crab.incident_facts(incident_id,record_id,compartment,statement,source,maker,recorded_at,occurred_at,flagged_at,awareness) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, record.IncidentID, record.RecordID, record.Compartment, fact.Statement, fact.Source, record.Maker, record.Timestamp, fact.OccurredAt, fact.FlaggedAt, awareness); err != nil {
				return err
			}
		case domain.TypeUncertainty:
			item, err := domain.DecodePayload[domain.Uncertainty](record)
			if err != nil {
				return err
			}
			if item.Kind != "unknown" && item.Kind != "hypothesis" {
				return errors.New("invalid uncertainty kind")
			}
			if _, err = tx.Exec(ctx, `INSERT INTO crab.incident_uncertainties(incident_id,record_id,compartment,kind,statement,maker,recorded_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, record.IncidentID, record.RecordID, record.Compartment, item.Kind, item.Statement, record.Maker, record.Timestamp); err != nil {
				return err
			}
		default:
			return errors.New("controlled incident state cannot use immediate record semantic")
		}
		return applyTimeline(ctx, tx, entry)
	}

	data, err := domain.DecodeGovernedData(record.Data)
	if err != nil {
		return err
	}
	if record.Status == "proposed" {
		if err := validateControlledPayload(record); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO crab.incident_proposals(incident_id,record_id,record_type,compartment,version,maker,payload,proposed_at,mode) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT (incident_id,record_type,record_id) DO UPDATE SET compartment=EXCLUDED.compartment,version=EXCLUDED.version,maker=EXCLUDED.maker,payload=EXCLUDED.payload,proposed_at=EXCLUDED.proposed_at,mode=EXCLUDED.mode`, record.IncidentID, record.RecordID, record.RecordType, record.Compartment, data.Version, record.Maker, []byte(data.Payload), record.Timestamp, record.IncidentMode); err != nil {
			return err
		}
		return applyTimeline(ctx, tx, entry)
	}
	if record.Status != "confirmed" && record.Status != "rejected" {
		return errors.New("invalid governed incident status")
	}

	var version, maker, compartment, mode string
	var payload []byte
	err = tx.QueryRow(ctx, `SELECT version,maker,compartment,mode,payload FROM crab.incident_proposals WHERE incident_id=$1 AND record_type=$2 AND record_id=$3`, record.IncidentID, record.RecordType, record.RecordID).Scan(&version, &maker, &compartment, &mode, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("decision has no matching pending proposal")
	}
	if err != nil {
		return err
	}
	if version != data.Version || maker != record.Maker || compartment != record.Compartment || mode != record.IncidentMode || !bytes.Equal(payload, []byte(data.Payload)) {
		return errors.New("decision does not match exact pending proposal")
	}
	if record.Status == "confirmed" {
		if err := applyConfirmedIncident(ctx, tx, record); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `DELETE FROM crab.incident_proposals WHERE incident_id=$1 AND record_type=$2 AND record_id=$3`, record.IncidentID, record.RecordType, record.RecordID); err != nil {
		return err
	}
	return applyTimeline(ctx, tx, entry)
}

func validateControlledPayload(record ledger.Record) error {
	switch record.RecordType {
	case domain.TypeIncidentDeclaration:
		_, err := domain.DecodePayload[domain.IncidentDeclaration](record)
		return err
	case domain.TypeDecision:
		_, err := domain.DecodePayload[domain.Decision](record)
		return err
	case domain.TypeAction:
		action, err := domain.DecodePayload[domain.Action](record)
		if err != nil {
			return err
		}
		return domain.ValidateAction(action)
	case domain.TypeOperationalRoleAssignment:
		_, err := domain.DecodePayload[domain.OperationalRoleAssignment](record)
		return err
	default:
		return errors.New("observational incident type cannot use governed semantic")
	}
}

func applyConfirmedIncident(ctx context.Context, tx pgx.Tx, record ledger.Record) error {
	switch record.RecordType {
	case domain.TypeIncidentDeclaration:
		declaration, err := domain.DecodePayload[domain.IncidentDeclaration](record)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO crab.incidents(incident_id,mode,title,summary,severity,declaration_record_id,declared_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, record.IncidentID, record.IncidentMode, declaration.Title, declaration.Summary, declaration.Severity, record.RecordID, record.Timestamp)
		return err
	case domain.TypeDecision:
		decision, err := domain.DecodePayload[domain.Decision](record)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO crab.incident_decisions(incident_id,record_id,compartment,decision_text,rationale,maker,confirmer,confirmed_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, record.IncidentID, record.RecordID, record.Compartment, decision.Decision, decision.Rationale, record.Maker, record.Confirmer, record.Timestamp)
		return err
	case domain.TypeAction:
		action, err := domain.DecodePayload[domain.Action](record)
		if err != nil {
			return err
		}
		if err := domain.ValidateAction(action); err != nil {
			return err
		}
		evidence, err := json.Marshal(action.EvidenceRefs)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO crab.incident_actions(incident_id,record_id,compartment,title,owner_human_id,status,priority,due_at,source_ref,notes,evidence_refs,skip_reason,checklist_template_id,checklist_step_id,maker,confirmer,confirmed_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17) ON CONFLICT (incident_id,record_id) DO UPDATE SET compartment=EXCLUDED.compartment,title=EXCLUDED.title,owner_human_id=EXCLUDED.owner_human_id,status=EXCLUDED.status,priority=EXCLUDED.priority,due_at=EXCLUDED.due_at,source_ref=EXCLUDED.source_ref,notes=EXCLUDED.notes,evidence_refs=EXCLUDED.evidence_refs,skip_reason=EXCLUDED.skip_reason,checklist_template_id=EXCLUDED.checklist_template_id,checklist_step_id=EXCLUDED.checklist_step_id,maker=EXCLUDED.maker,confirmer=EXCLUDED.confirmer,confirmed_at=EXCLUDED.confirmed_at`, record.IncidentID, record.RecordID, record.Compartment, action.Title, action.OwnerHumanID, action.Status, action.Priority, action.DueAt, action.SourceRef, action.Notes, evidence, action.SkipReason, action.ChecklistTemplateID, action.ChecklistStepID, record.Maker, record.Confirmer, record.Timestamp)
		return err
	case domain.TypeOperationalRoleAssignment:
		assignment, err := domain.DecodePayload[domain.OperationalRoleAssignment](record)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO crab.incident_operational_roles(incident_id,record_id,compartment,role_name,human_id,deputy,maker,confirmer,confirmed_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, record.IncidentID, record.RecordID, record.Compartment, assignment.RoleName, assignment.HumanID, assignment.Deputy, record.Maker, record.Confirmer, record.Timestamp)
		return err
	default:
		return errors.New("unsupported controlled incident type")
	}
}

func applyTimeline(ctx context.Context, tx pgx.Tx, entry ledger.Entry) error {
	record := entry.Record
	summary, occurred, flagged, awareness, err := timelineFields(record)
	if err != nil {
		return err
	}
	awarenessJSON, err := json.Marshal(awareness)
	if err != nil {
		return err
	}
	governance := record.Status
	if record.WriteSemantic == "record" {
		governance = "recorded"
	}
	_, err = tx.Exec(ctx, `INSERT INTO crab.incident_timeline(sequence,incident_id,record_id,record_type,governance,compartment,summary,occurred_at,flagged_at,recorded_at,awareness,time_status) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, entry.Sequence, record.IncidentID, record.RecordID, record.RecordType, governance, record.Compartment, summary, occurred, flagged, record.Timestamp, awarenessJSON, record.TimeStatus)
	return err
}

func timelineFields(record ledger.Record) (string, *time.Time, *time.Time, []domain.AwarenessTime, error) {
	switch record.RecordType {
	case domain.TypeIncidentDeclaration:
		v, err := domain.DecodePayload[domain.IncidentDeclaration](record)
		return v.Title, nil, nil, nil, err
	case domain.TypeFact:
		v, err := domain.DecodePayload[domain.Fact](record)
		return v.Statement, v.OccurredAt, v.FlaggedAt, v.Awareness, err
	case domain.TypeUncertainty:
		v, err := domain.DecodePayload[domain.Uncertainty](record)
		return v.Statement, nil, nil, nil, err
	case domain.TypeDecision:
		v, err := domain.DecodePayload[domain.Decision](record)
		return v.Decision, nil, nil, nil, err
	case domain.TypeAction:
		v, err := domain.DecodePayload[domain.Action](record)
		return v.Title, nil, nil, nil, err
	case domain.TypeOperationalRoleAssignment:
		v, err := domain.DecodePayload[domain.OperationalRoleAssignment](record)
		return v.RoleName + ": " + v.HumanID, nil, nil, nil, err
	default:
		return "", nil, nil, nil, errors.New("unsupported timeline type")
	}
}

func slice3Type(recordType string) bool {
	switch recordType {
	case domain.TypeIncidentDeclaration, domain.TypeFact, domain.TypeUncertainty, domain.TypeDecision, domain.TypeAction, domain.TypeOperationalRoleAssignment:
		return true
	default:
		return false
	}
}

type IncidentStore struct {
	pool *pgxpool.Pool
}

func NewIncidentStore(pool *pgxpool.Pool) (*IncidentStore, error) {
	if pool == nil {
		return nil, errors.New("PostgreSQL pool is required")
	}
	return &IncidentStore{pool: pool}, nil
}

func (s *IncidentStore) CommonOperatingPicture(ctx context.Context, incidentID string) (domain.OperatingPicture, bool, error) {
	var picture domain.OperatingPicture
	err := s.pool.QueryRow(ctx, `SELECT incident_id,mode,title,summary,severity,declared_at FROM crab.incidents WHERE incident_id=$1`, incidentID).Scan(&picture.Incident.IncidentID, &picture.Incident.Mode, &picture.Incident.Title, &picture.Incident.Summary, &picture.Incident.Severity, &picture.Incident.DeclaredAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OperatingPicture{}, false, nil
	}
	if err != nil {
		return domain.OperatingPicture{}, false, fmt.Errorf("read incident: %w", err)
	}
	if err := s.readFacts(ctx, incidentID, &picture); err != nil {
		return domain.OperatingPicture{}, false, err
	}
	if err := s.readUncertainties(ctx, incidentID, &picture); err != nil {
		return domain.OperatingPicture{}, false, err
	}
	if err := s.readDecisions(ctx, incidentID, &picture); err != nil {
		return domain.OperatingPicture{}, false, err
	}
	if err := s.readActions(ctx, incidentID, &picture, time.Now().UTC()); err != nil {
		return domain.OperatingPicture{}, false, err
	}
	if err := s.readRoles(ctx, incidentID, &picture); err != nil {
		return domain.OperatingPicture{}, false, err
	}
	if err := s.readPending(ctx, incidentID, &picture); err != nil {
		return domain.OperatingPicture{}, false, err
	}
	if err := s.readTimeline(ctx, incidentID, &picture); err != nil {
		return domain.OperatingPicture{}, false, err
	}
	return picture, true, nil
}

func (s *IncidentStore) readFacts(ctx context.Context, incidentID string, picture *domain.OperatingPicture) error {
	rows, err := s.pool.Query(ctx, `SELECT record_id,compartment,statement,source,maker,recorded_at,occurred_at,flagged_at,awareness FROM crab.incident_facts WHERE incident_id=$1 ORDER BY recorded_at,record_id`, incidentID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var v domain.FactView
		var awareness []byte
		if err := rows.Scan(&v.RecordID, &v.Compartment, &v.Fact.Statement, &v.Fact.Source, &v.Maker, &v.RecordedAt, &v.Fact.OccurredAt, &v.Fact.FlaggedAt, &awareness); err != nil {
			return err
		}
		if err := json.Unmarshal(awareness, &v.Fact.Awareness); err != nil {
			return err
		}
		picture.Facts = append(picture.Facts, v)
	}
	return rows.Err()
}

func (s *IncidentStore) readUncertainties(ctx context.Context, incidentID string, picture *domain.OperatingPicture) error {
	rows, err := s.pool.Query(ctx, `SELECT record_id,compartment,kind,statement,maker,recorded_at FROM crab.incident_uncertainties WHERE incident_id=$1 ORDER BY recorded_at,record_id`, incidentID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var v domain.UncertaintyView
		if err := rows.Scan(&v.RecordID, &v.Compartment, &v.Uncertainty.Kind, &v.Uncertainty.Statement, &v.Maker, &v.RecordedAt); err != nil {
			return err
		}
		if v.Uncertainty.Kind == "unknown" {
			picture.Unknowns = append(picture.Unknowns, v)
		} else {
			picture.Hypotheses = append(picture.Hypotheses, v)
		}
	}
	return rows.Err()
}

func (s *IncidentStore) readDecisions(ctx context.Context, incidentID string, picture *domain.OperatingPicture) error {
	rows, err := s.pool.Query(ctx, `SELECT record_id,compartment,decision_text,rationale,maker,confirmer,confirmed_at FROM crab.incident_decisions WHERE incident_id=$1 ORDER BY confirmed_at,record_id`, incidentID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var v domain.DecisionView
		if err := rows.Scan(&v.RecordID, &v.Compartment, &v.Decision.Decision, &v.Decision.Rationale, &v.Maker, &v.Confirmer, &v.ConfirmedAt); err != nil {
			return err
		}
		picture.Decisions = append(picture.Decisions, v)
	}
	return rows.Err()
}

func (s *IncidentStore) readActions(ctx context.Context, incidentID string, picture *domain.OperatingPicture, now time.Time) error {
	rows, err := s.pool.Query(ctx, `SELECT record_id,compartment,title,owner_human_id,status,priority,due_at,source_ref,notes,evidence_refs,skip_reason,checklist_template_id,checklist_step_id,maker,confirmer,confirmed_at FROM crab.incident_actions WHERE incident_id=$1 ORDER BY confirmed_at,record_id`, incidentID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var v domain.ActionView
		var evidence []byte
		if err := rows.Scan(&v.RecordID, &v.Compartment, &v.Action.Title, &v.Action.OwnerHumanID, &v.Action.Status, &v.Action.Priority, &v.Action.DueAt, &v.Action.SourceRef, &v.Action.Notes, &evidence, &v.Action.SkipReason, &v.Action.ChecklistTemplateID, &v.Action.ChecklistStepID, &v.Maker, &v.Confirmer, &v.ConfirmedAt); err != nil {
			return err
		}
		if err := json.Unmarshal(evidence, &v.Action.EvidenceRefs); err != nil {
			return err
		}
		v.Overdue = domain.ActionOverdue(v.Action, now)
		picture.Actions = append(picture.Actions, v)
	}
	return rows.Err()
}

func (s *IncidentStore) readRoles(ctx context.Context, incidentID string, picture *domain.OperatingPicture) error {
	rows, err := s.pool.Query(ctx, `SELECT record_id,compartment,role_name,human_id,deputy,maker,confirmer,confirmed_at FROM crab.incident_operational_roles WHERE incident_id=$1 ORDER BY confirmed_at,record_id`, incidentID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var v domain.OperationalRoleView
		if err := rows.Scan(&v.RecordID, &v.Compartment, &v.Assignment.RoleName, &v.Assignment.HumanID, &v.Assignment.Deputy, &v.Maker, &v.Confirmer, &v.ConfirmedAt); err != nil {
			return err
		}
		picture.OperationalRoles = append(picture.OperationalRoles, v)
	}
	return rows.Err()
}

func (s *IncidentStore) readPending(ctx context.Context, incidentID string, picture *domain.OperatingPicture) error {
	rows, err := s.pool.Query(ctx, `SELECT record_id,record_type,compartment,version,maker,proposed_at FROM crab.incident_proposals WHERE incident_id=$1 ORDER BY proposed_at,record_type,record_id`, incidentID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var v domain.PendingProposal
		if err := rows.Scan(&v.RecordID, &v.RecordType, &v.Compartment, &v.Version, &v.Maker, &v.ProposedAt); err != nil {
			return err
		}
		picture.Pending = append(picture.Pending, v)
	}
	return rows.Err()
}

func (s *IncidentStore) readTimeline(ctx context.Context, incidentID string, picture *domain.OperatingPicture) error {
	rows, err := s.pool.Query(ctx, `SELECT sequence,record_id,record_type,governance,compartment,summary,occurred_at,flagged_at,recorded_at,awareness,time_status FROM crab.incident_timeline WHERE incident_id=$1 ORDER BY recorded_at,sequence`, incidentID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var v domain.TimelineEntry
		var awareness []byte
		if err := rows.Scan(&v.Sequence, &v.RecordID, &v.RecordType, &v.Governance, &v.Compartment, &v.Summary, &v.OccurredAt, &v.FlaggedAt, &v.RecordedAt, &awareness, &v.TimeStatus); err != nil {
			return err
		}
		if err := json.Unmarshal(awareness, &v.Awareness); err != nil {
			return err
		}
		picture.Timeline = append(picture.Timeline, v)
	}
	return rows.Err()
}
