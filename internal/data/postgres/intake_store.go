package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/access"
)

// IntakeKeyState reads one intake key's projected state.
func (s *AccessStore) IntakeKeyState(ctx context.Context, keyName string) (access.IntakeKeyState, error) {
	state := access.IntakeKeyState{KeyName: keyName}
	var rotated sql.NullTime
	err := s.pool.QueryRow(ctx, `SELECT accountable_human,status,expires_at,issued_at,rotated_at FROM crab.intake_keys WHERE key_name=$1`, keyName).
		Scan(&state.AccountableHuman, &state.Status, &state.ExpiresAt, &state.IssuedAt, &rotated)
	if errors.Is(err, pgx.ErrNoRows) {
		return access.IntakeKeyState{KeyName: keyName}, nil
	}
	if err != nil {
		return access.IntakeKeyState{}, fmt.Errorf("read intake key: %w", err)
	}
	state.Found = true
	state.ExpiresAt, state.IssuedAt = state.ExpiresAt.UTC(), state.IssuedAt.UTC()
	if rotated.Valid {
		v := rotated.Time.UTC()
		state.RotatedAt = &v
	}
	return state, nil
}

// IntakeKeys lists every intake key, with when the previous secret stops
// working if a rotation overlap is still running at now.
func (s *AccessStore) IntakeKeys(ctx context.Context, now time.Time) ([]access.IntakeKeyState, map[string]time.Time, error) {
	rows, err := s.pool.Query(ctx, `SELECT key_name,accountable_human,status,expires_at,issued_at,rotated_at FROM crab.intake_keys ORDER BY key_name`)
	if err != nil {
		return nil, nil, fmt.Errorf("list intake keys: %w", err)
	}
	var out []access.IntakeKeyState
	for rows.Next() {
		state := access.IntakeKeyState{Found: true}
		var rotated sql.NullTime
		if err := rows.Scan(&state.KeyName, &state.AccountableHuman, &state.Status, &state.ExpiresAt, &state.IssuedAt, &rotated); err != nil {
			rows.Close()
			return nil, nil, err
		}
		state.ExpiresAt, state.IssuedAt = state.ExpiresAt.UTC(), state.IssuedAt.UTC()
		if rotated.Valid {
			v := rotated.Time.UTC()
			state.RotatedAt = &v
		}
		out = append(out, state)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	overlaps := map[string]time.Time{}
	orows, err := s.pool.Query(ctx, `SELECT key_name, max(valid_until) FROM crab.intake_key_secrets WHERE valid_until IS NOT NULL AND valid_until > $1 GROUP BY key_name`, now)
	if err != nil {
		return nil, nil, fmt.Errorf("list intake key overlaps: %w", err)
	}
	defer orows.Close()
	for orows.Next() {
		var name string
		var until time.Time
		if err := orows.Scan(&name, &until); err != nil {
			return nil, nil, err
		}
		overlaps[name] = until.UTC()
	}
	return out, overlaps, orows.Err()
}

// PutIntakeSecret stores a new secret hash for a key. Every earlier secret of
// the key stops working at oldUntil at the latest, in the same transaction.
func (s *AccessStore) PutIntakeSecret(ctx context.Context, keyName string, hash []byte, oldUntil, now time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE crab.intake_key_secrets SET valid_until=LEAST(COALESCE(valid_until, 'infinity'::timestamptz), $2) WHERE key_name=$1`, keyName, oldUntil.UTC()); err != nil {
		return fmt.Errorf("bound previous intake secrets: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO crab.intake_key_secrets(secret_hash,key_name,created_at) VALUES($1,$2,$3)`, hash, keyName, now.UTC()); err != nil {
		return fmt.Errorf("store intake secret: %w", err)
	}
	return tx.Commit(ctx)
}

// IntakeSecretValid reports whether hash is a current secret of the key at now.
func (s *AccessStore) IntakeSecretValid(ctx context.Context, keyName string, hash []byte, now time.Time) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM crab.intake_key_secrets WHERE secret_hash=$1 AND key_name=$2 AND (valid_until IS NULL OR valid_until > $3))`, hash, keyName, now).Scan(&ok)
	return ok, err
}

// FirstReportByRef returns the report a key already submitted with ref.
func (s *ContinuityStore) FirstReportByRef(ctx context.Context, keyName, ref string) (string, bool, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT report_id FROM crab.first_reports WHERE payload ? 'external_ref' AND payload->>'intake_key'=$1 AND payload->>'external_ref'=$2`, keyName, ref).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read first report by reference: %w", err)
	}
	return id, true, nil
}

func (r *AccessRuntime) IntakeKeyState(ctx context.Context, keyName string) (access.IntakeKeyState, error) {
	return r.Store.IntakeKeyState(ctx, keyName)
}

func (r *AccessRuntime) IntakeKeys(ctx context.Context, now time.Time) ([]access.IntakeKeyState, map[string]time.Time, error) {
	return r.Store.IntakeKeys(ctx, now)
}

func (r *AccessRuntime) PutIntakeSecret(ctx context.Context, keyName string, hash []byte, oldUntil, now time.Time) error {
	return r.Store.PutIntakeSecret(ctx, keyName, hash, oldUntil, now)
}

func (r *AccessRuntime) IntakeSecretValid(ctx context.Context, keyName string, hash []byte, now time.Time) (bool, error) {
	return r.Store.IntakeSecretValid(ctx, keyName, hash, now)
}

func (r *ContinuityRuntime) FirstReportByRef(ctx context.Context, keyName, ref string) (string, bool, error) {
	return r.Store.FirstReportByRef(ctx, keyName, ref)
}
