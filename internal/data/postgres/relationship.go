package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

// RelationshipProjection keeps pending and active incident relationships.
// Confirmed links are stored in both directions so either incident's Board
// can list them; readers filter by what the viewer may see.
type RelationshipProjection struct{}

func (RelationshipProjection) Reset(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `TRUNCATE crab.incident_relationship_proposals, crab.incident_relationships`)
	return err
}

func (RelationshipProjection) Apply(ctx context.Context, tx pgx.Tx, entry ledger.Entry) error {
	record := entry.Record
	if record.RecordType != domain.TypeIncidentRelationship {
		return nil
	}
	if record.WriteSemantic != "governed" || record.Compartment != "GENERAL" || (record.IncidentMode != "live" && record.IncidentMode != "exercise") {
		return errors.New("incident relationships must be governed GENERAL incident records")
	}
	data, err := domain.DecodeGovernedData(record.Data)
	if err != nil {
		return err
	}
	var value domain.IncidentRelationship
	if err := decodeStrictJSON(data.Payload, &value); err != nil {
		return err
	}
	if err := domain.ValidateIncidentRelationship(record.IncidentID, value); err != nil {
		return err
	}
	if record.Status == "proposed" {
		_, err := tx.Exec(ctx, `INSERT INTO crab.incident_relationship_proposals(incident_id,record_id,related_incident_id,version,maker,payload,proposed_at,mode) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, record.IncidentID, record.RecordID, value.RelatedIncidentID, data.Version, record.Maker, []byte(data.Payload), record.Timestamp, record.IncidentMode)
		return err
	}
	var version, maker string
	var payload []byte
	err = tx.QueryRow(ctx, `SELECT version,maker,payload FROM crab.incident_relationship_proposals WHERE incident_id=$1 AND record_id=$2`, record.IncidentID, record.RecordID).Scan(&version, &maker, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("relationship decision has no matching pending proposal")
	}
	if err != nil {
		return err
	}
	if version != data.Version || maker != record.Maker || !jsonEqual(payload, data.Payload) {
		return errors.New("relationship decision does not match exact pending proposal")
	}
	if _, err := tx.Exec(ctx, `DELETE FROM crab.incident_relationship_proposals WHERE incident_id=$1 AND record_id=$2`, record.IncidentID, record.RecordID); err != nil {
		return err
	}
	if record.Status != "confirmed" {
		return nil
	}
	a, b := record.IncidentID, value.RelatedIncidentID
	if !value.Active {
		_, err := tx.Exec(ctx, `DELETE FROM crab.incident_relationships WHERE (incident_id=$1 AND related_incident_id=$2) OR (incident_id=$2 AND related_incident_id=$1)`, a, b)
		return err
	}
	for _, pair := range [][2]string{{a, b}, {b, a}} {
		if _, err := tx.Exec(ctx, `INSERT INTO crab.incident_relationships(incident_id,related_incident_id,reason,record_id,maker,confirmer,confirmed_at) VALUES($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT(incident_id,related_incident_id) DO UPDATE SET reason=EXCLUDED.reason,record_id=EXCLUDED.record_id,maker=EXCLUDED.maker,confirmer=EXCLUDED.confirmer,confirmed_at=EXCLUDED.confirmed_at`,
			pair[0], pair[1], value.Reason, record.RecordID, record.Maker, record.Confirmer, record.Timestamp); err != nil {
			return err
		}
	}
	return nil
}

func decodeStrictJSON(raw []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// RelationshipRuntime appends and reads relationships. Callers must filter by
// the viewer's incident authority before showing anything.
type RelationshipRuntime struct {
	Kernel *Kernel
	Signer Signer
}

func NewRelationshipRuntime(kernel *Kernel, signer Signer) (*RelationshipRuntime, error) {
	if kernel == nil || signer.KeyID == "" {
		return nil, errors.New("relationship runtime requires kernel and signer")
	}
	return &RelationshipRuntime{Kernel: kernel, Signer: signer}, nil
}

func (r *RelationshipRuntime) AppendRelationship(ctx context.Context, record ledger.Record) error {
	_, err := r.Kernel.Append(ctx, record, r.Signer, RelationshipProjection{})
	return err
}

func (r *RelationshipRuntime) PendingRelationship(ctx context.Context, incidentID, recordID string) (ledger.Record, bool, error) {
	var version, maker, mode string
	var payload []byte
	var proposedAt time.Time
	err := r.Kernel.pool.QueryRow(ctx, `SELECT version,maker,payload,proposed_at,mode FROM crab.incident_relationship_proposals WHERE incident_id=$1 AND record_id=$2`, incidentID, recordID).Scan(&version, &maker, &payload, &proposedAt, &mode)
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
	record := ledger.Record{SchemaVersion: 1, Timestamp: proposedAt.UTC(), TimeStatus: ledger.TimeTrusted, IncidentID: incidentID, IncidentMode: mode, WriteSemantic: "governed", RecordType: domain.TypeIncidentRelationship, RecordID: recordID, Status: "proposed", Maker: maker, Compartment: "GENERAL", Data: wrapped, MakerSignature: ledger.Signature{MissingReason: "maker authenticated on immutable proposal entry"}}
	if _, err := domain.ProposalVersion(record); err != nil {
		return ledger.Record{}, false, err
	}
	return record, true, nil
}

// PendingRelationshipView is one pending link proposal.
type PendingRelationshipView struct {
	IncidentID, RelatedIncidentID, RecordID, Version, Maker string
	ProposedAt                                              time.Time
}

func (r *RelationshipRuntime) PendingRelationships(ctx context.Context, incidentID string) ([]PendingRelationshipView, error) {
	rows, err := r.Kernel.pool.Query(ctx, `SELECT incident_id,related_incident_id,record_id,version,maker,proposed_at FROM crab.incident_relationship_proposals WHERE incident_id=$1 ORDER BY proposed_at,record_id`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PendingRelationshipView
	for rows.Next() {
		var v PendingRelationshipView
		if err := rows.Scan(&v.IncidentID, &v.RelatedIncidentID, &v.RecordID, &v.Version, &v.Maker, &v.ProposedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *RelationshipRuntime) Relationships(ctx context.Context, incidentID string) ([]domain.RelationshipView, error) {
	rows, err := r.Kernel.pool.Query(ctx, `SELECT incident_id,related_incident_id,reason,record_id,maker,confirmer,confirmed_at FROM crab.incident_relationships WHERE incident_id=$1 ORDER BY confirmed_at,related_incident_id`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.RelationshipView
	for rows.Next() {
		var v domain.RelationshipView
		if err := rows.Scan(&v.IncidentID, &v.RelatedIncidentID, &v.Reason, &v.RecordID, &v.Maker, &v.Confirmer, &v.ConfirmedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
