package authn

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
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

func VerifyPassword(r PasswordRecord, password string) bool {
	if r.HumanID == "" || r.Iterations < MinPBKDF2Iterations || password == "" {
		return false
	}
	salt, err := base64.StdEncoding.DecodeString(r.Salt)
	if err != nil || len(salt) < MinSaltBytes {
		return false
	}
	want, err := base64.StdEncoding.DecodeString(r.Hash)
	if err != nil || len(want) != PasswordHashBytes {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, r.Iterations, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(want, got) == 1
}
