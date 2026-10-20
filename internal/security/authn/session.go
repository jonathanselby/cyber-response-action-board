package authn

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"
)

const SessionTokenBytes = 32

type Session struct {
	HumanID    string
	AuthMethod string
	BreakGlass bool
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

type SessionStore interface {
	PutSession(context.Context, [32]byte, Session) error
	GetSession(context.Context, [32]byte, time.Time) (Session, bool, error)
	DeleteSession(context.Context, [32]byte) error
}

type Sessions struct {
	Store SessionStore
	TTL   time.Duration
	Now   func() time.Time
}

func (s Sessions) Create(ctx context.Context, humanID, authMethod string, breakGlass bool) (string, Session, error) {
	if s.Store == nil || s.TTL <= 0 || humanID == "" || authMethod == "" {
		return "", Session{}, errors.New("invalid session configuration")
	}
	raw := make([]byte, SessionTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", Session{}, err
	}
	now := s.now().UTC()
	sess := Session{HumanID: humanID, AuthMethod: authMethod, BreakGlass: breakGlass, CreatedAt: now, ExpiresAt: now.Add(s.TTL)}
	digest := sha256.Sum256(raw)
	if err := s.Store.PutSession(ctx, digest, sess); err != nil {
		return "", Session{}, err
	}
	return base64.RawURLEncoding.EncodeToString(raw), sess, nil
}

func (s Sessions) Resolve(ctx context.Context, token string) (Session, bool, error) {
	if s.Store == nil || token == "" {
		return Session{}, false, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != SessionTokenBytes {
		return Session{}, false, nil
	}
	digest := sha256.Sum256(raw)
	sess, ok, err := s.Store.GetSession(ctx, digest, s.now().UTC())
	if err != nil || !ok {
		return Session{}, ok, err
	}
	if !sess.ExpiresAt.After(s.now().UTC()) {
		_ = s.Store.DeleteSession(ctx, digest)
		return Session{}, false, nil
	}
	return sess, true, nil
}

func (s Sessions) Revoke(ctx context.Context, token string) error {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != SessionTokenBytes {
		return nil
	}
	digest := sha256.Sum256(raw)
	return s.Store.DeleteSession(ctx, digest)
}

func (s Sessions) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}
