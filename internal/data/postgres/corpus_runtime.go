package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

type CorpusRuntime struct {
	Kernel *Kernel
	Signer Signer
	Blobs  *BlobStore
	Store  *CorpusStore
}

func NewCorpusRuntime(kernel *Kernel, signer Signer, blobs *BlobStore, store *CorpusStore) (*CorpusRuntime, error) {
	if kernel == nil || signer.KeyID == "" || blobs == nil || store == nil {
		return nil, errors.New("corpus runtime requires kernel, signer, blob store and corpus store")
	}
	return &CorpusRuntime{Kernel: kernel, Signer: signer, Blobs: blobs, Store: store}, nil
}

func (r *CorpusRuntime) PutCorpusBlob(ctx context.Context, scope domain.CorpusScope, filename, declared string, src io.Reader) (domain.BlobReceipt, error) {
	return r.Blobs.PutCorpus(ctx, scope, filename, declared, src)
}

func (r *CorpusRuntime) ValidateCorpusBlob(ctx context.Context, blobID, format string) error {
	_, raw, err := r.Blobs.ReadVerifiedCorpusBytes(ctx, blobID)
	if err != nil {
		return err
	}
	_, err = domain.ExtractCorpus(format, raw)
	return err
}

func (r *CorpusRuntime) AppendCorpus(ctx context.Context, record ledger.Record) error {
	_, err := r.Kernel.Append(ctx, record, r.Signer, CorpusProjection{})
	return err
}

func (r *CorpusRuntime) PendingCorpusSource(ctx context.Context, sourceID string) (ledger.Record, bool, error) {
	var incidentID, mode, compartment, version, maker, timeStatus string
	var payload []byte
	var proposedAt time.Time
	err := r.Store.pool.QueryRow(ctx, `SELECT incident_id,mode,compartment,version,maker,time_status,payload,proposed_at FROM crab.corpus_proposals WHERE source_id=$1`, sourceID).Scan(&incidentID, &mode, &compartment, &version, &maker, &timeStatus, &payload, &proposedAt)
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
	record := ledger.Record{SchemaVersion: 1, Timestamp: proposedAt.UTC(), TimeStatus: timeStatus, IncidentID: incidentID, IncidentMode: mode, WriteSemantic: "governed", RecordType: domain.TypeCorpusSource, RecordID: sourceID, Status: "proposed", Maker: maker, Compartment: compartment, Data: wrapped, MakerSignature: ledger.Signature{MissingReason: "maker authenticated on immutable proposal entry"}}
	if _, err := domain.ProposalVersion(record); err != nil {
		return ledger.Record{}, false, err
	}
	return record, true, nil
}

func (r *CorpusRuntime) CorpusCandidates(ctx context.Context, incidentID string, compartments []string) ([]domain.CorpusCandidate, error) {
	return r.Store.Candidates(ctx, incidentID, compartments)
}
