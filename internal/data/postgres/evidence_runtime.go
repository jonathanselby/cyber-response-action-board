package postgres

import (
	"context"
	"errors"
	"io"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

type EvidenceRuntime struct {
	Kernel *Kernel
	Signer Signer
	Blobs  *BlobStore
	Store  *EvidenceStore
}

func NewEvidenceRuntime(kernel *Kernel, signer Signer, blobs *BlobStore, store *EvidenceStore) (*EvidenceRuntime, error) {
	if kernel == nil || signer.KeyID == "" || blobs == nil || store == nil {
		return nil, errors.New("evidence runtime requires kernel, signer, blob store and evidence store")
	}
	return &EvidenceRuntime{Kernel: kernel, Signer: signer, Blobs: blobs, Store: store}, nil
}

func (r *EvidenceRuntime) PutEvidenceBlob(ctx context.Context, scope domain.Scope, filename, declaredMediaType string, src io.Reader) (domain.BlobReceipt, error) {
	return r.Blobs.Put(ctx, scope, filename, declaredMediaType, src)
}

func (r *EvidenceRuntime) AppendEvidence(ctx context.Context, record ledger.Record) error {
	_, err := r.Kernel.Append(ctx, record, r.Signer, EvidenceProjection{})
	return err
}

func (r *EvidenceRuntime) Evidence(ctx context.Context, evidenceID string) (domain.EvidenceView, bool, error) {
	return r.Store.Evidence(ctx, evidenceID)
}

func (r *EvidenceRuntime) EvidenceHistory(ctx context.Context, evidenceID string) ([]domain.EvidenceHandlingView, error) {
	return r.Store.History(ctx, evidenceID)
}

func (r *EvidenceRuntime) EvidenceRegister(ctx context.Context, incidentID, compartment string) ([]domain.EvidenceView, error) {
	return r.Store.Register(ctx, incidentID, compartment)
}

func (r *EvidenceRuntime) VerifyEvidenceBlob(ctx context.Context, blobID string) (domain.BlobReceipt, error) {
	return r.Blobs.Verify(ctx, blobID)
}

func (r *EvidenceRuntime) CopyVerifiedEvidenceBlob(ctx context.Context, blobID string, dst io.Writer) (domain.BlobReceipt, error) {
	return r.Blobs.CopyVerified(ctx, blobID, dst)
}
