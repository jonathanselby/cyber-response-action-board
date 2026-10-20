package architecture

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSlice4AddsNoWorkflowEngine(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate source tree")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "internal", "workflow")); err == nil {
		t.Fatal("Slice 4 introduced an internal workflow engine")
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, tree := range []string{"domain", "app", "data"} {
		checkTree(t, filepath.Join(root, "internal", tree), func(path, imp string) {
			lower := strings.ToLower(imp)
			if strings.Contains(lower, "/workflow") || strings.Contains(lower, "temporal.io") {
				t.Errorf("Slice 4 must use Action semantics rather than a workflow engine: %s -> %s", path, imp)
			}
		})
	}
}
