package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

func (k *Kernel) AppendBatch(ctx context.Context, records []ledger.Record, signer Signer, projections ...Projection) ([]ledger.Entry, error) {
	if len(records) == 0 {
		return nil, errors.New("ledger batch is empty")
	}
	if k.failed.Load() {
		return nil, ErrIntegrityFailed
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.failed.Load() {
		return nil, ErrIntegrityFailed
	}

	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin ledger batch: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := lockLedger(ctx, tx); err != nil {
		return nil, err
	}
	seq, hash, err := readTail(ctx, tx)
	if err != nil {
		if errors.Is(err, ErrUnexpectedTail) {
			k.failed.Store(true)
		}
		return nil, err
	}
	if seq != k.tailSeq || hash != k.tailHash {
		k.failed.Store(true)
		return nil, ErrUnexpectedTail
	}

	entries := make([]ledger.Entry, 0, len(records))
	for _, record := range records {
		entry, err := ledger.BuildEntry(seq+1, hash, record, signer.KeyID, signer.PrivateKey, k.keys)
		if err != nil {
			return nil, fmt.Errorf("build ledger batch entry: %w", err)
		}
		raw, err := json.Marshal(entry)
		if err != nil {
			return nil, fmt.Errorf("marshal ledger batch entry: %w", err)
		}
		if len(raw) == 0 || len(raw) > ledger.MaxEntryBytes {
			return nil, fmt.Errorf("serialized ledger entry exceeds %d bytes", ledger.MaxEntryBytes)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO crab.ledger_entries(seq, prev_hash, hash, entry_bytes) VALUES ($1,$2,$3,$4)`, int64(entry.Seq), entry.PrevHash, entry.Hash, raw); err != nil {
			return nil, fmt.Errorf("insert ledger batch entry: %w", err)
		}
		for _, projection := range projections {
			if projection != nil {
				if err := projection.Apply(ctx, tx, entry); err != nil {
					return nil, fmt.Errorf("apply batch projection: %w", err)
				}
			}
		}
		entries = append(entries, entry)
		seq, hash = entry.Seq, entry.Hash
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit ledger batch: %w", err)
	}
	k.tailSeq, k.tailHash = seq, hash
	return entries, nil
}

func (s *AccessStore) Bootstrapped(ctx context.Context) (bool, error) {
	var roles, custodians int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM crab.role_eligibility WHERE role='Major'`).Scan(&roles); err != nil {
		return false, err
	}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM crab.custodian_designations`).Scan(&custodians); err != nil {
		return false, err
	}
	return roles >= 2 && custodians >= 2, nil
}
