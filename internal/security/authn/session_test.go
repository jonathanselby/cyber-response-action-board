package authn

import (
	"context"
	"testing"
	"time"
)

type memorySessions struct {
	rows map[[32]byte]Session
}

func (m *memorySessions) PutSession(_ context.Context, digest [32]byte, s Session) error {
	if m.rows == nil {
		m.rows = map[[32]byte]Session{}
	}
	m.rows[digest] = s
	return nil
}

func (m *memorySessions) GetSession(_ context.Context, digest [32]byte, now time.Time) (Session, bool, error) {
	s, ok := m.rows[digest]
	if !ok || !s.ExpiresAt.After(now) {
		return Session{}, false, nil
	}
	return s, true, nil
}

func (m *memorySessions) DeleteSession(_ context.Context, digest [32]byte) error {
	delete(m.rows, digest)
	return nil
}

func TestSessionStoresOnlyTokenDigestAndExpires(t *testing.T) {
	now := time.Date(2026, 10, 20, 1, 0, 0, 0, time.UTC)
	store := &memorySessions{}
	svc := Sessions{Store: store, TTL: time.Hour, Now: func() time.Time { return now }}

	token, created, err := svc.Create(context.Background(), "alice", "local", false)
	if err != nil {
		t.Fatal(err)
	}
	if token == "" || created.HumanID != "alice" || len(store.rows) != 1 {
		t.Fatalf("token=%q session=%+v rows=%d", token, created, len(store.rows))
	}
	resolved, ok, err := svc.Resolve(context.Background(), token)
	if err != nil || !ok || resolved.HumanID != "alice" {
		t.Fatalf("resolve session=%+v ok=%v err=%v", resolved, ok, err)
	}

	now = now.Add(2 * time.Hour)
	if _, ok, err := svc.Resolve(context.Background(), token); err != nil || ok {
		t.Fatalf("expired session ok=%v err=%v", ok, err)
	}
}

func TestSessionRevocation(t *testing.T) {
	now := time.Date(2026, 10, 20, 1, 0, 0, 0, time.UTC)
	store := &memorySessions{}
	svc := Sessions{Store: store, TTL: time.Hour, Now: func() time.Time { return now }}
	token, _, err := svc.Create(context.Background(), "alice", "local", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Revoke(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := svc.Resolve(context.Background(), token); err != nil || ok {
		t.Fatalf("revoked session ok=%v err=%v", ok, err)
	}
}
