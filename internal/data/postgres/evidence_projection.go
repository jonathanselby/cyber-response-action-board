package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

type EvidenceProjection struct{}

func (EvidenceProjection) Reset(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `TRUNCATE crab.evidence_handling, crab.evidence_register`)
	return err
}

func (EvidenceProjection) Apply(ctx context.Context, tx pgx.Tx, entry ledger.Entry) error {
	record := entry.Record
	if record.RecordType != domain.TypeEvidence && record.RecordType != domain.TypeEvidenceHandling {
		return nil
	}
	if record.WriteSemantic != "record" || record.Status != "recorded" {
		return errors.New("evidence history requires immediate record semantics")
	}
	if record.IncidentID == "" || (record.IncidentMode != "live" && record.IncidentMode != "exercise") || record.Compartment == "" {
		return errors.New("evidence history requires incident scope")
	}
	switch record.RecordType {
	case domain.TypeEvidence:
		value, err := domain.DecodePayload[domain.Evidence](record)
		if err != nil {
			return err
		}
		if err := domain.ValidateEvidence(value); err != nil {
			return err
		}
		var incidentID, mode, compartment, sha, filename, declared, detected string
		var size int64
		err = tx.QueryRow(ctx, `SELECT incident_id,mode,compartment,sha256,size_bytes,original_filename,declared_media_type,detected_media_type FROM crab.blobs WHERE blob_id=$1`, value.BlobID).Scan(&incidentID, &mode, &compartment, &sha, &size, &filename, &declared, &detected)
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("evidence references unknown blob")
		}
		if err != nil {
			return err
		}
		if incidentID != record.IncidentID || mode != record.IncidentMode || compartment != record.Compartment || sha != value.SHA256 || size != value.SizeBytes || filename != value.OriginalFilename || declared != value.DeclaredMediaType || detected != value.DetectedMediaType {
			return errors.New("evidence record does not exactly bind to immutable blob metadata")
		}
		_, err = tx.Exec(ctx, `INSERT INTO crab.evidence_register(evidence_id,incident_id,mode,compartment,blob_id,sha256,size_bytes,original_filename,declared_media_type,detected_media_type,description,collector,source,collected_at,occurred_at,source_location,current_handler,related_ref,recorded_at,time_status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`, record.RecordID, record.IncidentID, record.IncidentMode, record.Compartment, value.BlobID, value.SHA256, value.SizeBytes, value.OriginalFilename, value.DeclaredMediaType, value.DetectedMediaType, value.Description, value.Collector, value.Source, value.CollectedAt, value.OccurredAt, value.SourceLocation, value.CurrentHandler, value.RelatedRef, record.Timestamp, record.TimeStatus)
		return err
	case domain.TypeEvidenceHandling:
		value, err := domain.DecodePayload[domain.EvidenceHandling](record)
		if err != nil {
			return err
		}
		if err := domain.ValidateEvidenceHandling(value); err != nil {
			return err
		}
		var incidentID, mode, compartment string
		err = tx.QueryRow(ctx, `SELECT incident_id,mode,compartment FROM crab.evidence_register WHERE evidence_id=$1`, value.EvidenceID).Scan(&incidentID, &mode, &compartment)
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("evidence handling references unknown evidence")
		}
		if err != nil {
			return err
		}
		if incidentID != record.IncidentID || mode != record.IncidentMode || compartment != record.Compartment {
			return errors.New("evidence handling scope does not match evidence")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO crab.evidence_handling(record_id,evidence_id,incident_id,mode,compartment,action,handler,location,note,occurred_at,recorded_at,time_status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, record.RecordID, value.EvidenceID, record.IncidentID, record.IncidentMode, record.Compartment, value.Action, value.Handler, value.Location, value.Note, value.OccurredAt, record.Timestamp, record.TimeStatus); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE crab.evidence_register SET current_handler=$2 WHERE evidence_id=$1`, value.EvidenceID, value.Handler)
		return err
	}
	return nil
}

type EvidenceStore struct{ pool *pgxpool.Pool }

func NewEvidenceStore(pool *pgxpool.Pool) (*EvidenceStore, error) {
	if pool == nil {
		return nil, errors.New("PostgreSQL pool is required")
	}
	return &EvidenceStore{pool: pool}, nil
}

func (s *EvidenceStore) Evidence(ctx context.Context, evidenceID string) (domain.EvidenceView, bool, error) {
	var view domain.EvidenceView
	var collectedAt, occurredAt sql.NullTime
	err := s.pool.QueryRow(ctx, `SELECT evidence_id,incident_id,mode,compartment,recorded_at,time_status,blob_id,sha256,size_bytes,original_filename,declared_media_type,detected_media_type,description,collector,source,collected_at,occurred_at,source_location,current_handler,related_ref FROM crab.evidence_register WHERE evidence_id=$1`, evidenceID).Scan(&view.EvidenceID, &view.IncidentID, &view.Mode, &view.Compartment, &view.RecordedAt, &view.TimeStatus, &view.Evidence.BlobID, &view.Evidence.SHA256, &view.Evidence.SizeBytes, &view.Evidence.OriginalFilename, &view.Evidence.DeclaredMediaType, &view.Evidence.DetectedMediaType, &view.Evidence.Description, &view.Evidence.Collector, &view.Evidence.Source, &collectedAt, &occurredAt, &view.Evidence.SourceLocation, &view.Evidence.CurrentHandler, &view.Evidence.RelatedRef)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.EvidenceView{}, false, nil
	}
	if err != nil {
		return domain.EvidenceView{}, false, fmt.Errorf("read evidence: %w", err)
	}
	view.Evidence.CollectedAt = nullTimePtr(collectedAt)
	view.Evidence.OccurredAt = nullTimePtr(occurredAt)
	return view, true, nil
}

func (s *EvidenceStore) History(ctx context.Context, evidenceID string) ([]domain.EvidenceHandlingView, error) {
	rows, err := s.pool.Query(ctx, `SELECT record_id,evidence_id,incident_id,mode,compartment,recorded_at,time_status,action,handler,location,note,occurred_at FROM crab.evidence_handling WHERE evidence_id=$1 ORDER BY recorded_at,record_id`, evidenceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.EvidenceHandlingView
	for rows.Next() {
		var view domain.EvidenceHandlingView
		var occurredAt sql.NullTime
		if err := rows.Scan(&view.RecordID, &view.EvidenceID, &view.IncidentID, &view.Mode, &view.Compartment, &view.RecordedAt, &view.TimeStatus, &view.Handling.Action, &view.Handling.Handler, &view.Handling.Location, &view.Handling.Note, &occurredAt); err != nil {
			return nil, err
		}
		view.Handling.EvidenceID = view.EvidenceID
		view.Handling.OccurredAt = nullTimePtr(occurredAt)
		result = append(result, view)
	}
	return result, rows.Err()
}

func (s *EvidenceStore) Register(ctx context.Context, incidentID, compartment string) ([]domain.EvidenceView, error) {
	if compartment == "" {
		compartment = "GENERAL"
	}
	rows, err := s.pool.Query(ctx, `SELECT evidence_id FROM crab.evidence_register WHERE incident_id=$1 AND (compartment='GENERAL' OR compartment=$2) ORDER BY recorded_at,evidence_id`, incidentID, compartment)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	result := make([]domain.EvidenceView, 0, len(ids))
	for _, id := range ids {
		view, found, err := s.Evidence(ctx, id)
		if err != nil {
			return nil, err
		}
		if found {
			result = append(result, view)
		}
	}
	return result, nil
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	v := value.Time
	return &v
}
