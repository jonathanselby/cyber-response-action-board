package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

type RecoveryProjection struct{}

func (RecoveryProjection) Reset(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `DELETE FROM crab.incident_timeline WHERE record_type IN ($1,$2,$3,$4,$5)`, domain.TypeRecoveryUpdate, domain.TypeStandDown, domain.TypePIR, domain.TypeRetention, domain.TypeClosure); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `TRUNCATE crab.incident_recovery_proposals, crab.incident_recovery_updates, crab.incident_recovery_state`)
	return err
}

func (RecoveryProjection) Apply(ctx context.Context, tx pgx.Tx, entry ledger.Entry) error {
	record := entry.Record
	if !domain.IsRecoveryType(record.RecordType) {
		return nil
	}
	if record.IncidentID == "" || (record.IncidentMode != "live" && record.IncidentMode != "exercise") {
		return errors.New("recovery projection requires live/exercise incident scope")
	}
	closed, err := recoveryClosedTx(ctx, tx, record.IncidentID)
	if err != nil {
		return err
	}
	if closed {
		return domain.ErrIncidentClosed
	}

	if record.RecordType == domain.TypeRecoveryUpdate {
		if record.WriteSemantic != "record" || record.Status != "recorded" {
			return errors.New("recovery update requires record semantic")
		}
		value, err := domain.DecodePayload[domain.RecoveryUpdate](record)
		if err != nil {
			return err
		}
		if err := domain.ValidateRecoveryUpdate(value); err != nil {
			return err
		}
		payload, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO crab.incident_recovery_updates(incident_id,item_id,record_id,compartment,payload,maker,recorded_at) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (incident_id,item_id) DO UPDATE SET record_id=EXCLUDED.record_id,compartment=EXCLUDED.compartment,payload=EXCLUDED.payload,maker=EXCLUDED.maker,recorded_at=EXCLUDED.recorded_at`, record.IncidentID, value.ItemID, record.RecordID, record.Compartment, payload, record.Maker, record.Timestamp); err != nil {
			return err
		}
		return applyRecoveryTimeline(ctx, tx, entry, value.Summary)
	}
	if record.WriteSemantic != "governed" {
		return errors.New("recovery milestone requires governed semantic")
	}
	data, err := domain.DecodeGovernedData(record.Data)
	if err != nil {
		return err
	}
	if err := validateRecoveryPayload(record); err != nil {
		return err
	}
	if record.Status == "proposed" {
		if _, err := tx.Exec(ctx, `INSERT INTO crab.incident_recovery_proposals(incident_id,record_id,record_type,compartment,version,maker,payload,proposed_at,mode) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, record.IncidentID, record.RecordID, record.RecordType, record.Compartment, data.Version, record.Maker, []byte(data.Payload), record.Timestamp, record.IncidentMode); err != nil {
			return err
		}
		return applyRecoveryTimeline(ctx, tx, entry, recoverySummary(record))
	}
	if record.Status != "confirmed" && record.Status != "rejected" {
		return errors.New("invalid governed recovery status")
	}

	var version, maker, compartment, mode string
	var payload []byte
	err = tx.QueryRow(ctx, `SELECT version,maker,compartment,mode,payload FROM crab.incident_recovery_proposals WHERE incident_id=$1 AND record_type=$2 AND record_id=$3`, record.IncidentID, record.RecordType, record.RecordID).Scan(&version, &maker, &compartment, &mode, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("recovery decision has no matching pending proposal")
	}
	if err != nil {
		return err
	}
	if version != data.Version || maker != record.Maker || compartment != record.Compartment || mode != record.IncidentMode || !bytes.Equal(payload, []byte(data.Payload)) {
		return errors.New("recovery decision does not match exact pending proposal")
	}
	if record.Status == "confirmed" {
		if err := applyConfirmedRecovery(ctx, tx, record); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `DELETE FROM crab.incident_recovery_proposals WHERE incident_id=$1 AND record_type=$2 AND record_id=$3`, record.IncidentID, record.RecordType, record.RecordID); err != nil {
		return err
	}
	return applyRecoveryTimeline(ctx, tx, entry, recoverySummary(record))
}

func validateRecoveryPayload(record ledger.Record) error {
	switch record.RecordType {
	case domain.TypeStandDown:
		value, err := domain.DecodePayload[domain.StandDown](record)
		if err != nil {
			return err
		}
		if value.Reason == "" {
			return errors.New("stand-down reason is required")
		}
		return nil
	case domain.TypePIR:
		value, err := domain.DecodePayload[domain.PIR](record)
		if err != nil {
			return err
		}
		return domain.ValidatePIR(value)
	case domain.TypeRetention:
		value, err := domain.DecodePayload[domain.RetentionDisposition](record)
		if err != nil {
			return err
		}
		return domain.ValidateRetention(value)
	case domain.TypeClosure:
		value, err := domain.DecodePayload[domain.Closure](record)
		if err != nil {
			return err
		}
		return domain.ValidateClosure(value)
	default:
		return errors.New("unsupported recovery milestone")
	}
}

func applyConfirmedRecovery(ctx context.Context, tx pgx.Tx, record ledger.Record) error {
	if record.Compartment != "GENERAL" {
		return errors.New("incident recovery milestones must use GENERAL compartment")
	}
	if _, err := tx.Exec(ctx, `INSERT INTO crab.incident_recovery_state(incident_id) VALUES ($1) ON CONFLICT (incident_id) DO NOTHING`, record.IncidentID); err != nil {
		return err
	}
	switch record.RecordType {
	case domain.TypeStandDown:
		value, err := domain.DecodePayload[domain.StandDown](record)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(value)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE crab.incident_recovery_state SET stand_down_record_id=$2,stand_down_at=$3,stand_down_payload=$4 WHERE incident_id=$1`, record.IncidentID, record.RecordID, record.Timestamp, payload)
		return err
	case domain.TypePIR:
		value, err := domain.DecodePayload[domain.PIR](record)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(value)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE crab.incident_recovery_state SET pir_record_id=$2,pir_at=$3,pir_payload=$4 WHERE incident_id=$1`, record.IncidentID, record.RecordID, record.Timestamp, payload)
		return err
	case domain.TypeRetention:
		value, err := domain.DecodePayload[domain.RetentionDisposition](record)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(value)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE crab.incident_recovery_state SET retention_record_id=$2,retention_at=$3,retention_payload=$4 WHERE incident_id=$1`, record.IncidentID, record.RecordID, record.Timestamp, payload)
		return err
	case domain.TypeClosure:
		state, err := readRecoveryStateTx(ctx, tx, record.IncidentID)
		if err != nil {
			return err
		}
		if err := domain.ClosurePrerequisites(state); err != nil {
			return err
		}
		value, err := domain.DecodePayload[domain.Closure](record)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(value)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE crab.incident_recovery_state SET closure_record_id=$2,closed_at=$3,closure_payload=$4 WHERE incident_id=$1`, record.IncidentID, record.RecordID, record.Timestamp, payload)
		return err
	default:
		return errors.New("unsupported confirmed recovery milestone")
	}
}

func applyRecoveryTimeline(ctx context.Context, tx pgx.Tx, entry ledger.Entry, summary string) error {
	record := entry.Record
	governance := record.Status
	if record.WriteSemantic == "record" {
		governance = "recorded"
	}
	_, err := tx.Exec(ctx, `INSERT INTO crab.incident_timeline(sequence,incident_id,record_id,record_type,governance,compartment,summary,recorded_at,awareness,time_status) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'[]'::jsonb,$9)`, entry.Seq, record.IncidentID, record.RecordID, record.RecordType, governance, record.Compartment, summary, record.Timestamp, record.TimeStatus)
	return err
}

func recoverySummary(record ledger.Record) string {
	switch record.RecordType {
	case domain.TypeStandDown:
		value, err := domain.DecodePayload[domain.StandDown](record)
		if err == nil {
			return "Stand-down: " + value.Reason
		}
	case domain.TypePIR:
		return "Post-Incident Review"
	case domain.TypeRetention:
		return "Retention / hold / disposition hand-off"
	case domain.TypeClosure:
		return "Administrative closure"
	}
	return record.RecordType
}

type RecoveryStore struct {
	pool      *pgxpool.Pool
	incidents *IncidentStore
}

func NewRecoveryStore(pool *pgxpool.Pool) (*RecoveryStore, error) {
	if pool == nil {
		return nil, errors.New("PostgreSQL pool is required")
	}
	incidents, err := NewIncidentStore(pool)
	if err != nil {
		return nil, err
	}
	return &RecoveryStore{pool: pool, incidents: incidents}, nil
}

func (s *RecoveryStore) State(ctx context.Context, incidentID string) (domain.RecoveryState, bool, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM crab.incidents WHERE incident_id=$1)`, incidentID).Scan(&exists); err != nil {
		return domain.RecoveryState{}, false, err
	}
	if !exists {
		return domain.RecoveryState{}, false, nil
	}
	state := domain.RecoveryState{IncidentID: incidentID}
	rows, err := s.pool.Query(ctx, `SELECT record_id,compartment,payload,maker,recorded_at FROM crab.incident_recovery_updates WHERE incident_id=$1 ORDER BY recorded_at,item_id`, incidentID)
	if err != nil {
		return domain.RecoveryState{}, false, err
	}
	for rows.Next() {
		var view domain.RecoveryUpdateView
		var payload []byte
		if err := rows.Scan(&view.RecordID, &view.Compartment, &payload, &view.Maker, &view.RecordedAt); err != nil {
			rows.Close()
			return domain.RecoveryState{}, false, err
		}
		if err := json.Unmarshal(payload, &view.Update); err != nil {
			rows.Close()
			return domain.RecoveryState{}, false, err
		}
		state.Updates = append(state.Updates, view)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return domain.RecoveryState{}, false, err
	}
	rows.Close()
	if err := readRecoveryMilestones(ctx, s.pool, &state); err != nil {
		return domain.RecoveryState{}, false, err
	}
	return domain.FinaliseRecoveryState(state), true, nil
}

func readRecoveryMilestones(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, state *domain.RecoveryState) error {
	var standDownAt, pirAt, retentionAt, closedAt *time.Time
	var standDownRaw, pirRaw, retentionRaw, closureRaw []byte
	err := q.QueryRow(ctx, `SELECT stand_down_at,stand_down_payload,pir_at,pir_payload,retention_at,retention_payload,closed_at,closure_payload FROM crab.incident_recovery_state WHERE incident_id=$1`, state.IncidentID).Scan(&standDownAt, &standDownRaw, &pirAt, &pirRaw, &retentionAt, &retentionRaw, &closedAt, &closureRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	state.StandDownAt, state.PIRAt, state.RetentionAt, state.ClosedAt = standDownAt, pirAt, retentionAt, closedAt
	if len(standDownRaw) > 0 {
		state.StandDown = &domain.StandDown{}
		if err := json.Unmarshal(standDownRaw, state.StandDown); err != nil {
			return err
		}
	}
	if len(pirRaw) > 0 {
		state.PIR = &domain.PIR{}
		if err := json.Unmarshal(pirRaw, state.PIR); err != nil {
			return err
		}
	}
	if len(retentionRaw) > 0 {
		state.Retention = &domain.RetentionDisposition{}
		if err := json.Unmarshal(retentionRaw, state.Retention); err != nil {
			return err
		}
	}
	if len(closureRaw) > 0 {
		state.Closure = &domain.Closure{}
		if err := json.Unmarshal(closureRaw, state.Closure); err != nil {
			return err
		}
	}
	return nil
}

func readRecoveryStateTx(ctx context.Context, tx pgx.Tx, incidentID string) (domain.RecoveryState, error) {
	state := domain.RecoveryState{IncidentID: incidentID}
	rows, err := tx.Query(ctx, `SELECT record_id,compartment,payload,maker,recorded_at FROM crab.incident_recovery_updates WHERE incident_id=$1 ORDER BY recorded_at,item_id`, incidentID)
	if err != nil {
		return state, err
	}
	for rows.Next() {
		var view domain.RecoveryUpdateView
		var payload []byte
		if err := rows.Scan(&view.RecordID, &view.Compartment, &payload, &view.Maker, &view.RecordedAt); err != nil {
			rows.Close()
			return state, err
		}
		if err := json.Unmarshal(payload, &view.Update); err != nil {
			rows.Close()
			return state, err
		}
		state.Updates = append(state.Updates, view)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return state, err
	}
	rows.Close()
	if err := readRecoveryMilestones(ctx, tx, &state); err != nil {
		return state, err
	}
	return domain.FinaliseRecoveryState(state), nil
}

func recoveryClosedTx(ctx context.Context, tx pgx.Tx, incidentID string) (bool, error) {
	var closed bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM crab.incident_recovery_state WHERE incident_id=$1 AND closed_at IS NOT NULL)`, incidentID).Scan(&closed)
	return closed, err
}

type RecoveryRuntime struct {
	Kernel *Kernel
	Signer Signer
	Store  *RecoveryStore
}

func NewRecoveryRuntime(kernel *Kernel, signer Signer, store *RecoveryStore) (*RecoveryRuntime, error) {
	if kernel == nil || store == nil || signer.KeyID == "" {
		return nil, errors.New("recovery runtime requires kernel, signer and store")
	}
	return &RecoveryRuntime{Kernel: kernel, Signer: signer, Store: store}, nil
}

func (r *RecoveryRuntime) AppendRecovery(ctx context.Context, record ledger.Record) error {
	_, err := r.Kernel.Append(ctx, record, r.Signer, RecoveryProjection{})
	return err
}

func (r *RecoveryRuntime) PendingRecovery(ctx context.Context, incidentID, recordType, recordID string) (ledger.Record, bool, error) {
	var version, maker, compartment, mode string
	var payload []byte
	var proposedAt time.Time
	err := r.Store.pool.QueryRow(ctx, `SELECT version,maker,compartment,mode,payload,proposed_at FROM crab.incident_recovery_proposals WHERE incident_id=$1 AND record_type=$2 AND record_id=$3`, incidentID, recordType, recordID).Scan(&version, &maker, &compartment, &mode, &payload, &proposedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ledger.Record{}, false, nil
	}
	if err != nil {
		return ledger.Record{}, false, err
	}
	wrapped, err := json.Marshal(domain.GovernedData{Version: version, Payload: json.RawMessage(payload)})
	if err != nil {
		return ledger.Record{}, false, err
	}
	record := ledger.Record{SchemaVersion: 1, Timestamp: proposedAt.UTC(), TimeStatus: "trusted", IncidentID: incidentID, IncidentMode: mode, WriteSemantic: "governed", RecordType: recordType, RecordID: recordID, Status: "proposed", Maker: maker, Compartment: compartment, Data: wrapped, MakerSignature: ledger.Signature{MissingReason: "maker authenticated on immutable proposal entry"}}
	if _, err := domain.ProposalVersion(record); err != nil {
		return ledger.Record{}, false, err
	}
	return record, true, nil
}

func (r *RecoveryRuntime) RecoveryState(ctx context.Context, incidentID string) (domain.RecoveryState, bool, error) {
	return r.Store.State(ctx, incidentID)
}

func (r *RecoveryRuntime) CommonOperatingPicture(ctx context.Context, incidentID string) (domain.OperatingPicture, bool, error) {
	return r.Store.incidents.CommonOperatingPicture(ctx, incidentID)
}
