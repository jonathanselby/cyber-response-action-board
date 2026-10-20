package oob

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash"
	"strings"
	"time"
)

const (
	MinSessionKeyBytes = 32
	MaxCallbackTTL     = 30 * time.Minute
)

type Person struct {
	ID       string
	Callback string
}

type Session struct {
	Token                 string
	PersonID              string
	Callback              string
	ExpiresAt             time.Time
	InboundContactIgnored bool
	Seal                  string
}

type Result struct {
	Verified bool
	PersonID string
	Reason   string
}

// StartCallback deliberately chooses only the route registered before the
// instruction arrived. inboundContact is recorded only as ignored metadata.
func StartCallback(person Person, inboundContact string, now time.Time, ttl time.Duration, sessionKey []byte) (Session, error) {
	if strings.TrimSpace(person.ID) == "" || strings.TrimSpace(person.Callback) == "" {
		return Session{}, errors.New("pre-registered callback required")
	}
	if now.IsZero() || ttl <= 0 || ttl > MaxCallbackTTL {
		return Session{}, errors.New("invalid callback time or ttl")
	}
	if len(sessionKey) < MinSessionKeyBytes {
		return Session{}, errors.New("callback session key below minimum")
	}
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return Session{}, err
	}
	session := Session{
		Token:                 base64.RawURLEncoding.EncodeToString(raw),
		PersonID:              person.ID,
		Callback:              person.Callback,
		ExpiresAt:             now.Add(ttl).UTC(),
		InboundContactIgnored: strings.TrimSpace(inboundContact) != "",
	}
	session.Seal = seal(session, sessionKey)
	return session, nil
}

func CompleteCallback(session Session, token, personID string, connected bool, now time.Time, sessionKey []byte) (Result, error) {
	result := Result{PersonID: personID}
	if len(sessionKey) < MinSessionKeyBytes || now.IsZero() {
		return Result{}, errors.New("invalid callback completion parameters")
	}
	if session.Token == "" || session.PersonID == "" || session.Callback == "" || session.ExpiresAt.IsZero() || session.Seal == "" {
		result.Reason = "invalid callback session"
		return result, nil
	}
	wantSeal := seal(session, sessionKey)
	if subtle.ConstantTimeCompare([]byte(wantSeal), []byte(session.Seal)) != 1 {
		result.Reason = "callback session integrity rejected"
		return result, nil
	}
	if !now.Before(session.ExpiresAt) {
		result.Reason = "callback session expired"
		return result, nil
	}
	if personID != session.PersonID || subtle.ConstantTimeCompare([]byte(token), []byte(session.Token)) != 1 {
		result.Reason = "callback session mismatch"
		return result, nil
	}
	if !connected {
		result.Reason = "registered callback did not connect"
		return result, nil
	}
	result.Verified = true
	return result, nil
}

func seal(session Session, key []byte) string {
	mac := hmac.New(sha256.New, key)
	writeField(mac, session.Token)
	writeField(mac, session.PersonID)
	writeField(mac, session.Callback)
	var timestamp [8]byte
	binary.BigEndian.PutUint64(timestamp[:], uint64(session.ExpiresAt.UTC().UnixNano()))
	_, _ = mac.Write(timestamp[:])
	if session.InboundContactIgnored {
		_, _ = mac.Write([]byte{1})
	} else {
		_, _ = mac.Write([]byte{0})
	}
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func writeField(mac hash.Hash, value string) {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	_, _ = mac.Write(length[:])
	_, _ = mac.Write([]byte(value))
}
