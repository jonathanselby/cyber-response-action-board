package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

type IncidentRuntime struct {
	Kernel *Kernel
	Signer Signer
	Store  *IncidentStore
}

func NewIncidentRuntime(kernel *Kernel, signer Signer, store *IncidentStore) (*IncidentRuntime, error) {
	if kernel == nil || store == nil || signer.KeyID == "" {
		return nil, errors.New("incident runtime requires kernel, signer and store")
	}
	return &IncidentRuntime{Kernel: kernel, Signer: signer, Store: store}, nil
}

func (r *IncidentRuntime) AppendIncident(ctx context.Context, record ledger.Record) error {
	_, err := r.Kernel.Append(ctx, record, r.Signer, IncidentProjection{})
	return err
}

func (r *IncidentRuntime) PendingProposalRecord(ctx context.Context, incidentID, recordType, recordID string) (ledger.Record, bool, error) {
	var version, maker, compartment, mode string
	var payload []byte
	var proposedAt time.Time
	err := r.Store.pool.QueryRow(ctx, `SELECT version,maker,compartment,mode,payload,proposed_at FROM crab.incident_proposals WHERE incident_id=$1 AND record_type=$2 AND record_id=$3`, incidentID, recordType, recordID).Scan(&version, &maker, &compartment, &mode, &payload, &proposedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ledger.Record{}, false, nil
	}
	if err != nil {
		return ledger.Record{}, false, err
	}
	if mode != "live" && mode != "exercise" {
		return ledger.Record{}, false, errors.New("pending proposal requires projection rebuild to restore incident mode")
	}
	wrapped, err := json.Marshal(domain.GovernedData{Version: version, Payload: json.RawMessage(payload)})
	if err != nil {
		return ledger.Record{}, false, err
	}
	record := ledger.Record{
		SchemaVersion:  1,
		Timestamp:      proposedAt.UTC(),
		TimeStatus:     "trusted",
		IncidentID:     incidentID,
		IncidentMode:   mode,
		WriteSemantic:  "governed",
		RecordType:     recordType,
		RecordID:       recordID,
		Status:         "proposed",
		Maker:          maker,
		Compartment:    compartment,
		Data:           wrapped,
		MakerSignature: ledger.Signature{MissingReason: "maker authenticated on immutable proposal entry"},
	}
	if _, err := domain.ProposalVersion(record); err != nil {
		return ledger.Record{}, false, err
	}
	return record, true, nil
}

func (r *IncidentRuntime) CommonOperatingPicture(ctx context.Context, incidentID string) (domain.OperatingPicture, bool, error) {
	return r.Store.CommonOperatingPicture(ctx, incidentID)
}

func (r *IncidentRuntime) ObligationClocks(ctx context.Context, incidentID, compartment string, now time.Time) ([]domain.ObligationClockView, error) {
	store, err := NewObligationStore(r.Store.pool)
	if err != nil {
		return nil, err
	}
	return store.ActiveClocksInCompartment(ctx, incidentID, compartment, now)
}

func (r *IncidentRuntime) DeclarationExists(ctx context.Context, incidentID string) (bool, error) {
	var exists bool
	err := r.Store.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM crab.incidents WHERE incident_id=$1) OR EXISTS(SELECT 1 FROM crab.incident_proposals WHERE incident_id=$1 AND record_type=$2)`, incidentID, domain.TypeIncidentDeclaration).Scan(&exists)
	return exists, err
}
