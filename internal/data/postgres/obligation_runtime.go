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

type ObligationRuntime struct {
	Kernel *Kernel
	Signer Signer
	Store  *ObligationStore
}

func NewObligationRuntime(kernel *Kernel, signer Signer, store *ObligationStore) (*ObligationRuntime, error) {
	if kernel == nil || store == nil || signer.KeyID == "" {
		return nil, errors.New("obligation runtime requires kernel, signer and store")
	}
	return &ObligationRuntime{Kernel: kernel, Signer: signer, Store: store}, nil
}

func (r *ObligationRuntime) AppendObligation(ctx context.Context, record ledger.Record) error {
	_, err := r.Kernel.Append(ctx, record, r.Signer, ObligationProjection{})
	return err
}

func (r *ObligationRuntime) PendingObligation(ctx context.Context, scopeID, recordType, recordID string) (ledger.Record, bool, error) {
	var version, maker, compartment, mode string
	var payload []byte
	var proposedAt time.Time
	err := r.Store.pool.QueryRow(ctx, `SELECT version,maker,compartment,mode,payload,proposed_at FROM crab.obligation_proposals WHERE scope_id=$1 AND record_type=$2 AND record_id=$3`, scopeID, recordType, recordID).Scan(&version, &maker, &compartment, &mode, &payload, &proposedAt)
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
	record := ledger.Record{SchemaVersion: 1, Timestamp: proposedAt.UTC(), TimeStatus: "trusted", IncidentID: scopeID, IncidentMode: mode, WriteSemantic: "governed", RecordType: recordType, RecordID: recordID, Status: "proposed", Maker: maker, Compartment: compartment, Data: wrapped, MakerSignature: ledger.Signature{MissingReason: "maker authenticated on immutable proposal entry"}}
	if mode == "system" {
		record.IncidentID = ""
	}
	if _, err := domain.ProposalVersion(record); err != nil {
		return ledger.Record{}, false, err
	}
	return record, true, nil
}

func (r *ObligationRuntime) ObligationClocks(ctx context.Context, incidentID, compartment string, now time.Time) ([]domain.ObligationClockView, error) {
	return r.Store.ClocksInCompartment(ctx, incidentID, compartment, now)
}
