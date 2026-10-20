package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

const evidenceChunkBytes = 64 * 1024

var (
	ErrBlobTooLarge  = errors.New("blob exceeds configured maximum")
	ErrBlobNotFound  = errors.New("blob not found")
	ErrBlobIntegrity = errors.New("blob integrity verification failed")
)

type BlobStore struct {
	pool     *pgxpool.Pool
	maxBytes int64
}

func NewBlobStore(pool *pgxpool.Pool, maxBytes int64) (*BlobStore, error) {
	if pool == nil {
		return nil, errors.New("PostgreSQL pool is required")
	}
	if maxBytes <= 0 {
		return nil, errors.New("blob maximum must be positive")
	}
	return &BlobStore{pool: pool, maxBytes: maxBytes}, nil
}

func (s *BlobStore) Put(ctx context.Context, scope domain.Scope, filename, declaredMediaType string, r io.Reader) (domain.BlobReceipt, error) {
	if strings.TrimSpace(scope.IncidentID) == "" || (scope.Mode != "live" && scope.Mode != "exercise") || strings.TrimSpace(scope.Compartment) == "" {
		return domain.BlobReceipt{}, errors.New("blob requires live/exercise incident scope")
	}
	if strings.TrimSpace(filename) == "" || r == nil {
		return domain.BlobReceipt{}, errors.New("blob filename and reader are required")
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
			if total+int64(n) > s.maxBytes {
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
			if _, err := h.Write(piece); err != nil {
				return domain.BlobReceipt{}, err
			}
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
		if n == 0 {
			continue
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

func (s *BlobStore) Verify(ctx context.Context, blobID string) (domain.BlobReceipt, error) {
	receipt, err := s.receipt(ctx, blobID)
	if err != nil {
		return domain.BlobReceipt{}, err
	}
	rows, err := s.pool.Query(ctx, `SELECT chunk_no,bytes FROM crab.blob_chunks WHERE blob_id=$1 ORDER BY chunk_no`, blobID)
	if err != nil {
		return domain.BlobReceipt{}, err
	}
	defer rows.Close()
	h := sha256.New()
	var total int64
	expected := 0
	for rows.Next() {
		var chunkNo int
		var raw []byte
		if err := rows.Scan(&chunkNo, &raw); err != nil {
			return domain.BlobReceipt{}, err
		}
		if chunkNo != expected || len(raw) == 0 || len(raw) > evidenceChunkBytes {
			return domain.BlobReceipt{}, ErrBlobIntegrity
		}
		expected++
		total += int64(len(raw))
		_, _ = h.Write(raw)
	}
	if err := rows.Err(); err != nil {
		return domain.BlobReceipt{}, err
	}
	if total != receipt.SizeBytes || hex.EncodeToString(h.Sum(nil)) != receipt.SHA256 {
		return domain.BlobReceipt{}, ErrBlobIntegrity
	}
	return receipt, nil
}

func (s *BlobStore) CopyVerified(ctx context.Context, blobID string, w io.Writer) (domain.BlobReceipt, error) {
	if w == nil {
		return domain.BlobReceipt{}, errors.New("writer is required")
	}
	receipt, err := s.Verify(ctx, blobID)
	if err != nil {
		return domain.BlobReceipt{}, err
	}
	rows, err := s.pool.Query(ctx, `SELECT bytes FROM crab.blob_chunks WHERE blob_id=$1 ORDER BY chunk_no`, blobID)
	if err != nil {
		return domain.BlobReceipt{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return domain.BlobReceipt{}, err
		}
		if _, err := w.Write(raw); err != nil {
			return domain.BlobReceipt{}, err
		}
	}
	if err := rows.Err(); err != nil {
		return domain.BlobReceipt{}, err
	}
	return receipt, nil
}

func (s *BlobStore) receipt(ctx context.Context, blobID string) (domain.BlobReceipt, error) {
	var receipt domain.BlobReceipt
	err := s.pool.QueryRow(ctx, `SELECT blob_id,sha256,size_bytes,original_filename,declared_media_type,detected_media_type FROM crab.blobs WHERE blob_id=$1`, blobID).Scan(&receipt.BlobID, &receipt.SHA256, &receipt.SizeBytes, &receipt.OriginalFilename, &receipt.DeclaredMediaType, &receipt.DetectedMediaType)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.BlobReceipt{}, ErrBlobNotFound
	}
	if err != nil {
		return domain.BlobReceipt{}, fmt.Errorf("read blob metadata: %w", err)
	}
	return receipt, nil
}

func opaqueBlobID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "blob_" + hex.EncodeToString(raw), nil
}
