package postgres

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

func (s *BlobStore) PutCorpus(ctx context.Context, scope domain.CorpusScope, filename, declaredMediaType string, r io.Reader) (domain.BlobReceipt, error) {
	if err := domain.ValidateCorpusScope(scope); err != nil {
		return domain.BlobReceipt{}, err
	}
	if strings.TrimSpace(filename) == "" || r == nil {
		return domain.BlobReceipt{}, errors.New("corpus filename and reader are required")
	}
	maxBytes := s.maxBytes
	if maxBytes > domain.MaxCorpusSourceBytes {
		maxBytes = domain.MaxCorpusSourceBytes
	}
	blobID, err := opaqueBlobID()
	if err != nil {
		return domain.BlobReceipt{}, err
	}
	tx, err := s.pool.Begin(ctx)
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
		n, readErr := r.Read(buf)
		if n > 0 {
			if total+int64(n) > maxBytes {
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
	if _, err := tx.Exec(ctx, `INSERT INTO crab.blobs(blob_id,incident_id,mode,compartment,sha256,size_bytes,original_filename,declared_media_type,detected_media_type,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, blobID, scope.IncidentID, scope.Mode, scope.Compartment, digest, total, filename, declaredMediaType, detected, now); err != nil {
		return domain.BlobReceipt{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.BlobReceipt{}, err
	}
	return domain.BlobReceipt{BlobID: blobID, SHA256: digest, SizeBytes: total, OriginalFilename: filename, DeclaredMediaType: declaredMediaType, DetectedMediaType: detected}, nil
}

func (s *BlobStore) ReadVerifiedCorpusBytes(ctx context.Context, blobID string) (domain.BlobReceipt, []byte, error) {
	receipt, err := s.Verify(ctx, blobID)
	if err != nil {
		return domain.BlobReceipt{}, nil, err
	}
	if receipt.SizeBytes > domain.MaxCorpusSourceBytes || receipt.SizeBytes > s.maxBytes {
		return domain.BlobReceipt{}, nil, ErrBlobTooLarge
	}
	var out bytes.Buffer
	out.Grow(int(receipt.SizeBytes))
	if _, err := s.CopyVerified(ctx, blobID, &out); err != nil {
		return domain.BlobReceipt{}, nil, err
	}
	return receipt, out.Bytes(), nil
}
