package postgres

import (
	"encoding/base32"
	"testing"

	"github.com/jonathanselby/cyber-response-action-board/internal/security/authn"
)

func TestWrappedTOTPAndBreakGlassPersistence(t *testing.T) {
	db := setupTestDB(t)
	store, err := NewSecurityStore(db.runtime)
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	wrapKey := []byte("0123456789abcdef0123456789abcdef")
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))
	if err := store.PutTOTPRecord(ctx, wrapKey, authn.TOTPRecord{HumanID: "human-1", Secret: secret, Window: 1}); err != nil {
		t.Fatal(err)
	}
	var ciphertext []byte
	if err := db.runtime.QueryRow(ctx, `SELECT secret_ciphertext FROM crab.totp_credentials WHERE human_id='human-1'`).Scan(&ciphertext); err != nil {
		t.Fatal(err)
	}
	if string(ciphertext) == secret {
		t.Fatal("TOTP secret stored as plaintext")
	}
	record, ok, err := store.GetTOTPRecord(ctx, wrapKey, "human-1")
	if err != nil || !ok || record.Secret != secret {
		t.Fatalf("TOTP record=%+v ok=%v err=%v", record, ok, err)
	}
	if _, _, err := store.GetTOTPRecord(ctx, []byte("wrong-key-wrong-key-wrong-key-12"), "human-1"); err == nil {
		t.Fatal("wrong wrap key unexpectedly decrypted TOTP secret")
	}

	a, err := authn.NewBreakGlassRecord("keyholder-a", "offline-a", authn.MinPBKDF2Iterations)
	if err != nil {
		t.Fatal(err)
	}
	b, err := authn.NewBreakGlassRecord("keyholder-b", "offline-b", authn.MinPBKDF2Iterations)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutBreakGlassRecord(ctx, a, true); err != nil {
		t.Fatal(err)
	}
	if err := store.PutBreakGlassRecord(ctx, b, true); err != nil {
		t.Fatal(err)
	}
	records, err := store.ActiveBreakGlassRecords(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authn.VerifyBreakGlass(records, map[string]string{"keyholder-a": "offline-a", "keyholder-b": "offline-b"}); err != nil {
		t.Fatal(err)
	}
	if err := store.PutBreakGlassRecord(ctx, b, false); err != nil {
		t.Fatal(err)
	}
	records, err = store.ActiveBreakGlassRecords(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authn.VerifyBreakGlass(records, map[string]string{"keyholder-a": "offline-a", "keyholder-b": "offline-b"}); err == nil {
		t.Fatal("revoked break-glass keyholder remained usable")
	}
}
