package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

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
		wrapped, err := json.Marshal(domain.GovernedData{Version: version, Payload: json.RawMessage(payload)})
		if err != nil {
			return nil, err
		}
		record := ledger.Record{
			SchemaVersion:  1,
			Timestamp:      proposedAt.UTC(),
			TimeStatus:     "trusted",
			IncidentID:     incidentID,
			IncidentMode:   mode,
			WriteSemantic:  "governed",
			RecordType:     domain.TypeSeverityChange,
			RecordID:       recordID,
			Status:         "proposed",
			Maker:          maker,
			Compartment:    compartment,
			Data:           wrapped,
			MakerSignature: ledger.Signature{MissingReason: "maker authenticated on immutable proposal entry"},
		}
		if err := ledger.ValidateRecord(record); err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}
