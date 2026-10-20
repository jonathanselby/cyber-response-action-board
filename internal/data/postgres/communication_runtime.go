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

type CommunicationRuntime struct {
	Kernel *Kernel
	Signer Signer
	Store  *CommunicationStore
}

func NewCommunicationRuntime(kernel *Kernel, signer Signer, store *CommunicationStore) (*CommunicationRuntime, error) {
	if kernel == nil || store == nil || signer.KeyID == "" {
		return nil, errors.New("communication runtime requires kernel, signer and store")
	}
	return &CommunicationRuntime{Kernel: kernel, Signer: signer, Store: store}, nil
}

func (r *CommunicationRuntime) AppendCommunication(ctx context.Context, record ledger.Record) error {
	_, err := r.Kernel.Append(ctx, record, r.Signer, CommunicationProjection{})
	return err
}

func (r *CommunicationRuntime) PendingCommunication(ctx context.Context, incidentID, recordType, recordID string) (ledger.Record, bool, error) {
	var version, maker, compartment, mode string
	var payload []byte
	var proposedAt time.Time
	err := r.Store.pool.QueryRow(ctx, `SELECT version,maker,compartment,mode,payload,proposed_at FROM crab.communication_proposals WHERE incident_id=$1 AND record_type=$2 AND record_id=$3`, incidentID, recordType, recordID).Scan(&version, &maker, &compartment, &mode, &payload, &proposedAt)
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

func (r *CommunicationRuntime) CommunicationDraft(ctx context.Context, incidentID, recordID string) (domain.CommunicationDraftView, bool, error) {
	return r.Store.CommunicationDraft(ctx, incidentID, recordID)
}

func (r *CommunicationRuntime) IssuedCommunication(ctx context.Context, incidentID, recordID string) (domain.CommunicationView, bool, error) {
	return r.Store.IssuedCommunication(ctx, incidentID, recordID)
}

func (r *CommunicationRuntime) CommunicationCadence(ctx context.Context, incidentID, cadenceID string) (domain.CommunicationCadenceView, bool, error) {
	return r.Store.CommunicationCadence(ctx, incidentID, cadenceID)
}
