package postgres

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

type fakeCommandRunner struct {
	calls [][]string
	fail  error
	dump  []byte
}

func (f *fakeCommandRunner) Run(_ context.Context, name string, args ...string) error {
	call := append([]string{name}, args...)
	f.calls = append(f.calls, call)
	if f.fail != nil {
		return f.fail
	}
	if name == "pg_dump" {
		for i := 0; i+1 < len(args); i++ {
			if args[i] == "--file" {
				return os.WriteFile(args[i+1], f.dump, 0o600)
			}
		}
		return errors.New("pg_dump --file argument missing")
	}
	return nil
}

func TestBackupCreatesHashMetadataAndDoesNotUseShell(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "crab.dump")
	runner := &fakeCommandRunner{dump: []byte("verified-postgres-backup")}
	when := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	meta, err := backupWithRunner(context.Background(), "postgres://runtime@db/crab", output, runner, func() time.Time { return when })
	if err != nil {
		t.Fatal(err)
	}
	if meta.Format != "postgresql-custom" || meta.SHA256 == "" || meta.Bytes != int64(len(runner.dump)) || !meta.CreatedAt.Equal(when) {
		t.Fatalf("metadata = %+v", meta)
	}
	wantCall := []string{"pg_dump", "--format=custom", "--no-owner", "--file"}
	if len(runner.calls) != 1 || len(runner.calls[0]) < 6 || !reflect.DeepEqual(runner.calls[0][:4], wantCall) {
		t.Fatalf("pg_dump call = %#v", runner.calls)
	}
	if err := VerifyBackup(output); err != nil {
		t.Fatalf("verify backup: %v", err)
	}
	info, err := os.Stat(output)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("backup mode = %o", info.Mode().Perm())
	}
}

func TestFailedBackupPreservesLastGoodBackup(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "crab.dump")
	if err := os.WriteFile(output, []byte("last-good"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &fakeCommandRunner{fail: errors.New("dump failed")}
	if _, err := backupWithRunner(context.Background(), "postgres://runtime@db/crab", output, runner, time.Now); err == nil {
		t.Fatal("expected backup failure")
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "last-good" {
		t.Fatalf("last-good backup changed: %q", got)
	}
}

func TestRestoreRefusesTamperedBackupBeforePostgres(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "crab.dump")
	backupRunner := &fakeCommandRunner{dump: []byte("original")}
	if _, err := backupWithRunner(context.Background(), "postgres://runtime@db/crab", output, backupRunner, time.Now); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	restoreRunner := &fakeCommandRunner{}
	if err := restoreWithRunner(context.Background(), "postgres://owner@db/crab", output, restoreRunner); err == nil {
		t.Fatal("expected integrity failure")
	}
	if len(restoreRunner.calls) != 0 {
		t.Fatalf("pg_restore should not run for tampered backup: %#v", restoreRunner.calls)
	}
}

func TestRestoreUsesPostgresNativeTooling(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "crab.dump")
	backupRunner := &fakeCommandRunner{dump: []byte("original")}
	if _, err := backupWithRunner(context.Background(), "postgres://runtime@db/crab", output, backupRunner, time.Now); err != nil {
		t.Fatal(err)
	}
	restoreRunner := &fakeCommandRunner{}
	if err := restoreWithRunner(context.Background(), "postgres://owner@db/crab", output, restoreRunner); err != nil {
		t.Fatal(err)
	}
	want := []string{"pg_restore", "--clean", "--if-exists", "--exit-on-error", "--no-owner", "--dbname", "postgres://owner@db/crab", output}
	if len(restoreRunner.calls) != 1 || !reflect.DeepEqual(restoreRunner.calls[0], want) {
		t.Fatalf("pg_restore call = %#v want %#v", restoreRunner.calls, want)
	}
}
