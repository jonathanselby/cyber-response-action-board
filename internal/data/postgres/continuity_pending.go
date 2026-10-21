package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

// NewContinuityRuntimeFromKernel reuses the verified operator kernel, its
// existing PostgreSQL pool and the already-configured hostile-byte boundary.
func NewContinuityRuntimeFromKernel(kernel *Kernel, signer Signer, blobs *BlobStore) (*ContinuityRuntime, error) {
	if kernel == nil || blobs == nil {
		return nil, errors.New("continuity kernel and blob store are required")
	}
	store, err := NewContinuityStore(kernel.pool)
	if err != nil {
		return nil, err
	}
	return NewContinuityRuntime(kernel, signer, store, blobs)
}

// PendingSeverityChange reconstructs one proposal without normalising the JSONB
// payload bytes. The decision projection deliberately requires an exact match
// to the immutable pending proposal.
func (r *ContinuityRuntime) PendingSeverityChange(ctx context.Context, incidentID, recordID string) (ledger.Record, bool, error) {
	if r == nil || r.Store == nil {
		return ledger.Record{}, false, errors.New("continuity runtime is required")
	}
	var version, maker, compartment, mode string
	var payload []byte
	var proposedAt time.Time
	err := r.Store.pool.QueryRow(ctx, `SELECT version,maker,compartment,mode,payload,proposed_at FROM crab.continuity_proposals WHERE incident_id=$1 AND record_type=$2 AND record_id=$3`, strings.TrimSpace(incidentID), domain.TypeSeverityChange, strings.TrimSpace(recordID)).Scan(&version, &maker, &compartment, &mode, &payload, &proposedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ledger.Record{}, false, nil
	}
	if err != nil {
		return ledger.Record{}, false, err
	}
	record, err := pendingSeverityRecord(incidentID, recordID, version, maker, compartment, mode, payload, proposedAt)
	if err != nil {
		return ledger.Record{}, false, err
	}
	return record, true, nil
}

// PendingSeverityChanges reconstructs the immutable proposals required by the
// two-person severity decision path. It is deliberately read-only; decisions
// still flow through ContinuityService.DecideSeverity and the ledger.
func (r *ContinuityRuntime) PendingSeverityChanges(ctx context.Context, incidentID, compartment string) ([]ledger.Record, error) {
	if r == nil || r.Store == nil {
		return nil, errors.New("continuity runtime is required")
	}
	incidentID = strings.TrimSpace(incidentID)
	compartment = strings.TrimSpace(compartment)
	if incidentID == "" || compartment == "" {
		return nil, errors.New("incident id and compartment are required")
	}
	rows, err := r.Store.pool.Query(ctx, `SELECT record_id,version,maker,payload,proposed_at,mode FROM crab.continuity_proposals WHERE incident_id=$1 AND compartment=$2 AND record_type=$3 ORDER BY proposed_at DESC,record_id`, incidentID, compartment, domain.TypeSeverityChange)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ledger.Record
	for rows.Next() {
		var recordID, version, maker, mode string
		var payload []byte
		var proposedAt time.Time
		if err := rows.Scan(&recordID, &version, &maker, &payload, &proposedAt, &mode); err != nil {
			return nil, err
		}
		record, err := pendingSeverityRecord(incidentID, recordID, version, maker, compartment, mode, payload, proposedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

func pendingSeverityRecord(incidentID, recordID, version, maker, compartment, mode string, payload []byte, proposedAt time.Time) (ledger.Record, error) {
	encodedVersion, err := json.Marshal(version)
	if err != nil {
		return ledger.Record{}, err
	}
	wrapped := make([]byte, 0, len(payload)+len(encodedVersion)+24)
	wrapped = append(wrapped, `{"version":`...)
	wrapped = append(wrapped, encodedVersion...)
	wrapped = append(wrapped, `,"payload":`...)
	wrapped = append(wrapped, payload...)
	wrapped = append(wrapped, '}')
	if !json.Valid(wrapped) {
		return ledger.Record{}, errors.New("invalid pending severity proposal payload")
	}
	record := ledger.Record{
		SchemaVersion:  1,
		Timestamp:      proposedAt.UTC(),
		TimeStatus:     "trusted",
		IncidentID:     strings.TrimSpace(incidentID),
		IncidentMode:   mode,
		WriteSemantic:  "governed",
		RecordType:     domain.TypeSeverityChange,
		RecordID:       strings.TrimSpace(recordID),
		Status:         "proposed",
		Maker:          maker,
		Compartment:    compartment,
		Data:           wrapped,
		MakerSignature: ledger.Signature{MissingReason: "maker authenticated on immutable proposal entry"},
	}
	if err := ledger.ValidateRecord(record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}
