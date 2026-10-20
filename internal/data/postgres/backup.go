package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type BackupMetadata struct {
	Format    string    `json:"format"`
	SHA256    string    `json:"sha256"`
	Bytes     int64     `json:"bytes"`
	CreatedAt time.Time `json:"created_at"`
}

type commandRunner interface {
	Run(context.Context, string, ...string) error
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Run()
}

func Backup(ctx context.Context, databaseURL, output string) (BackupMetadata, error) {
	return backupWithRunner(ctx, databaseURL, output, execRunner{}, time.Now)
}

func Restore(ctx context.Context, databaseURL, input string) error {
	return restoreWithRunner(ctx, databaseURL, input, execRunner{})
}

func backupWithRunner(ctx context.Context, databaseURL, output string, runner commandRunner, now func() time.Time) (BackupMetadata, error) {
	if databaseURL == "" {
		return BackupMetadata{}, errors.New("database URL is required")
	}
	if output == "" {
		return BackupMetadata{}, errors.New("backup output path is required")
	}
	if runner == nil || now == nil {
		return BackupMetadata{}, errors.New("backup runner and clock are required")
	}
	dir := filepath.Dir(output)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return BackupMetadata{}, fmt.Errorf("create backup directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".crab-backup-*.dump")
	if err != nil {
		return BackupMetadata{}, fmt.Errorf("create backup temp file: %w", err)
	}
	tmpName := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return BackupMetadata{}, fmt.Errorf("close backup temp file: %w", err)
	}
	defer os.Remove(tmpName)
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return BackupMetadata{}, fmt.Errorf("secure backup temp file: %w", err)
	}
	if err := runner.Run(ctx, "pg_dump", "--format=custom", "--no-owner", "--file", tmpName, databaseURL); err != nil {
		return BackupMetadata{}, fmt.Errorf("pg_dump: %w", err)
	}
	meta, err := backupMetadata(tmpName, now().UTC())
	if err != nil {
		return BackupMetadata{}, err
	}
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return BackupMetadata{}, fmt.Errorf("encode backup metadata: %w", err)
	}
	metaBytes = append(metaBytes, '\n')
	metaTmp, err := os.CreateTemp(dir, ".crab-backup-metadata-*.json")
	if err != nil {
		return BackupMetadata{}, fmt.Errorf("create backup metadata temp file: %w", err)
	}
	metaTmpName := metaTmp.Name()
	defer os.Remove(metaTmpName)
	if err := metaTmp.Chmod(0o600); err != nil {
		_ = metaTmp.Close()
		return BackupMetadata{}, fmt.Errorf("secure backup metadata temp file: %w", err)
	}
	if _, err := metaTmp.Write(metaBytes); err != nil {
		_ = metaTmp.Close()
		return BackupMetadata{}, fmt.Errorf("write backup metadata: %w", err)
	}
	if err := metaTmp.Sync(); err != nil {
		_ = metaTmp.Close()
		return BackupMetadata{}, fmt.Errorf("sync backup metadata: %w", err)
	}
	if err := metaTmp.Close(); err != nil {
		return BackupMetadata{}, fmt.Errorf("close backup metadata: %w", err)
	}
	if err := os.Rename(tmpName, output); err != nil {
		return BackupMetadata{}, fmt.Errorf("commit backup: %w", err)
	}
	if err := os.Rename(metaTmpName, output+".json"); err != nil {
		return BackupMetadata{}, fmt.Errorf("commit backup metadata: %w", err)
	}
	return meta, nil
}

func restoreWithRunner(ctx context.Context, databaseURL, input string, runner commandRunner) error {
	if databaseURL == "" {
		return errors.New("database URL is required")
	}
	if input == "" {
		return errors.New("backup input path is required")
	}
	if runner == nil {
		return errors.New("restore runner is required")
	}
	if err := VerifyBackup(input); err != nil {
		return err
	}
	if err := runner.Run(ctx, "pg_restore", "--clean", "--if-exists", "--exit-on-error", "--no-owner", "--dbname", databaseURL, input); err != nil {
		return fmt.Errorf("pg_restore: %w", err)
	}
	return nil
}

func VerifyBackup(input string) error {
	metaBytes, err := os.ReadFile(input + ".json")
	if err != nil {
		return fmt.Errorf("read backup metadata: %w", err)
	}
	var expected BackupMetadata
	if err := json.Unmarshal(metaBytes, &expected); err != nil {
		return fmt.Errorf("parse backup metadata: %w", err)
	}
	actual, err := backupMetadata(input, expected.CreatedAt)
	if err != nil {
		return err
	}
	if expected.Format != "postgresql-custom" || expected.SHA256 == "" || expected.SHA256 != actual.SHA256 || expected.Bytes != actual.Bytes {
		return errors.New("backup integrity verification failed")
	}
	return nil
}

func backupMetadata(path string, createdAt time.Time) (BackupMetadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return BackupMetadata{}, fmt.Errorf("open backup: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return BackupMetadata{}, fmt.Errorf("hash backup: %w", err)
	}
	return BackupMetadata{Format: "postgresql-custom", SHA256: hex.EncodeToString(h.Sum(nil)), Bytes: n, CreatedAt: createdAt}, nil
}
