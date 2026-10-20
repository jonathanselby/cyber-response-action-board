package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOperatorSessionFileRequiresPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session")
	if err := os.WriteFile(path, []byte("secret-token\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readOperatorSessionFile(path); err == nil {
		t.Fatal("expected group/other-readable session file to be refused")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	token, err := readOperatorSessionFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if token != "secret-token" {
		t.Fatalf("token=%q", token)
	}
}

func TestMCPRequiresAuthenticatedSessionFile(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runMCP(nil, strings.NewReader(""), &out, &errOut)
	if code != exitUsage || !strings.Contains(errOut.String(), "session-file") {
		t.Fatalf("exit=%d stderr=%q", code, errOut.String())
	}
}
