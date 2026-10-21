package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

type ContinuityProjection struct{}

func (ContinuityProjection) Reset(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `TRUNCATE crab.incident_resource_requests, crab.incident_handovers, crab.incident_severity_changes, crab.incident_scope_assessments, crab.continuity_proposals, crab.first_report_dispositions, crab.first_reports`)
	return err
}

func (ContinuityProjection) Apply(ctx context.Context, tx pgx.Tx, entry ledger.Entry) error {
	r := entry.Record
	switch r.RecordType {
	case domain.TypeFirstReport:
		if r.IncidentMode != "system" || r.WriteSemantic != "record" {
			return errors.New("first report requires system record semantic")
		}
		v, err := domain.DecodePayload[domain.FirstReport](r)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO crab.first_reports(report_id,payload,maker,recorded_at) VALUES($1,$2,$3,$4) ON CONFLICT (report_id) DO NOTHING`, r.RecordID, payload, r.Maker, r.Timestamp)
		return err
	case domain.TypeFirstReportDisposition:
		if r.IncidentMode != "system" || r.WriteSemantic != "record" {
			return errors.New("first report disposition requires system record semantic")
		}
		v, err := domain.DecodePayload[domain.FirstReportDisposition](r)
		if err != nil {
			return err
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM crab.first_reports WHERE report_id=$1)`, v.ReportID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return errors.New("first report disposition references unknown report")
		}
		payload, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO crab.first_report_dispositions(record_id,report_id,payload,maker,recorded_at) VALUES($1,$2,$3,$4,$5)`, r.RecordID, v.ReportID, payload, r.Maker, r.Timestamp)
		return err
	case domain.TypeScopeAssessment:
		if r.WriteSemantic != "record" {
			return errors.New("scope assessment requires record semantic")
		}
		v, err := domain.DecodePayload[domain.ScopeAssessment](r)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO crab.incident_scope_assessments(record_id,incident_id,compartment,payload,maker,recorded_at) VALUES($1,$2,$3,$4,$5,$6)`, r.RecordID, r.IncidentID, r.Compartment, payload, r.Maker, r.Timestamp)
		return err
	case domain.TypeSeverityChange:
		return applySeverityChange(ctx, tx, r)
	case domain.TypeHandover:
		if r.WriteSemantic != "record" {
			return errors.New("handover requires record semantic")
		}
		v, err := domain.DecodePayload[domain.Handover](r)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO crab.incident_handovers(record_id,incident_id,compartment,payload,maker,recorded_at) VALUES($1,$2,$3,$4,$5,$6)`, r.RecordID, r.IncidentID, r.Compartment, payload, r.Maker, r.Timestamp)
		return err
	case domain.TypeResourceRequest:
		if r.WriteSemantic != "record" {
			return errors.New("resource request requires record semantic")
		}
		v, err := domain.DecodePayload[domain.ResourceRequest](r)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO crab.incident_resource_requests(record_id,incident_id,compartment,payload,maker,recorded_at) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT (record_id) DO UPDATE SET payload=EXCLUDED.payload,maker=EXCLUDED.maker,recorded_at=EXCLUDED.recorded_at`, r.RecordID, r.IncidentID, r.Compartment, payload, r.Maker, r.Timestamp)
		return err
	default:
		return nil
	}
}

func applySeverityChange(ctx context.Context, tx pgx.Tx, r ledger.Record) error {
	if r.WriteSemantic != "governed" {
		return errors.New("severity change requires governed semantic")
	}
	data, err := domain.DecodeGovernedData(r.Data)
	if err != nil {
		return err
	}
	if r.Status == "proposed" {
		if _, err := domain.DecodePayload[domain.SeverityChange](r); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO crab.continuity_proposals(incident_id,record_id,record_type,compartment,version,maker,payload,proposed_at,mode) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT (incident_id,record_type,record_id) DO UPDATE SET compartment=EXCLUDED.compartment,version=EXCLUDED.version,maker=EXCLUDED.maker,payload=EXCLUDED.payload,proposed_at=EXCLUDED.proposed_at,mode=EXCLUDED.mode`, r.IncidentID, r.RecordID, r.RecordType, r.Compartment, data.Version, r.Maker, []byte(data.Payload), r.Timestamp, r.IncidentMode)
		return err
	}
	if r.Status != "confirmed" && r.Status != "rejected" {
		return errors.New("invalid severity decision status")
	}
	var version, maker, compartment, mode string
	err = tx.QueryRow(ctx, `SELECT version,maker,compartment,mode FROM crab.continuity_proposals WHERE incident_id=$1 AND record_type=$2 AND record_id=$3`, r.IncidentID, r.RecordType, r.RecordID).Scan(&version, &maker, &compartment, &mode)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("severity decision has no matching pending proposal")
	}
	if err != nil {
		return err
	}
	// DecodeGovernedData has already verified that data.Version is the SHA-256
	// of the canonical decision payload. Matching the immutable stored proposal
	// version therefore proves payload identity without comparing PostgreSQL
	// jsonb bytes to the ledger's independently canonicalised JSON bytes.
	if version != data.Version || maker != r.Maker || compartment != r.Compartment || mode != r.IncidentMode {
		return errors.New("severity decision does not match exact pending proposal")
	}
	if r.Status == "confirmed" {
		v, err := domain.DecodePayload[domain.SeverityChange](r)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO crab.incident_severity_changes(record_id,incident_id,compartment,payload,maker,confirmer,confirmed_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, r.RecordID, r.IncidentID, r.Compartment, data.Payload, r.Maker, r.Confirmer, r.Timestamp); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE crab.incidents SET severity=$2 WHERE incident_id=$1`, r.IncidentID, v.Severity); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `DELETE FROM crab.continuity_proposals WHERE incident_id=$1 AND record_type=$2 AND record_id=$3`, r.IncidentID, r.RecordType, r.RecordID)
	return err
}

type ContinuityStore struct{ pool *pgxpool.Pool }

func NewContinuityStore(pool *pgxpool.Pool) (*ContinuityStore, error) {
	if pool == nil {
		return nil, errors.New("PostgreSQL pool is required")
	}
	return &ContinuityStore{pool: pool}, nil
}

func (s *ContinuityStore) FirstReports(ctx context.Context) ([]domain.FirstReportView, error) {
	rows, err := s.pool.Query(ctx, `SELECT f.report_id,f.payload,f.maker,f.recorded_at,d.payload,d.recorded_at FROM crab.first_reports f LEFT JOIN LATERAL (SELECT payload,recorded_at FROM crab.first_report_dispositions WHERE report_id=f.report_id ORDER BY recorded_at DESC,record_id DESC LIMIT 1) d ON true ORDER BY f.recorded_at DESC,f.report_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.FirstReportView
	for rows.Next() {
		var v domain.FirstReportView
		var payload []byte
		var dispositionPayload []byte
		var dispositionAt *time.Time
		if err := rows.Scan(&v.ReportID, &payload, &v.Maker, &v.RecordedAt, &dispositionPayload, &dispositionAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(payload, &v.Report); err != nil {
			return nil, err
		}
		if len(dispositionPayload) > 0 {
			var d domain.FirstReportDisposition
			if err := json.Unmarshal(dispositionPayload, &d); err != nil {
				return nil, err
			}
			v.Disposition = &d
			v.DispositionAt = dispositionAt
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *ContinuityStore) ScopeAssessments(ctx context.Context, incidentID, compartment string) ([]domain.ScopeAssessmentView, error) {
	rows, err := s.pool.Query(ctx, `SELECT record_id,compartment,payload,maker,recorded_at FROM crab.incident_scope_assessments WHERE incident_id=$1 AND compartment=$2 ORDER BY recorded_at DESC,record_id`, incidentID, compartment)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ScopeAssessmentView
	for rows.Next() {
		var v domain.ScopeAssessmentView
		var p []byte
		if err := rows.Scan(&v.RecordID, &v.Compartment, &p, &v.Maker, &v.RecordedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(p, &v.Assessment); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *ContinuityStore) SeverityChanges(ctx context.Context, incidentID, compartment string) ([]domain.SeverityChangeView, error) {
	rows, err := s.pool.Query(ctx, `SELECT record_id,compartment,payload,maker,confirmer,confirmed_at FROM crab.incident_severity_changes WHERE incident_id=$1 AND compartment=$2 ORDER BY confirmed_at DESC,record_id`, incidentID, compartment)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.SeverityChangeView
	for rows.Next() {
		var v domain.SeverityChangeView
		var p []byte
		if err := rows.Scan(&v.RecordID, &v.Compartment, &p, &v.Maker, &v.Confirmer, &v.ConfirmedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(p, &v.Change); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *ContinuityStore) Handovers(ctx context.Context, incidentID, compartment string) ([]domain.HandoverView, error) {
	rows, err := s.pool.Query(ctx, `SELECT record_id,compartment,payload,maker,recorded_at FROM crab.incident_handovers WHERE incident_id=$1 AND compartment=$2 ORDER BY recorded_at DESC,record_id`, incidentID, compartment)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.HandoverView
	for rows.Next() {
		var v domain.HandoverView
		var p []byte
		if err := rows.Scan(&v.RecordID, &v.Compartment, &p, &v.Maker, &v.RecordedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(p, &v.Handover); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *ContinuityStore) ResourceRequests(ctx context.Context, incidentID, compartment string) ([]domain.ResourceRequestView, error) {
	rows, err := s.pool.Query(ctx, `SELECT record_id,compartment,payload,maker,recorded_at FROM crab.incident_resource_requests WHERE incident_id=$1 AND compartment=$2 ORDER BY recorded_at DESC,record_id`, incidentID, compartment)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ResourceRequestView
	for rows.Next() {
		var v domain.ResourceRequestView
		var p []byte
		if err := rows.Scan(&v.RecordID, &v.Compartment, &p, &v.Maker, &v.RecordedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(p, &v.Request); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

type ContinuityRuntime struct {
	Kernel *Kernel
	Signer Signer
	Store  *ContinuityStore
	Blobs  *BlobStore
}

func NewContinuityRuntime(kernel *Kernel, signer Signer, store *ContinuityStore, blobs *BlobStore) (*ContinuityRuntime, error) {
	if kernel == nil || store == nil || blobs == nil || signer.KeyID == "" {
		return nil, errors.New("continuity runtime requires kernel, signer, store and blob store")
	}
	return &ContinuityRuntime{Kernel: kernel, Signer: signer, Store: store, Blobs: blobs}, nil
}

func (r *ContinuityRuntime) PutFirstReportBlob(ctx context.Context, filename, mediaType string, src io.Reader) (domain.BlobReceipt, error) {
	if filename == "" || src == nil {
		return domain.BlobReceipt{}, errors.New("blob filename and reader are required")
	}
	blobID, err := opaqueBlobID()
	if err != nil {
		return domain.BlobReceipt{}, err
	}
	tx, err := r.Blobs.pool.Begin(ctx)
	if err != nil {
		return domain.BlobReceipt{}, err
	}
	defer tx.Rollback(ctx)
	h := sha256.New()
	buf := make([]byte, evidenceChunkBytes)
	sample := make([]byte, 0, 512)
	var total int64
	chunkNo := 0
	for {
		if err := ctx.Err(); err != nil {
			return domain.BlobReceipt{}, err
		}
		n, readErr := src.Read(buf)
		if n > 0 {
			if total+int64(n) > r.Blobs.maxBytes {
				return domain.BlobReceipt{}, ErrBlobTooLarge
			}
			piece := buf[:n]
			if len(sample) < 512 {
				need := 512 - len(sample)
				if need > n {
					need = n
				}
				sample = append(sample, piece[:need]...)
			}
			_, _ = h.Write(piece)
			if _, err := tx.Exec(ctx, `INSERT INTO crab.blob_chunks(blob_id,chunk_no,bytes) VALUES($1,$2,$3)`, blobID, chunkNo, piece); err != nil {
				return domain.BlobReceipt{}, err
			}
			total += int64(n)
			chunkNo++
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return domain.BlobReceipt{}, readErr
		}
	}
	digest := hex.EncodeToString(h.Sum(nil))
	detected := http.DetectContentType(sample)
	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, `INSERT INTO crab.blobs(blob_id,incident_id,mode,compartment,sha256,size_bytes,original_filename,declared_media_type,detected_media_type,created_at) VALUES($1,'','system','SYSTEM',$2,$3,$4,$5,$6,$7)`, blobID, digest, total, filename, mediaType, detected, now); err != nil {
		return domain.BlobReceipt{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.BlobReceipt{}, err
	}
	return domain.BlobReceipt{BlobID: blobID, SHA256: digest, SizeBytes: total, OriginalFilename: filename, DeclaredMediaType: mediaType, DetectedMediaType: detected}, nil
}
func (r *ContinuityRuntime) AppendContinuity(ctx context.Context, record ledger.Record) error {
	_, err := r.Kernel.Append(ctx, record, r.Signer, ContinuityProjection{})
	return err
}
func (r *ContinuityRuntime) PendingContinuity(ctx context.Context, incidentID, recordType, recordID string) (ledger.Record, bool, error) {
	var version, maker, compartment, mode string
	var payload []byte
	var proposedAt time.Time
	err := r.Store.pool.QueryRow(ctx, `SELECT version,maker,compartment,mode,payload,proposed_at FROM crab.continuity_proposals WHERE incident_id=$1 AND record_type=$2 AND record_id=$3`, incidentID, recordType, recordID).Scan(&version, &maker, &compartment, &mode, &payload, &proposedAt)
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
func (r *ContinuityRuntime) FirstReports(ctx context.Context) ([]domain.FirstReportView, error) {
	return r.Store.FirstReports(ctx)
}
func (r *ContinuityRuntime) ScopeAssessments(ctx context.Context, i, c string) ([]domain.ScopeAssessmentView, error) {
	return r.Store.ScopeAssessments(ctx, i, c)
}
func (r *ContinuityRuntime) SeverityChanges(ctx context.Context, i, c string) ([]domain.SeverityChangeView, error) {
	return r.Store.SeverityChanges(ctx, i, c)
}
func (r *ContinuityRuntime) Handovers(ctx context.Context, i, c string) ([]domain.HandoverView, error) {
	return r.Store.Handovers(ctx, i, c)
}
func (r *ContinuityRuntime) ResourceRequests(ctx context.Context, i, c string) ([]domain.ResourceRequestView, error) {
	return r.Store.ResourceRequests(ctx, i, c)
}

func continuitySummary(record ledger.Record) string {
	switch record.RecordType {
	case domain.TypeScopeAssessment:
		v, _ := domain.DecodePayload[domain.ScopeAssessment](record)
		return "Scope/impact: " + v.Impact
	case domain.TypeSeverityChange:
		v, _ := domain.DecodePayload[domain.SeverityChange](record)
		return "Severity: " + v.Severity
	case domain.TypeHandover:
		return "Shift handover"
	case domain.TypeResourceRequest:
		v, _ := domain.DecodePayload[domain.ResourceRequest](record)
		return "Resource request: " + v.Request
	default:
		return fmt.Sprintf("%s", record.RecordType)
	}
}
