package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authn"
)

// SecurityStore persists authentication supporting state only. It contains no
// role, incident-membership, compartment or output-authorisation decisions.
type SecurityStore struct {
	pool *pgxpool.Pool
}

func NewSecurityStore(pool *pgxpool.Pool) (*SecurityStore, error) {
	if pool == nil {
		return nil, errors.New("PostgreSQL pool is required")
	}
	return &SecurityStore{pool: pool}, nil
}

// PutPasswordRecord stores verification material. The data adapter deliberately
// does not decide whether a credential change is authorised; callers must pass
// through the application governance boundary before invoking a live change.
func (s *SecurityStore) PutPasswordRecord(ctx context.Context, record authn.PasswordRecord) error {
	if err := authn.ValidatePasswordRecord(record); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO crab.local_credentials(human_id, salt, iterations, password_hash, updated_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (human_id) DO UPDATE SET
			salt = EXCLUDED.salt,
			iterations = EXCLUDED.iterations,
			password_hash = EXCLUDED.password_hash,
			updated_at = now()`,
		record.HumanID, record.Salt, record.Iterations, record.Hash)
	if err != nil {
		return fmt.Errorf("store local credential: %w", err)
	}
	return nil
}

func (s *SecurityStore) GetPasswordRecord(ctx context.Context, humanID string) (authn.PasswordRecord, bool, error) {
	var record authn.PasswordRecord
	err := s.pool.QueryRow(ctx, `
		SELECT human_id, salt, iterations, password_hash
		FROM crab.local_credentials
		WHERE human_id = $1`, humanID).Scan(&record.HumanID, &record.Salt, &record.Iterations, &record.Hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return authn.PasswordRecord{}, false, nil
	}
	if err != nil {
		return authn.PasswordRecord{}, false, fmt.Errorf("read local credential: %w", err)
	}
	if err := authn.ValidatePasswordRecord(record); err != nil {
		return authn.PasswordRecord{}, false, fmt.Errorf("invalid stored local credential: %w", err)
	}
	return record, true, nil
}

func (s *SecurityStore) PutSession(ctx context.Context, digest [32]byte, session authn.Session) error {
	if session.HumanID == "" || session.AuthMethod == "" || session.CreatedAt.IsZero() || !session.ExpiresAt.After(session.CreatedAt) {
		return errors.New("invalid session")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO crab.sessions(token_digest, human_id, auth_method, break_glass, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		digest[:], session.HumanID, session.AuthMethod, session.BreakGlass, session.CreatedAt.UTC(), session.ExpiresAt.UTC())
	if err != nil {
		return fmt.Errorf("store session: %w", err)
	}
	return nil
}

func (s *SecurityStore) GetSession(ctx context.Context, digest [32]byte, now time.Time) (authn.Session, bool, error) {
	var session authn.Session
	err := s.pool.QueryRow(ctx, `
		SELECT human_id, auth_method, break_glass, created_at, expires_at
		FROM crab.sessions
		WHERE token_digest = $1 AND expires_at > $2`, digest[:], now.UTC()).Scan(
		&session.HumanID, &session.AuthMethod, &session.BreakGlass, &session.CreatedAt, &session.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return authn.Session{}, false, nil
	}
	if err != nil {
		return authn.Session{}, false, fmt.Errorf("read session: %w", err)
	}
	return session, true, nil
}

func (s *SecurityStore) DeleteSession(ctx context.Context, digest [32]byte) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM crab.sessions WHERE token_digest = $1`, digest[:]); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}
