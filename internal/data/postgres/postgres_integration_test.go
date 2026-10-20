package postgres

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

const (
	testOwnerRole   = "crab_owner_test"
	testRuntimeRole = "crab_runtime_test"
)

type testDB struct {
	admin   *pgxpool.Pool
	owner   *pgxpool.Pool
	runtime *pgxpool.Pool
}

func setupTestDB(t *testing.T) *testDB {
	t.Helper()
	adminURL := os.Getenv("CRAB_TEST_POSTGRES_ADMIN_URL")
	if adminURL == "" {
		t.Skip("CRAB_TEST_POSTGRES_ADMIN_URL not set")
	}
	ctx := context.Background()
	admin, err := OpenPool(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	cleanupRoles := func() {
		_, _ = admin.Exec(ctx, `DROP SCHEMA IF EXISTS crab CASCADE`)
		_, _ = admin.Exec(ctx, `REVOKE ALL PRIVILEGES ON DATABASE crab_test FROM `+testRuntimeRole)
		_, _ = admin.Exec(ctx, `REVOKE ALL PRIVILEGES ON DATABASE crab_test FROM `+testOwnerRole)
		_, _ = admin.Exec(ctx, `DROP ROLE IF EXISTS `+testRuntimeRole)
		_, _ = admin.Exec(ctx, `DROP ROLE IF EXISTS `+testOwnerRole)
	}
	cleanupRoles()
	if _, err := admin.Exec(ctx, `CREATE ROLE `+testOwnerRole+` LOGIN PASSWORD 'owner-test-password'`); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `CREATE ROLE `+testRuntimeRole+` LOGIN PASSWORD 'runtime-test-password'`); err != nil {
		cleanupRoles()
		admin.Close()
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `GRANT CREATE ON DATABASE crab_test TO `+testOwnerRole); err != nil {
		cleanupRoles()
		admin.Close()
		t.Fatal(err)
	}
	owner, err := OpenPool(ctx, strings.Replace(adminURL, "postgres:postgres", testOwnerRole+":owner-test-password", 1))
	if err != nil {
		cleanupRoles()
		admin.Close()
		t.Fatal(err)
	}
	if err := Migrate(ctx, owner, testRuntimeRole); err != nil {
		owner.Close()
		cleanupRoles()
		admin.Close()
		t.Fatal(err)
	}
	runtime, err := OpenPool(ctx, strings.Replace(adminURL, "postgres:postgres", testRuntimeRole+":runtime-test-password", 1))
	if err != nil {
		owner.Close()
		cleanupRoles()
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		runtime.Close()
		owner.Close()
		cleanupRoles()
		admin.Close()
	})
	return &testDB{admin: admin, owner: owner, runtime: runtime}
}

func keys(t *testing.T) (ledger.Keyring, ed25519.PrivateKey, ed25519.PrivateKey) {
	t.Helper()
	makerPub, makerPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serverPub, serverPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return ledger.Keyring{
		"maker":  base64.StdEncoding.EncodeToString(makerPub),
		"server": base64.StdEncoding.EncodeToString(serverPub),
	}, makerPriv, serverPriv
}

func signedRecord(t *testing.T, makerPriv ed25519.PrivateKey, id string) ledger.Record {
	t.Helper()
	r := ledger.Record{
		SchemaVersion: 1,
		Timestamp:     time.Date(2026, 10, 20, 1, 2, 3, 0, time.UTC),
		TimeStatus:    "trusted",
		IncidentID:    "INC-001",
		IncidentMode:  "exercise",
		WriteSemantic: "record",
		RecordType:    "test-record",
		RecordID:      id,
		Status:        "recorded",
		Maker:         "maker",
		Compartment:   "GENERAL",
		Data:          json.RawMessage(`{"z":2,"a":1}`),
	}
	sig, err := ledger.SignRecord(r, makerPriv)
	if err != nil {
		t.Fatal(err)
	}
	r.MakerSignature = ledger.Signature{KeyID: "maker", Value: sig}
	return r
}

type tableProjection struct {
	fail bool
}

func (p tableProjection) Reset(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `TRUNCATE crab.test_projection`)
	return err
}

func (p tableProjection) Apply(ctx context.Context, tx pgx.Tx, entry ledger.Entry) error {
	if _, err := tx.Exec(ctx, `INSERT INTO crab.test_projection(seq, record_id) VALUES ($1, $2)`, int64(entry.Seq), entry.Record.RecordID); err != nil {
		return err
	}
	if p.fail {
		return errors.New("injected projection failure")
	}
	return nil
}

func prepareProjection(t *testing.T, db *testDB) {
	t.Helper()
	ctx := context.Background()
	if _, err := db.owner.Exec(ctx, `
		CREATE TABLE crab.test_projection(seq bigint PRIMARY KEY, record_id text NOT NULL);
		GRANT SELECT, INSERT, DELETE, TRUNCATE ON crab.test_projection TO `+testRuntimeRole); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationSeparatesOwnerAndRuntime(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	if err := ValidateSchema(ctx, db.runtime); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRuntimeRole(ctx, db.runtime); err != nil {
		t.Fatal(err)
	}
	var owner string
	if err := db.runtime.QueryRow(ctx, `SELECT pg_get_userbyid(c.relowner) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='crab' AND c.relname='ledger_entries'`).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	if owner != testOwnerRole {
		t.Fatalf("ledger owner=%q", owner)
	}
	if _, err := db.runtime.Exec(ctx, `UPDATE crab.ledger_entries SET hash = hash`); err == nil {
		t.Fatal("runtime unexpectedly has UPDATE on authoritative ledger")
	}
	if _, err := db.owner.Exec(ctx, `INSERT INTO crab.schema_migrations(version) VALUES (4)`); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSchema(ctx, db.runtime); err == nil {
		t.Fatal("future unsupported schema version unexpectedly accepted")
	}
}

func TestAppendReplayRollbackTamperAndExport(t *testing.T) {
	db := setupTestDB(t)
	prepareProjection(t, db)
	ctx := context.Background()
	keyring, makerPriv, serverPriv := keys(t)
	k, err := OpenKernel(ctx, db.runtime, keyring)
	if err != nil {
		t.Fatal(err)
	}
	if k.IntegrityFailed() || !k.InitialVerification().Valid {
		t.Fatalf("initial verification=%+v", k.InitialVerification())
	}

	signer := Signer{KeyID: "server", PrivateKey: serverPriv}
	entry1, err := k.Append(ctx, signedRecord(t, makerPriv, "R-1"), signer, tableProjection{})
	if err != nil {
		t.Fatal(err)
	}
	if entry1.Seq != 1 || entry1.PrevHash != "" {
		t.Fatalf("first entry=%+v", entry1)
	}
	var projected string
	if err := db.runtime.QueryRow(ctx, `SELECT record_id FROM crab.test_projection WHERE seq=1`).Scan(&projected); err != nil || projected != "R-1" {
		t.Fatalf("projection=%q err=%v", projected, err)
	}

	if _, err := k.Append(ctx, signedRecord(t, makerPriv, "R-2-fail"), signer, tableProjection{fail: true}); err == nil {
		t.Fatal("projection failure unexpectedly committed")
	}
	var ledgerCount, projectionCount int
	if err := db.runtime.QueryRow(ctx, `SELECT count(*) FROM crab.ledger_entries`).Scan(&ledgerCount); err != nil {
		t.Fatal(err)
	}
	if err := db.runtime.QueryRow(ctx, `SELECT count(*) FROM crab.test_projection`).Scan(&projectionCount); err != nil {
		t.Fatal(err)
	}
	if ledgerCount != 1 || projectionCount != 1 {
		t.Fatalf("rollback failed ledger=%d projection=%d", ledgerCount, projectionCount)
	}

	entry2, err := k.Append(ctx, signedRecord(t, makerPriv, "R-2"), signer, tableProjection{})
	if err != nil {
		t.Fatal(err)
	}
	if entry2.Seq != 2 || entry2.PrevHash != entry1.Hash {
		t.Fatalf("second entry=%+v", entry2)
	}

	if _, err := db.owner.Exec(ctx, `DELETE FROM crab.test_projection WHERE seq=1`); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Rebuild(ctx, tableProjection{}); err != nil {
		t.Fatal(err)
	}
	if err := db.runtime.QueryRow(ctx, `SELECT count(*) FROM crab.test_projection`).Scan(&projectionCount); err != nil || projectionCount != 2 {
		t.Fatalf("rebuild projection=%d err=%v", projectionCount, err)
	}

	var exported bytes.Buffer
	res, err := k.Export(ctx, &exported)
	if err != nil || !res.Valid || res.Entries != 2 {
		t.Fatalf("export res=%+v err=%v", res, err)
	}
	if verified := ledger.Verify(bytes.NewReader(exported.Bytes()), keyring); !verified.Valid || verified.Entries != 2 {
		t.Fatalf("portable verify=%+v", verified)
	}

	lines := bytes.Split(bytes.TrimSpace(exported.Bytes()), []byte{'\n'})
	if _, err := db.owner.Exec(ctx, `TRUNCATE crab.test_projection, crab.ledger_entries`); err != nil {
		t.Fatal(err)
	}
	for _, raw := range lines {
		var e ledger.Entry
		if err := json.Unmarshal(raw, &e); err != nil {
			t.Fatal(err)
		}
		if _, err := db.owner.Exec(ctx, `INSERT INTO crab.ledger_entries(seq, prev_hash, hash, entry_bytes) VALUES ($1,$2,$3,$4)`, int64(e.Seq), e.PrevHash, e.Hash, raw); err != nil {
			t.Fatal(err)
		}
	}
	k2, err := OpenKernel(ctx, db.runtime, keyring)
	if err != nil || k2.IntegrityFailed() {
		t.Fatalf("restored open err=%v integrity=%v", err, k2 != nil && k2.IntegrityFailed())
	}
	if _, err := k2.Rebuild(ctx, tableProjection{}); err != nil {
		t.Fatal(err)
	}
	var got []string
	rows, err := db.runtime.Query(ctx, `SELECT record_id FROM crab.test_projection ORDER BY seq`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		got = append(got, id)
	}
	rows.Close()
	if fmt.Sprint(got) != "[R-1 R-2]" {
		t.Fatalf("restored projection=%v", got)
	}

	var original []byte
	if err := db.owner.QueryRow(ctx, `SELECT entry_bytes FROM crab.ledger_entries WHERE seq=2`).Scan(&original); err != nil {
		t.Fatal(err)
	}
	var tampered map[string]any
	if err := json.Unmarshal(original, &tampered); err != nil {
		t.Fatal(err)
	}
	record := tampered["record"].(map[string]any)
	record["record_id"] = "TAMPERED"
	bad, _ := json.Marshal(tampered)
	if _, err := db.owner.Exec(ctx, `UPDATE crab.ledger_entries SET entry_bytes=$1 WHERE seq=2`, bad); err != nil {
		t.Fatal(err)
	}
	check, err := k2.Verify(ctx)
	if err != nil || check.Valid || !k2.IntegrityFailed() {
		t.Fatalf("tamper check=%+v err=%v failed=%v", check, err, k2.IntegrityFailed())
	}
	if _, err := k2.Append(ctx, signedRecord(t, makerPriv, "R-3"), signer, tableProjection{}); !errors.Is(err, ErrIntegrityFailed) {
		t.Fatalf("append after tamper err=%v", err)
	}
	var degradedExport bytes.Buffer
	degraded, err := k2.Export(ctx, &degradedExport)
	if err != nil || degraded.Valid || degraded.Entries != 2 || degradedExport.Len() == 0 {
		t.Fatalf("integrity-failed export res=%+v bytes=%d err=%v", degraded, degradedExport.Len(), err)
	}
	if _, err := db.owner.Exec(ctx, `UPDATE crab.ledger_entries SET entry_bytes=$1 WHERE seq=2`, original); err != nil {
		t.Fatal(err)
	}
	if _, err := k2.Rebuild(ctx, tableProjection{}); err != nil {
		t.Fatal(err)
	}
	if k2.IntegrityFailed() {
		t.Fatal("verified explicit rebuild did not clear integrity gate")
	}
}
