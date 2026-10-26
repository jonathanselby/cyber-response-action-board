package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/access"
)

type AccessRuntime struct {
	Kernel *Kernel
	Signer Signer
	Store  *AccessStore
}

func NewAccessRuntime(kernel *Kernel, signer Signer, store *AccessStore) (*AccessRuntime, error) {
	if kernel == nil || store == nil || signer.KeyID == "" {
		return nil, errors.New("access runtime requires kernel, signer and store")
	}
	return &AccessRuntime{Kernel: kernel, Signer: signer, Store: store}, nil
}

func (r *AccessRuntime) AppendAccess(ctx context.Context, record ledger.Record) error {
	_, err := r.Kernel.Append(ctx, record, r.Signer, AccessProjection{})
	return err
}

func (r *AccessRuntime) PendingAccess(ctx context.Context, scopeID, recordType, recordID string) (ledger.Record, bool, error) {
	var version, maker, compartment, mode, timeStatus string
	var payload []byte
	var proposedAt time.Time
	err := r.Store.pool.QueryRow(ctx, `SELECT version,maker,compartment,mode,time_status,payload,proposed_at FROM crab.access_proposals WHERE scope_id=$1 AND record_type=$2 AND record_id=$3`, scopeID, recordType, recordID).Scan(&version, &maker, &compartment, &mode, &timeStatus, &payload, &proposedAt)
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
	record := ledger.Record{SchemaVersion: 1, Timestamp: proposedAt.UTC(), TimeStatus: timeStatus, IncidentID: scopeID, IncidentMode: mode, WriteSemantic: "governed", RecordType: recordType, RecordID: recordID, Status: "proposed", Maker: maker, Compartment: compartment, Data: wrapped, MakerSignature: ledger.Signature{MissingReason: "maker authenticated on immutable proposal entry"}}
	if mode == "system" {
		record.IncidentID = ""
	}
	if _, err := domain.ProposalVersion(record); err != nil {
		return ledger.Record{}, false, err
	}
	return record, true, nil
}

func (r *AccessRuntime) IsSponsor(ctx context.Context, humanID, incidentID string) (bool, error) {
	return r.Store.IsSponsor(ctx, humanID, incidentID)
}

func (r *AccessRuntime) HasSponsor(ctx context.Context, incidentID string) (bool, error) {
	return r.Store.HasSponsor(ctx, incidentID)
}

func (r *AccessRuntime) IsController(ctx context.Context, humanID, incidentID, compartment string) (bool, error) {
	return r.Store.IsController(ctx, humanID, incidentID, compartment)
}

func (r *AccessRuntime) IsCustodian(ctx context.Context, humanID string) (bool, error) {
	return r.Store.IsCustodian(ctx, humanID)
}

func (r *AccessRuntime) IsMajorEligible(ctx context.Context, humanID string) (bool, error) {
	return r.Store.IsMajorEligible(ctx, humanID)
}

func (r *AccessRuntime) HasRoleEligibility(ctx context.Context, humanID string) (bool, error) {
	return r.Store.HasRoleEligibility(ctx, humanID)
}

func (r *AccessRuntime) IncidentExists(ctx context.Context, incidentID string) (bool, error) {
	return r.Store.IncidentExists(ctx, incidentID)
}

func (r *AccessRuntime) HasIncidentMembershipAt(ctx context.Context, humanID, incidentID string, now time.Time) (bool, error) {
	return r.Store.HasIncidentMembershipAt(ctx, humanID, incidentID, now)
}

func (r *AccessRuntime) AdministrationView(ctx context.Context, now time.Time) (access.AdministrationView, error) {
	return r.Store.AdministrationView(ctx, now)
}
