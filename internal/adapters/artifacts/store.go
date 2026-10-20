package artifacts

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
)

type Store struct {
	PrimaryDir   string
	SecondaryDir string
	atomicWrite  func(string, []byte) error
}

func New(primaryDir, secondaryDir string) (*Store, error) {
	primaryDir = strings.TrimSpace(primaryDir)
	if primaryDir == "" {
		return nil, errors.New("primary derived-file directory is required")
	}
	return &Store{PrimaryDir: primaryDir, SecondaryDir: strings.TrimSpace(secondaryDir), atomicWrite: atomicWriteFile}, nil
}

func (s *Store) WriteArtifact(name string, data []byte) (app.ArtifactWriteResult, error) {
	if s == nil || s.atomicWrite == nil {
		return app.ArtifactWriteResult{}, errors.New("artifact store is not configured")
	}
	if filepath.Base(name) != name || name == "." || name == "" || strings.ContainsAny(name, `/\\`) {
		return app.ArtifactWriteResult{}, errors.New("artifact name must be a safe base filename")
	}
	if len(data) == 0 {
		return app.ArtifactWriteResult{}, errors.New("artifact bytes are required")
	}
	primary := filepath.Join(s.PrimaryDir, name)
	if err := s.atomicWrite(primary, data); err != nil {
		return app.ArtifactWriteResult{}, fmt.Errorf("write primary derived copy: %w", err)
	}
	result := app.ArtifactWriteResult{PrimaryPath: primary}
	if s.SecondaryDir == "" {
		return result, nil
	}
	secondary := filepath.Join(s.SecondaryDir, name)
	if err := s.atomicWrite(secondary, data); err != nil {
		result.SecondaryWarning = "secondary derived copy not updated: " + err.Error()
		return result, nil
	}
	result.SecondaryPath = secondary
	return result, nil
}

func atomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".crab-derived-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(0600); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}
