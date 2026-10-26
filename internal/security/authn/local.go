package authn

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"unicode"
)

const (
	MinPBKDF2Iterations = 600000
	MinSaltBytes        = 16
	PasswordHashBytes   = 32
)

type PasswordRecord struct {
	HumanID    string
	Salt       string
	Iterations int
	Hash       string
}

type PasswordStore interface {
	GetPasswordRecord(context.Context, string) (PasswordRecord, bool, error)
}

type Local struct {
	Store PasswordStore
}

func (l Local) Authenticate(ctx context.Context, humanID, password string) (bool, error) {
	if l.Store == nil || humanID == "" || password == "" {
		return false, nil
	}
	record, ok, err := l.Store.GetPasswordRecord(ctx, humanID)
	if err != nil || !ok {
		return false, err
	}
	return VerifyPassword(record, password), nil
}

func NewPasswordRecord(humanID, password string, iterations int) (PasswordRecord, error) {
	if humanID == "" || password == "" || iterations < MinPBKDF2Iterations {
		return PasswordRecord{}, errors.New("password record parameters below minimum")
	}
	salt := make([]byte, MinSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return PasswordRecord{}, err
	}
	h, err := pbkdf2.Key(sha256.New, password, salt, iterations, PasswordHashBytes)
	if err != nil {
		return PasswordRecord{}, err
	}
	return PasswordRecord{
		HumanID:    humanID,
		Salt:       base64.StdEncoding.EncodeToString(salt),
		Iterations: iterations,
		Hash:       base64.StdEncoding.EncodeToString(h),
	}, nil
}

// MinPasswordLength is the minimum length of a human-chosen password.
const MinPasswordLength = 12

// ValidateNewPassword is the single rule for human-chosen passwords (T-193):
// at least 12 characters with an upper-case letter, a lower-case letter and a
// character that is neither a letter nor a digit. Generated break-glass keys
// are not human-chosen and keep their own rule.
func ValidateNewPassword(password string) error {
	var upper, lower, symbol bool
	count := 0
	for _, r := range password {
		count++
		switch {
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsLower(r):
			lower = true
		case !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsSpace(r) && !unicode.IsControl(r):
			symbol = true
		}
	}
	if count < MinPasswordLength || !upper || !lower || !symbol {
		return errors.New("password must be at least 12 characters and include an upper-case letter, a lower-case letter and a symbol")
	}
	return nil
}

func ValidatePasswordRecord(r PasswordRecord) error {
	if r.HumanID == "" || r.Iterations < MinPBKDF2Iterations {
		return errors.New("password record parameters below minimum")
	}
	salt, err := base64.StdEncoding.DecodeString(r.Salt)
	if err != nil || len(salt) < MinSaltBytes {
		return errors.New("password record has invalid salt")
	}
	hash, err := base64.StdEncoding.DecodeString(r.Hash)
	if err != nil || len(hash) != PasswordHashBytes {
		return errors.New("password record has invalid hash")
	}
	return nil
}

func VerifyPassword(r PasswordRecord, password string) bool {
	if password == "" || ValidatePasswordRecord(r) != nil {
		return false
	}
	salt, _ := base64.StdEncoding.DecodeString(r.Salt)
	want, _ := base64.StdEncoding.DecodeString(r.Hash)
	got, err := pbkdf2.Key(sha256.New, password, salt, r.Iterations, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(want, got) == 1
}
