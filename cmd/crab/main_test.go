package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"version"}, &out, &errOut)
	if code != exitOK {
		t.Fatalf("exit = %d stderr=%s", code, errOut.String())
	}
	if !strings.HasPrefix(out.String(), "crab 0.0.15-dev") {
		t.Fatalf("version output = %q", out.String())
	}
}

func TestReadinessFailsClosedBeforeConfiguredDurableStore(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"readiness"}, &out, &errOut)
	if code != exitRefused {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, out.String(), errOut.String())
	}
	for _, want := range []string{
		`"ready": false`,
		`"postgresql"`,
		`"key_custody"`,
		`"authentication_authorisation"`,
		`"incident_semantic_kernel"`,
		`"actions_checklists_timeline"`,
		`"obligations_timed_waits"`,
		`"access_governance_compartments"`,
		`"evidence_immutable_blobs"`,
		`"corpus_deterministic_retrieval"`,
		`"model_assistance"`,
		`"communications_notifier"`,
		`"print_paper_resilience"`,
		`"recovery_pir_retention_closure"`,
		`"backup_restore"`,
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("readiness output missing %s: %s", want, out.String())
		}
	}
}

func TestUnbuiltControlPathRefusesRatherThanFakingCapability(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"mcp"}, &out, &errOut)
	if code != exitRefused || !strings.Contains(errOut.String(), "no alternate control path") {
		t.Fatalf("exit=%d stderr=%q", code, errOut.String())
	}
}

func TestPrintCommandRemainsFailClosedUntilAuthenticatedOperatorWiringExists(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"print"}, &out, &errOut)
	if code != exitRefused || !strings.Contains(errOut.String(), "no alternate control path") {
		t.Fatalf("exit=%d stderr=%q", code, errOut.String())
	}
}

func TestBackupAndRestoreRequireExplicitPaths(t *testing.T) {
	for _, args := range [][]string{{"backup"}, {"restore"}} {
		var out, errOut bytes.Buffer
		code := run(args, &out, &errOut)
		if code != exitUsage {
			t.Fatalf("%v exit=%d stdout=%s stderr=%s", args, code, out.String(), errOut.String())
		}
	}
}
