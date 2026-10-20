package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/security/authn"
)

func TestSecurityStoreLocalAuthAndSessions(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	store, err := NewSecurityStore(db.runtime)
	if err != nil {
		t.Fatal(err)
	}

	record, err := authn.NewPasswordRecord("human-1", "correct horse battery staple", authn.MinPBKDF2Iterations)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutPasswordRecord(ctx, record); err != nil {
		t.Fatal(err)
	}
	local := authn.Local{Store: store}
	if ok, err := local.Authenticate(ctx, "human-1", "correct horse battery staple"); err != nil || !ok {
		t.Fatalf("valid local authentication ok=%v err=%v", ok, err)
	}
	if ok, err := local.Authenticate(ctx, "human-1", "wrong password"); err != nil || ok {
		t.Fatalf("bad password ok=%v err=%v", ok, err)
	}
	if ok, err := local.Authenticate(ctx, "unknown-human", "correct horse battery staple"); err != nil || ok {
		t.Fatalf("unknown human ok=%v err=%v", ok, err)
	}

	now := time.Date(2026, 10, 20, 1, 30, 0, 0, time.UTC)
	sessions := authn.Sessions{
		Store: store,
		TTL:   15 * time.Minute,
		Now:   func() time.Time { return now },
	}
	token, created, err := sessions.Create(ctx, "human-1", "local", false)
	if err != nil {
		t.Fatal(err)
	}
	if token == "" || created.HumanID != "human-1" || !created.ExpiresAt.Equal(now.Add(15*time.Minute)) {
		t.Fatalf("created session=%+v token_present=%v", created, token != "")
	}

	var digestBytes int
	if err := db.runtime.QueryRow(ctx, `SELECT octet_length(token_digest) FROM crab.sessions`).Scan(&digestBytes); err != nil {
		t.Fatal(err)
	}
	if digestBytes != 32 {
		t.Fatalf("stored session digest bytes=%d", digestBytes)
	}
	var plaintextTokenColumn bool
	if err := db.runtime.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema='crab' AND table_name='sessions' AND column_name='token'
		)`).Scan(&plaintextTokenColumn); err != nil {
		t.Fatal(err)
	}
	if plaintextTokenColumn {
		t.Fatal("sessions table unexpectedly stores a plaintext bearer token column")
	}

	resolved, ok, err := sessions.Resolve(ctx, token)
	if err != nil || !ok || resolved.HumanID != "human-1" || resolved.BreakGlass {
		t.Fatalf("resolved session=%+v ok=%v err=%v", resolved, ok, err)
	}

	now = now.Add(16 * time.Minute)
	if _, ok, err := sessions.Resolve(ctx, token); err != nil || ok {
		t.Fatalf("expired session ok=%v err=%v", ok, err)
	}

	now = time.Date(2026, 10, 20, 2, 0, 0, 0, time.UTC)
	token2, _, err := sessions.Create(ctx, "human-1", "local", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := sessions.Revoke(ctx, token2); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := sessions.Resolve(ctx, token2); err != nil || ok {
		t.Fatalf("revoked session ok=%v err=%v", ok, err)
	}
}
