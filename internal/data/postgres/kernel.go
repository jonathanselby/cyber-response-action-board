package postgres

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanselby/cyber-response-action-board/internal/adapters/clock"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

const ledgerLockKey = int64(0x435241424c454447) // Stable transaction-scoped lock for the single CRAB ledger tail.

var (
	ErrIntegrityFailed = errors.New("ledger integrity failed; governed mutation refused")
	ErrUnexpectedTail  = errors.New("ledger tail changed since full verification")
)

// Projection is the persistence hook used by the ledger transaction kernel.
// Implementations are deterministic and rebuildable. They must not invent
// business/security policy; those decisions remain in application/domain code.
type Projection interface {
	Reset(context.Context, pgx.Tx) error
	Apply(context.Context, pgx.Tx, ledger.Entry) error
}

type Signer struct {
	KeyID      string
	PrivateKey ed25519.PrivateKey
}

// Kernel owns the atomic ledger/projection transaction and integrity gate.
type Kernel struct {
	pool *pgxpool.Pool
	keys ledger.Keyring

	mu       sync.Mutex
	tailSeq  uint64
	tailHash string
	failed   atomic.Bool
	initial  ledger.VerifyResult
	clock    clock.Clock
}

// OpenKernel validates the runtime database boundary and performs the required
// full ledger verification before any mutation can be enabled.
func OpenKernel(ctx context.Context, pool *pgxpool.Pool, keys ledger.Keyring) (*Kernel, error) {
	if pool == nil {
		return nil, errors.New("PostgreSQL pool is required")
	}
	if err := ValidateSchema(ctx, pool); err != nil {
		return nil, err
	}
	if err := ValidateRuntimeRole(ctx, pool); err != nil {
		return nil, err
	}
	k := &Kernel{pool: pool, keys: keys, clock: clock.NewHost()}
	res, seq, hash, err := k.verifyDatabase(ctx)
	if err != nil {
		return nil, err
	}
	k.initial = res
	if !res.Valid {
		k.failed.Store(true)
		return k, nil
	}
	k.tailSeq, k.tailHash = seq, hash
	return k, nil
}

func (k *Kernel) InitialVerification() ledger.VerifyResult { return k.initial }

// SetClock replaces the time authority. Tests and replays use a fixed clock.
func (k *Kernel) SetClock(c clock.Clock) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if c != nil {
		k.clock = c
	}
}

// Clock returns the kernel's time authority so composition can show the same
// reading that writes are stamped with.
func (k *Kernel) Clock() clock.Clock {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.clock == nil {
		return clock.NewHost()
	}
	return k.clock
}

// stampTime caps a record's time status by the one clock (T-194). Records
// carrying a human cryptographic signature were stamped before signing and
// are left byte-identical.
func (k *Kernel) stampTime(record ledger.Record) ledger.Record {
	if record.MakerSignature.Value != "" || record.ConfirmerSignature.Value != "" {
		return record
	}
	c := k.clock
	if c == nil {
		c = clock.NewHost()
	}
	record.TimeStatus = ledger.WeakerTimeStatus(record.TimeStatus, c.Read().Status)
	return record
}
func (k *Kernel) IntegrityFailed() bool { return k.failed.Load() }

// Verify performs a full independent verification. A failure is sticky and
// places the process in integrity-failed read-only mode. Verification alone
// never silently re-enables mutation; Rebuild is the explicit recovery action.
func (k *Kernel) Verify(ctx context.Context) (ledger.VerifyResult, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	res, _, _, err := k.verifyDatabase(ctx)
	if err != nil {
		return ledger.VerifyResult{}, err
	}
	if !res.Valid {
		k.failed.Store(true)
	}
	return res, nil
}

// Append atomically appends one signed ledger entry and applies every affected
// projection in the same PostgreSQL transaction.
func (k *Kernel) Append(ctx context.Context, record ledger.Record, signer Signer, projections ...Projection) (ledger.Entry, error) {
	if k.failed.Load() {
		return ledger.Entry{}, ErrIntegrityFailed
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.failed.Load() {
		return ledger.Entry{}, ErrIntegrityFailed
	}

	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return ledger.Entry{}, fmt.Errorf("begin ledger append: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := lockLedger(ctx, tx); err != nil {
		return ledger.Entry{}, err
	}
	seq, hash, err := readTail(ctx, tx)
	if err != nil {
		if errors.Is(err, ErrUnexpectedTail) {
			k.failed.Store(true)
		}
		return ledger.Entry{}, err
	}
	if seq != k.tailSeq || hash != k.tailHash {
		k.failed.Store(true)
		return ledger.Entry{}, ErrUnexpectedTail
	}

	entry, err := ledger.BuildEntry(seq+1, hash, k.stampTime(record), signer.KeyID, signer.PrivateKey, k.keys)
	if err != nil {
		return ledger.Entry{}, fmt.Errorf("build ledger entry: %w", err)
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return ledger.Entry{}, fmt.Errorf("marshal ledger entry: %w", err)
	}
	if len(raw) == 0 || len(raw) > ledger.MaxEntryBytes {
		return ledger.Entry{}, fmt.Errorf("serialized ledger entry exceeds %d bytes", ledger.MaxEntryBytes)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO crab.ledger_entries(seq, prev_hash, hash, entry_bytes)
		VALUES ($1, $2, $3, $4)`, int64(entry.Seq), entry.PrevHash, entry.Hash, raw); err != nil {
		return ledger.Entry{}, fmt.Errorf("insert ledger entry: %w", err)
	}
	for _, projection := range projections {
		if projection == nil {
			continue
		}
		if err := projection.Apply(ctx, tx, entry); err != nil {
			return ledger.Entry{}, fmt.Errorf("apply projection: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return ledger.Entry{}, fmt.Errorf("commit ledger append: %w", err)
	}
	k.tailSeq, k.tailHash = entry.Seq, entry.Hash
	return entry, nil
}

// Rebuild is the explicit verified recovery path for rebuildable projections.
// It never edits ledger history. A successful full replay clears a prior local
// integrity-failed gate because the operator has explicitly chosen recovery.
func (k *Kernel) Rebuild(ctx context.Context, projections ...Projection) (ledger.VerifyResult, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return ledger.VerifyResult{}, fmt.Errorf("begin rebuild: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := lockLedger(ctx, tx); err != nil {
		return ledger.VerifyResult{}, err
	}
	res, seq, hash, err := k.verifyTx(ctx, tx)
	if err != nil {
		return ledger.VerifyResult{}, err
	}
	if !res.Valid {
		k.failed.Store(true)
		return res, ErrIntegrityFailed
	}
	for _, projection := range projections {
		if projection != nil {
			if err := projection.Reset(ctx, tx); err != nil {
				return res, fmt.Errorf("reset projection: %w", err)
			}
		}
	}
	// Read one verified sequence at a time so projection SQL can use the same
	// transaction without an open pgx Rows result making the connection busy.
	// This keeps replay memory bounded without introducing another queue/buffer.
	for replaySeq := uint64(1); replaySeq <= seq; replaySeq++ {
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT entry_bytes FROM crab.ledger_entries WHERE seq = $1`, int64(replaySeq)).Scan(&raw); err != nil {
			return res, fmt.Errorf("read ledger replay seq %d: %w", replaySeq, err)
		}
		var entry ledger.Entry
		if err := json.Unmarshal(raw, &entry); err != nil {
			return res, fmt.Errorf("decode verified ledger entry seq %d: %w", replaySeq, err)
		}
		for _, projection := range projections {
			if projection != nil {
				if err := projection.Apply(ctx, tx, entry); err != nil {
					return res, fmt.Errorf("replay projection: %w", err)
				}
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return res, fmt.Errorf("commit rebuild: %w", err)
	}
	k.tailSeq, k.tailHash = seq, hash
	k.failed.Store(false)
	return res, nil
}

// Export writes the exact authoritative entry bytes as portable JSONL under
// the same ledger lock used by append. Verification is always performed first.
// An integrity failure is returned in the VerifyResult and makes mutation sticky
// read-only, but does not prevent an operator from exporting the existing bytes.
func (k *Kernel) Export(ctx context.Context, w io.Writer) (ledger.VerifyResult, error) {
	if w == nil {
		return ledger.VerifyResult{}, errors.New("export writer is required")
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return ledger.VerifyResult{}, fmt.Errorf("begin ledger export: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := lockLedger(ctx, tx); err != nil {
		return ledger.VerifyResult{}, err
	}
	res, _, _, err := k.verifyTx(ctx, tx)
	if err != nil {
		return ledger.VerifyResult{}, err
	}
	if !res.Valid {
		k.failed.Store(true)
	}
	rows, err := tx.Query(ctx, `SELECT entry_bytes FROM crab.ledger_entries ORDER BY seq`)
	if err != nil {
		return res, fmt.Errorf("read ledger export: %w", err)
	}
	defer rows.Close()
	bw := bufio.NewWriter(w)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return res, fmt.Errorf("scan ledger export: %w", err)
		}
		if _, err := bw.Write(raw); err != nil {
			return res, err
		}
		if err := bw.WriteByte('\n'); err != nil {
			return res, err
		}
	}
	if err := rows.Err(); err != nil {
		return res, fmt.Errorf("read ledger export: %w", err)
	}
	if err := bw.Flush(); err != nil {
		return res, err
	}
	if err := tx.Commit(ctx); err != nil {
		return res, fmt.Errorf("finish ledger export: %w", err)
	}
	return res, nil
}

func (k *Kernel) verifyDatabase(ctx context.Context) (ledger.VerifyResult, uint64, string, error) {
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return ledger.VerifyResult{}, 0, "", fmt.Errorf("begin ledger verification: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := lockLedger(ctx, tx); err != nil {
		return ledger.VerifyResult{}, 0, "", err
	}
	res, seq, hash, err := k.verifyTx(ctx, tx)
	if err != nil {
		return ledger.VerifyResult{}, 0, "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return ledger.VerifyResult{}, 0, "", fmt.Errorf("finish ledger verification: %w", err)
	}
	return res, seq, hash, nil
}

type streamResult struct {
	seq    uint64
	hash   string
	errors []string
	err    error
}

func (k *Kernel) verifyTx(ctx context.Context, tx pgx.Tx) (ledger.VerifyResult, uint64, string, error) {
	rows, err := tx.Query(ctx, `SELECT seq, prev_hash, hash, entry_bytes FROM crab.ledger_entries ORDER BY seq`)
	if err != nil {
		return ledger.VerifyResult{}, 0, "", fmt.Errorf("read ledger verification: %w", err)
	}
	pr, pw := io.Pipe()
	resultCh := make(chan streamResult, 1)
	go func() {
		defer rows.Close()
		sr := streamResult{}
		for rows.Next() {
			var dbSeq int64
			var dbPrev, dbHash string
			var raw []byte
			if err := rows.Scan(&dbSeq, &dbPrev, &dbHash, &raw); err != nil {
				sr.err = err
				_ = pw.CloseWithError(err)
				resultCh <- sr
				return
			}
			var entry ledger.Entry
			if err := json.Unmarshal(raw, &entry); err == nil {
				if entry.Seq != uint64(dbSeq) || entry.PrevHash != dbPrev || entry.Hash != dbHash {
					sr.errors = append(sr.errors, fmt.Sprintf("database ledger metadata mismatch at seq %d", dbSeq))
				}
				sr.seq, sr.hash = entry.Seq, entry.Hash
			}
			if _, err := pw.Write(raw); err != nil {
				sr.err = err
				_ = pw.CloseWithError(err)
				resultCh <- sr
				return
			}
			if _, err := pw.Write([]byte{'\n'}); err != nil {
				sr.err = err
				_ = pw.CloseWithError(err)
				resultCh <- sr
				return
			}
		}
		if err := rows.Err(); err != nil {
			sr.err = err
			_ = pw.CloseWithError(err)
		} else {
			_ = pw.Close()
		}
		resultCh <- sr
	}()

	res := ledger.Verify(pr, k.keys)
	sr := <-resultCh
	if sr.err != nil {
		return ledger.VerifyResult{}, 0, "", fmt.Errorf("stream ledger verification: %w", sr.err)
	}
	if len(sr.errors) > 0 {
		res.Valid = false
		res.Errors = append(res.Errors, sr.errors...)
	}
	if !res.Valid {
		return res, 0, "", nil
	}
	return res, sr.seq, sr.hash, nil
}

func lockLedger(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, ledgerLockKey); err != nil {
		return fmt.Errorf("lock ledger tail: %w", err)
	}
	return nil
}

func readTail(ctx context.Context, tx pgx.Tx) (uint64, string, error) {
	var dbSeq int64
	var dbPrev, dbHash string
	var raw []byte
	err := tx.QueryRow(ctx, `
		SELECT seq, prev_hash, hash, entry_bytes
		FROM crab.ledger_entries
		ORDER BY seq DESC
		LIMIT 1`).Scan(&dbSeq, &dbPrev, &dbHash, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", nil
	}
	if err != nil {
		return 0, "", fmt.Errorf("read ledger tail: %w", err)
	}
	var entry ledger.Entry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return 0, "", fmt.Errorf("decode ledger tail: %w", err)
	}
	if entry.Seq != uint64(dbSeq) || entry.PrevHash != dbPrev || entry.Hash != dbHash {
		return 0, "", ErrUnexpectedTail
	}
	return entry.Seq, entry.Hash, nil
}
