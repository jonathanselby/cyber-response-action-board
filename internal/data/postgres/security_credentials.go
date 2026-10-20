package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authn"
)

func (s *SecurityStore) PutTOTPRecord(ctx context.Context, wrapKey []byte, record authn.TOTPRecord) error {
	if record.HumanID == "" || record.Window < 0 || record.Window > authn.MaxTOTPWindow {
		return errors.New("invalid TOTP record")
	}
	if err := authn.ValidateTOTPSecret(record.Secret); err != nil {
		return err
	}
	nonce, ciphertext, err := authn.WrapSecret(wrapKey, []byte(record.Secret))
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO crab.totp_credentials(human_id, secret_nonce, secret_ciphertext, window, updated_at)
		VALUES ($1,$2,$3,$4,now()) ON CONFLICT (human_id) DO UPDATE SET secret_nonce=EXCLUDED.secret_nonce, secret_ciphertext=EXCLUDED.secret_ciphertext, window=EXCLUDED.window, updated_at=now()`, record.HumanID, nonce, ciphertext, record.Window)
	if err != nil {
		return fmt.Errorf("store TOTP credential: %w", err)
	}
	return nil
}

func (s *SecurityStore) GetTOTPRecord(ctx context.Context, wrapKey []byte, humanID string) (authn.TOTPRecord, bool, error) {
	var nonce, ciphertext []byte
	var window int
	err := s.pool.QueryRow(ctx, `SELECT secret_nonce, secret_ciphertext, window FROM crab.totp_credentials WHERE human_id=$1`, humanID).Scan(&nonce, &ciphertext, &window)
	if errors.Is(err, pgx.ErrNoRows) {
		return authn.TOTPRecord{}, false, nil
	}
	if err != nil {
		return authn.TOTPRecord{}, false, fmt.Errorf("read TOTP credential: %w", err)
	}
	secret, err := authn.UnwrapSecret(wrapKey, nonce, ciphertext)
	if err != nil {
		return authn.TOTPRecord{}, false, fmt.Errorf("unwrap TOTP credential: %w", err)
	}
	record := authn.TOTPRecord{HumanID: humanID, Secret: string(secret), Window: window}
	if err := authn.ValidateTOTPSecret(record.Secret); err != nil {
		return authn.TOTPRecord{}, false, err
	}
	return record, true, nil
}

func (s *SecurityStore) PutPasskeyRecord(ctx context.Context, record authn.PasskeyRecord) error {
	if err := authn.ValidatePasskeyRecord(record); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO crab.passkey_credentials(human_id, credential_id, rp_id, origin, public_key_pem, require_user_verification, sign_count)
		VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (human_id, credential_id) DO UPDATE SET rp_id=EXCLUDED.rp_id, origin=EXCLUDED.origin, public_key_pem=EXCLUDED.public_key_pem, require_user_verification=EXCLUDED.require_user_verification, sign_count=EXCLUDED.sign_count`, record.HumanID, record.CredentialID, record.RPID, record.Origin, record.PublicKeyPEM, record.RequireUserVerification, int64(record.SignCount))
	if err != nil {
		return fmt.Errorf("store passkey credential: %w", err)
	}
	return nil
}

func (s *SecurityStore) GetPasskeyRecord(ctx context.Context, humanID, credentialID string) (authn.PasskeyRecord, bool, error) {
	var record authn.PasskeyRecord
	var count int64
	err := s.pool.QueryRow(ctx, `SELECT human_id, credential_id, rp_id, origin, public_key_pem, require_user_verification, sign_count FROM crab.passkey_credentials WHERE human_id=$1 AND credential_id=$2`, humanID, credentialID).Scan(&record.HumanID, &record.CredentialID, &record.RPID, &record.Origin, &record.PublicKeyPEM, &record.RequireUserVerification, &count)
	if errors.Is(err, pgx.ErrNoRows) {
		return authn.PasskeyRecord{}, false, nil
	}
	if err != nil {
		return authn.PasskeyRecord{}, false, fmt.Errorf("read passkey credential: %w", err)
	}
	record.SignCount = uint32(count)
	return record, true, authn.ValidatePasskeyRecord(record)
}

func (s *SecurityStore) UpdatePasskeyCounter(ctx context.Context, humanID, credentialID string, signCount uint32) error {
	_, err := s.pool.Exec(ctx, `UPDATE crab.passkey_credentials SET sign_count=$3 WHERE human_id=$1 AND credential_id=$2`, humanID, credentialID, int64(signCount))
	if err != nil {
		return fmt.Errorf("update passkey counter: %w", err)
	}
	return nil
}

func (s *SecurityStore) PutBreakGlassRecord(ctx context.Context, record authn.BreakGlassRecord, active bool) error {
	password := authn.PasswordRecord{HumanID: record.KeyholderID, Salt: record.Salt, Iterations: record.Iterations, Hash: record.Hash}
	if err := authn.ValidatePasswordRecord(password); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO crab.break_glass_credentials(keyholder_id, salt, iterations, key_hash, active, updated_at)
		VALUES ($1,$2,$3,$4,$5,now()) ON CONFLICT (keyholder_id) DO UPDATE SET salt=EXCLUDED.salt, iterations=EXCLUDED.iterations, key_hash=EXCLUDED.key_hash, active=EXCLUDED.active, updated_at=now()`, record.KeyholderID, record.Salt, record.Iterations, record.Hash, active)
	if err != nil {
		return fmt.Errorf("store break-glass credential: %w", err)
	}
	return nil
}

func (s *SecurityStore) ActiveBreakGlassRecords(ctx context.Context) ([]authn.BreakGlassRecord, error) {
	rows, err := s.pool.Query(ctx, `SELECT keyholder_id, salt, iterations, key_hash FROM crab.break_glass_credentials WHERE active ORDER BY keyholder_id`)
	if err != nil {
		return nil, fmt.Errorf("read break-glass credentials: %w", err)
	}
	defer rows.Close()
	var records []authn.BreakGlassRecord
	for rows.Next() {
		var record authn.BreakGlassRecord
		if err := rows.Scan(&record.KeyholderID, &record.Salt, &record.Iterations, &record.Hash); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}
