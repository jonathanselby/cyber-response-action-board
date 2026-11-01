package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authn"
)

// ErrNoCredentialWrapKey is returned when TOTP material is requested but the
// deployment has no credential wrap key configured.
var ErrNoCredentialWrapKey = errors.New("credential wrap key is not configured")

// CredentialStore persists local credential lifecycle state and sessions. It
// stores and reads; it never decides whether a credential change is allowed.
type CredentialStore struct {
	Security *SecurityStore
	WrapKey  []byte
}

func (c CredentialStore) GetCredential(ctx context.Context, humanID string) (authn.Credential, bool, error) {
	var cred authn.Credential
	var expires sql.NullTime
	err := c.Security.pool.QueryRow(ctx, `SELECT human_id, salt, iterations, password_hash, must_set_password, expires_at FROM crab.local_credentials WHERE human_id=$1`, humanID).
		Scan(&cred.Record.HumanID, &cred.Record.Salt, &cred.Record.Iterations, &cred.Record.Hash, &cred.MustSetPassword, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return authn.Credential{}, false, nil
	}
	if err != nil {
		return authn.Credential{}, false, fmt.Errorf("read local credential: %w", err)
	}
	if err := authn.ValidatePasswordRecord(cred.Record); err != nil {
		return authn.Credential{}, false, fmt.Errorf("invalid stored local credential: %w", err)
	}
	if expires.Valid {
		v := expires.Time.UTC()
		cred.ExpiresAt = &v
	}
	return cred, true, nil
}

// PutPassword stores a human-chosen password and clears any enrolment state.
func (c CredentialStore) PutPassword(ctx context.Context, record authn.PasswordRecord) error {
	return c.put(ctx, record, false, nil)
}

// PutEnrolmentCode stores a one-time enrolment code that must be replaced by a
// password before any session is created.
func (c CredentialStore) PutEnrolmentCode(ctx context.Context, record authn.PasswordRecord, expiresAt time.Time) error {
	expiry := expiresAt.UTC()
	return c.put(ctx, record, true, &expiry)
}

func (c CredentialStore) put(ctx context.Context, record authn.PasswordRecord, mustSet bool, expiresAt *time.Time) error {
	if err := authn.ValidatePasswordRecord(record); err != nil {
		return err
	}
	var expires any
	if expiresAt != nil {
		expires = *expiresAt
	}
	_, err := c.Security.pool.Exec(ctx, `
		INSERT INTO crab.local_credentials(human_id, salt, iterations, password_hash, must_set_password, expires_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,now())
		ON CONFLICT (human_id) DO UPDATE SET salt=EXCLUDED.salt, iterations=EXCLUDED.iterations, password_hash=EXCLUDED.password_hash,
			must_set_password=EXCLUDED.must_set_password, expires_at=EXCLUDED.expires_at, updated_at=now()`,
		record.HumanID, record.Salt, record.Iterations, record.Hash, mustSet, expires)
	if err != nil {
		return fmt.Errorf("store local credential: %w", err)
	}
	return nil
}

// GetTOTP returns the stored TOTP secret and whether it has been confirmed.
func (c CredentialStore) GetTOTP(ctx context.Context, humanID string) (authn.TOTPRecord, bool, bool, error) {
	if len(c.WrapKey) == 0 {
		return authn.TOTPRecord{}, false, false, ErrNoCredentialWrapKey
	}
	var confirmed bool
	if err := c.Security.pool.QueryRow(ctx, `SELECT confirmed FROM crab.totp_credentials WHERE human_id=$1`, humanID).Scan(&confirmed); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return authn.TOTPRecord{}, false, false, nil
		}
		return authn.TOTPRecord{}, false, false, fmt.Errorf("read TOTP state: %w", err)
	}
	record, found, err := c.Security.GetTOTPRecord(ctx, c.WrapKey, humanID)
	if err != nil || !found {
		return authn.TOTPRecord{}, false, false, err
	}
	return record, confirmed, true, nil
}

// PutPendingTOTP stores a new, unconfirmed TOTP secret, replacing any other.
func (c CredentialStore) PutPendingTOTP(ctx context.Context, record authn.TOTPRecord) error {
	if len(c.WrapKey) == 0 {
		return ErrNoCredentialWrapKey
	}
	if err := c.Security.PutTOTPRecord(ctx, c.WrapKey, record); err != nil {
		return err
	}
	_, err := c.Security.pool.Exec(ctx, `UPDATE crab.totp_credentials SET confirmed=false WHERE human_id=$1`, record.HumanID)
	return err
}

func (c CredentialStore) ConfirmTOTP(ctx context.Context, humanID string) error {
	_, err := c.Security.pool.Exec(ctx, `UPDATE crab.totp_credentials SET confirmed=true, updated_at=now() WHERE human_id=$1`, humanID)
	return err
}

func (c CredentialStore) DeleteTOTP(ctx context.Context, humanID string) error {
	_, err := c.Security.pool.Exec(ctx, `DELETE FROM crab.totp_credentials WHERE human_id=$1`, humanID)
	return err
}

// DeleteSessionsFor removes every session a person holds, on every surface.
func (c CredentialStore) DeleteSessionsFor(ctx context.Context, humanID string) error {
	_, err := c.Security.pool.Exec(ctx, `DELETE FROM crab.sessions WHERE human_id=$1`, humanID)
	return err
}

// SweepExpiredSessions deletes at most limit expired sessions. Called on each
// sign-in, it keeps the session table bounded without a scheduler.
func (c CredentialStore) SweepExpiredSessions(ctx context.Context, now time.Time, limit int) error {
	_, err := c.Security.pool.Exec(ctx, `DELETE FROM crab.sessions WHERE token_digest IN (SELECT token_digest FROM crab.sessions WHERE expires_at <= $1 LIMIT $2)`, now.UTC(), limit)
	return err
}
