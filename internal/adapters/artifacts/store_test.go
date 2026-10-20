package artifacts

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicPrimaryAndSecondaryCopies(t *testing.T) {
	primary := t.TempDir()
	secondary := t.TempDir()
	store, err := New(primary, secondary)
	if err != nil {
		t.Fatal(err)
	}
	result, err := store.WriteArtifact("brief.pdf", []byte("new-pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if result.PrimaryPath == "" || result.SecondaryPath == "" || result.SecondaryWarning != "" {
		t.Fatalf("result=%+v", result)
	}
	for _, path := range []string{result.PrimaryPath, result.SecondaryPath} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "new-pdf" {
			t.Fatalf("%s=%q", path, got)
		}
	}
}

func TestSecondaryFailureLeavesPrimaryCurrentAndSecondaryLastGoodUntouched(t *testing.T) {
	primary := t.TempDir()
	secondary := t.TempDir()
	oldSecondary := filepath.Join(secondary, "brief.pdf")
	if err := os.WriteFile(oldSecondary, []byte("last-good-secondary"), 0600); err != nil {
		t.Fatal(err)
	}
	store, _ := New(primary, secondary)
	originalWrite := store.atomicWrite
	store.atomicWrite = func(path string, data []byte) error {
		if filepath.Dir(path) == secondary {
			return errors.New("secondary unavailable")
		}
		return originalWrite(path, data)
	}
	result, err := store.WriteArtifact("brief.pdf", []byte("new-primary"))
	if err != nil {
		t.Fatal(err)
	}
	if result.SecondaryWarning == "" {
		t.Fatal("secondary failure not surfaced")
	}
	gotPrimary, _ := os.ReadFile(filepath.Join(primary, "brief.pdf"))
	gotSecondary, _ := os.ReadFile(oldSecondary)
	if string(gotPrimary) != "new-primary" || string(gotSecondary) != "last-good-secondary" {
		t.Fatalf("primary=%q secondary=%q", gotPrimary, gotSecondary)
	}
}

func TestPrimaryFailurePreservesLastGood(t *testing.T) {
	primary := t.TempDir()
	path := filepath.Join(primary, "brief.pdf")
	if err := os.WriteFile(path, []byte("last-good"), 0600); err != nil {
		t.Fatal(err)
	}
	store, _ := New(primary, "")
	store.atomicWrite = func(string, []byte) error { return errors.New("renderer/export target failed") }
	if _, err := store.WriteArtifact("brief.pdf", []byte("bad-new")); err == nil {
		t.Fatal("primary failure accepted")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "last-good" {
		t.Fatalf("last good changed: %q", got)
	}
}

func TestGeneratedDerivedFileRemainsReadableWithoutDatabase(t *testing.T) {
	store, _ := New(t.TempDir(), "")
	result, err := store.WriteArtifact("pack.pdf", []byte("%PDF-1.7 offline copy"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(result.PrimaryPath)
	if err != nil || string(got) != "%PDF-1.7 offline copy" {
		t.Fatalf("read offline derived copy: %v %q", err, got)
	}
}
