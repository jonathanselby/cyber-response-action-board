package postgres

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

type CorpusProjection struct{}

func (CorpusProjection) Reset(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `TRUNCATE crab.corpus_passages, crab.corpus_sources, crab.corpus_proposals`)
	return err
}

func (CorpusProjection) Apply(ctx context.Context, tx pgx.Tx, entry ledger.Entry) error {
	record := entry.Record
	if record.RecordType != domain.TypeCorpusSource {
		return nil
	}
	if record.WriteSemantic != "governed" {
		return errors.New("corpus source requires governed semantics")
	}
	data, err := domain.DecodeGovernedData(record.Data)
	if err != nil {
		return err
	}
	value, err := domain.DecodePayload[domain.CorpusSource](record)
	if err != nil {
		return err
	}
	if err := domain.ValidateCorpusSource(value); err != nil {
		return err
	}
	scope := domain.CorpusScope{IncidentID: record.IncidentID, Mode: record.IncidentMode, Compartment: record.Compartment}
	if err := domain.ValidateCorpusScope(scope); err != nil {
		return err
	}
	if err := exactCorpusBlobBinding(ctx, tx, scope, value); err != nil {
		return err
	}

	if record.Status == "proposed" {
		_, err := tx.Exec(ctx, `INSERT INTO crab.corpus_proposals(source_id,incident_id,mode,compartment,version,maker,payload,proposed_at,time_status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, record.RecordID, record.IncidentID, record.IncidentMode, record.Compartment, data.Version, record.Maker, []byte(data.Payload), record.Timestamp, record.TimeStatus)
		return err
	}
	if record.Status != "confirmed" && record.Status != "rejected" {
		return errors.New("invalid corpus governance status")
	}

	var incidentID, mode, compartment, version, maker, timeStatus string
	var payload []byte
	if err := tx.QueryRow(ctx, `SELECT incident_id,mode,compartment,version,maker,time_status,payload FROM crab.corpus_proposals WHERE source_id=$1`, record.RecordID).Scan(&incidentID, &mode, &compartment, &version, &maker, &timeStatus, &payload); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("corpus decision has no matching pending proposal")
		}
		return err
	}
	if incidentID != record.IncidentID || mode != record.IncidentMode || compartment != record.Compartment || version != data.Version || maker != record.Maker || timeStatus != record.TimeStatus || !bytes.Equal(payload, []byte(data.Payload)) {
		return errors.New("corpus decision does not match exact pending proposal")
	}
	if record.Status == "confirmed" {
		if err := applyConfirmedCorpus(ctx, tx, record, value); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `DELETE FROM crab.corpus_proposals WHERE source_id=$1`, record.RecordID)
	return err
}

func exactCorpusBlobBinding(ctx context.Context, tx pgx.Tx, scope domain.CorpusScope, value domain.CorpusSource) error {
	var incidentID, mode, compartment, sha, filename, declared, detected string
	var size int64
	err := tx.QueryRow(ctx, `SELECT incident_id,mode,compartment,sha256,size_bytes,original_filename,declared_media_type,detected_media_type FROM crab.blobs WHERE blob_id=$1`, value.BlobID).Scan(&incidentID, &mode, &compartment, &sha, &size, &filename, &declared, &detected)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("corpus source references unknown blob")
	}
	if err != nil {
		return err
	}
	if incidentID != scope.IncidentID || mode != scope.Mode || compartment != scope.Compartment || sha != value.SHA256 || size != value.SizeBytes || filename != value.OriginalFilename || declared != value.DeclaredMediaType || detected != value.DetectedMediaType {
		return errors.New("corpus source does not exactly bind to immutable blob metadata")
	}
	return nil
}

func applyConfirmedCorpus(ctx context.Context, tx pgx.Tx, record ledger.Record, value domain.CorpusSource) error {
	raw, err := verifiedBlobBytesTx(ctx, tx, value.BlobID, domain.MaxCorpusSourceBytes)
	if err != nil {
		return err
	}
	passages, err := domain.ExtractCorpus(value.Format, raw)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO crab.corpus_sources(source_id,incident_id,mode,compartment,blob_id,sha256,size_bytes,original_filename,declared_media_type,detected_media_type,title,format,maker,confirmer,confirmed_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) ON CONFLICT(source_id) DO UPDATE SET incident_id=EXCLUDED.incident_id,mode=EXCLUDED.mode,compartment=EXCLUDED.compartment,blob_id=EXCLUDED.blob_id,sha256=EXCLUDED.sha256,size_bytes=EXCLUDED.size_bytes,original_filename=EXCLUDED.original_filename,declared_media_type=EXCLUDED.declared_media_type,detected_media_type=EXCLUDED.detected_media_type,title=EXCLUDED.title,format=EXCLUDED.format,maker=EXCLUDED.maker,confirmer=EXCLUDED.confirmer,confirmed_at=EXCLUDED.confirmed_at`, record.RecordID, record.IncidentID, record.IncidentMode, record.Compartment, value.BlobID, value.SHA256, value.SizeBytes, value.OriginalFilename, value.DeclaredMediaType, value.DetectedMediaType, value.Title, value.Format, record.Maker, record.Confirmer, record.Timestamp); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM crab.corpus_passages WHERE source_id=$1`, record.RecordID); err != nil {
		return err
	}
	for _, passage := range passages {
		if _, err := tx.Exec(ctx, `INSERT INTO crab.corpus_passages(source_id,ordinal,source_sha256,locator,passage,terms) VALUES($1,$2,$3,$4,$5,$6)`, record.RecordID, passage.Ordinal, value.SHA256, passage.Locator, passage.Text, passage.Terms); err != nil {
			return err
		}
	}
	return nil
}

func verifiedBlobBytesTx(ctx context.Context, tx pgx.Tx, blobID string, maxBytes int64) ([]byte, error) {
	var wantSHA string
	var wantSize int64
	if err := tx.QueryRow(ctx, `SELECT sha256,size_bytes FROM crab.blobs WHERE blob_id=$1`, blobID).Scan(&wantSHA, &wantSize); err != nil {
		return nil, err
	}
	if wantSize < 0 || wantSize > maxBytes {
		return nil, ErrBlobTooLarge
	}
	rows, err := tx.Query(ctx, `SELECT chunk_no,bytes FROM crab.blob_chunks WHERE blob_id=$1 ORDER BY chunk_no`, blobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	h := sha256.New()
	var out bytes.Buffer
	out.Grow(int(wantSize))
	expected := 0
	for rows.Next() {
		var chunkNo int
		var raw []byte
		if err := rows.Scan(&chunkNo, &raw); err != nil {
			return nil, err
		}
		if chunkNo != expected || len(raw) == 0 || len(raw) > evidenceChunkBytes {
			return nil, ErrBlobIntegrity
		}
		expected++
		_, _ = h.Write(raw)
		_, _ = out.Write(raw)
		if int64(out.Len()) > maxBytes {
			return nil, ErrBlobTooLarge
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if int64(out.Len()) != wantSize || hex.EncodeToString(h.Sum(nil)) != wantSHA {
		return nil, ErrBlobIntegrity
	}
	return out.Bytes(), nil
}

type CorpusStore struct{ pool *pgxpool.Pool }

func NewCorpusStore(pool *pgxpool.Pool) (*CorpusStore, error) {
	if pool == nil {
		return nil, errors.New("PostgreSQL pool is required")
	}
	return &CorpusStore{pool: pool}, nil
}

func (s *CorpusStore) Candidates(ctx context.Context, incidentID string, compartments []string) ([]domain.CorpusCandidate, error) {
	if incidentID == "" || len(compartments) == 0 {
		return nil, errors.New("retrieval requires incident and authorised compartments")
	}
	rows, err := s.pool.Query(ctx, `SELECT s.source_id,s.title,s.sha256,s.incident_id,s.compartment,p.locator,p.passage,p.terms FROM crab.corpus_sources s JOIN crab.corpus_passages p ON p.source_id=s.source_id WHERE (s.incident_id='' OR s.incident_id=$1) AND s.compartment=ANY($2::text[]) ORDER BY s.source_id,p.ordinal`, incidentID, compartments)
	if err != nil {
		return nil, fmt.Errorf("read authorised corpus candidates: %w", err)
	}
	defer rows.Close()
	var result []domain.CorpusCandidate
	for rows.Next() {
		var candidate domain.CorpusCandidate
		if err := rows.Scan(&candidate.SourceID, &candidate.Title, &candidate.SHA256, &candidate.IncidentID, &candidate.Compartment, &candidate.Locator, &candidate.Passage, &candidate.Terms); err != nil {
			return nil, err
		}
		result = append(result, candidate)
	}
	return result, rows.Err()
}
